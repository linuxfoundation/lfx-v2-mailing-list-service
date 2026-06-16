// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// reindex_mailing_lists re-triggers the full event processing pipeline for any
// combination of GroupsIO object types by re-putting their NATS KV entries so the
// existing consumer event handler picks them up and re-indexes them.
//
// Re-putting a service or mailing list KV entry will republish both the main document
// and its settings document (groupsio_service_settings / groupsio_mailing_list_settings).
//
// Usage:
//
//	NATS_URL=nats://localhost:4222 OPENSEARCH_URL=http://localhost:9200 \
//	  go run ./scripts/reindex_mailing_lists/ -types groupsio_mailing_list,groupsio_member
//
// Optional flags:
//
//	-types    Comma-separated list of object types to reindex (required).
//	          Supported values:
//	            groupsio_service
//	            groupsio_mailing_list
//	            groupsio_member
//	            groupsio_artifact
//	-reindex  Actually re-put KV entries and trigger reindexing (default: false,
//	          logs what would be re-put without making any changes)
//
// Environment variables:
//
//	NATS_URL       NATS server URL (default: nats://127.0.0.1:4222)
//	OPENSEARCH_URL OpenSearch base URL (default: http://localhost:9200)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	kvBucketName   = "v1-objects"
	scrollPageSize = 200
)

// objectTypeConfig maps an OpenSearch object_type to the KV key prefix in the v1-objects bucket.
// Reindexing groupsio_service also republishes groupsio_service_settings from the same KV entry.
// Reindexing groupsio_mailing_list also republishes groupsio_mailing_list_settings from the same KV entry.
var objectTypeConfig = map[string]string{
	"groupsio_service":      "itx-groupsio-v2-service.",
	"groupsio_mailing_list": "itx-groupsio-v2-subgroup.",
	"groupsio_member":       "itx-groupsio-v2-member.",
	"groupsio_artifact":     "itx-groupsio-v2-artifact.",
}

// osHit holds just enough of each OpenSearch hit to get the object_id.
type osHit struct {
	Source struct {
		ObjectID string `json:"object_id"`
	} `json:"_source"`
}

// osScrollResponse is the minimal shape of a scroll API response.
type osScrollResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []osHit `json:"hits"`
	} `json:"hits"`
}

func main() {
	typesFlag := flag.String("types", "", "comma-separated list of object types to reindex (required)")
	reindex := flag.Bool("reindex", false, "actually re-put KV entries and trigger reindexing (default: logs only)")
	flag.Parse()

	osURL := os.Getenv("OPENSEARCH_URL")
	if osURL == "" {
		osURL = "http://localhost:9200"
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	if *typesFlag == "" {
		fmt.Fprintf(os.Stderr, "error: -types is required\nsupported types: %s\n", strings.Join(supportedTypes(), ", "))
		os.Exit(1)
	}

	requestedTypes := strings.Split(*typesFlag, ",")
	for _, t := range requestedTypes {
		t = strings.TrimSpace(t)
		if _, ok := objectTypeConfig[t]; !ok {
			slog.Error("unsupported object type", "type", t)
			fmt.Fprintf(os.Stderr, "supported types: %s\n", strings.Join(supportedTypes(), ", "))
			os.Exit(1)
		}
	}

	ctx := context.Background()

	slog.InfoContext(ctx, "reindex_mailing_lists starting",
		"opensearch_url", osURL,
		"nats_url", natsURL,
		"reindex", *reindex,
		"types", *typesFlag,
	)

	nc, err := nats.Connect(natsURL,
		nats.Timeout(10*time.Second),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to NATS", "error", err)
		os.Exit(1)
	}

	exitCode := run(ctx, nc, osURL, requestedTypes, *reindex)
	nc.Close()
	os.Exit(exitCode)
}

func run(ctx context.Context, nc *nats.Conn, osURL string, requestedTypes []string, reindex bool) int {
	js, err := jetstream.New(nc)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create JetStream context", "error", err)
		return 1
	}

	kv, err := js.KeyValue(ctx, kvBucketName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to bind to KV bucket", "bucket", kvBucketName, "error", err)
		return 1
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	var totalProcessed, totalFailed, totalSkipped, totalNotFound int

	for _, objectType := range requestedTypes {
		objectType = strings.TrimSpace(objectType)
		kvPrefix := objectTypeConfig[objectType]

		slog.InfoContext(ctx, "processing object type", "object_type", objectType)

		processed, failed, skipped, notFound, err := reindexType(ctx, httpClient, kv, osURL, objectType, kvPrefix, reindex)
		if err != nil {
			slog.ErrorContext(ctx, "fatal error processing type", "object_type", objectType, "error", err)
			return 1
		}

		slog.InfoContext(ctx, "finished object type",
			"object_type", objectType,
			"processed", processed,
			"failed", failed,
			"skipped", skipped,
			"not_found", notFound,
		)

		totalProcessed += processed
		totalFailed += failed
		totalSkipped += skipped
		totalNotFound += notFound
	}

	slog.InfoContext(ctx, "reindex_mailing_lists complete",
		"processed", totalProcessed,
		"failed", totalFailed,
		"skipped", totalSkipped,
		"not_found", totalNotFound,
	)

	if totalFailed > 0 {
		return 1
	}
	return 0
}

// reindexType scrolls OpenSearch for a given object_type, then re-puts each
// matching KV entry to re-trigger the event processing pipeline.
func reindexType(
	ctx context.Context,
	httpClient *http.Client,
	kv jetstream.KeyValue,
	osURL, objectType, kvPrefix string,
	reindex bool,
) (processed, failed, skipped, notFound int, err error) {
	scrollID, firstPage, err := openScroll(ctx, httpClient, osURL, objectType, scrollPageSize)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("open scroll: %w", err)
	}
	defer func() {
		if err := deleteScroll(ctx, httpClient, osURL, scrollID); err != nil {
			slog.WarnContext(ctx, "failed to delete scroll context", "error", err)
		}
	}()

	page := firstPage
	for len(page) > 0 {
		for _, hit := range page {
			id := hit.Source.ObjectID
			if id == "" {
				skipped++
				continue
			}

			kvKey := kvPrefix + id

			if !reindex {
				slog.InfoContext(ctx, "[dry-run] would re-put", "key", kvKey)
				processed++
				continue
			}

			entry, getErr := kv.Get(ctx, kvKey)
			if getErr != nil {
				if errors.Is(getErr, jetstream.ErrKeyNotFound) {
					slog.WarnContext(ctx, "key not found in KV bucket", "key", kvKey)
					notFound++
					continue
				}
				slog.ErrorContext(ctx, "failed to get KV entry", "key", kvKey, "error", getErr)
				failed++
				continue
			}

			if _, putErr := kv.Put(ctx, kvKey, entry.Value()); putErr != nil {
				slog.ErrorContext(ctx, "failed to re-put KV entry", "key", kvKey, "error", putErr)
				failed++
				continue
			}

			processed++
		}

		slog.InfoContext(ctx, "progress",
			"object_type", objectType,
			"processed", processed,
			"failed", failed,
			"skipped", skipped,
			"not_found", notFound,
		)

		page, scrollID, err = nextScrollPage(ctx, httpClient, osURL, scrollID)
		if err != nil {
			return processed, failed, skipped, notFound, fmt.Errorf("scroll page: %w", err)
		}
	}

	return processed, failed, skipped, notFound, nil
}

// openScroll opens an OpenSearch scroll for all documents of the given object_type.
func openScroll(ctx context.Context, client *http.Client, osURL, objectType string, pageSize int) (string, []osHit, error) {
	query := map[string]any{
		"query": map[string]any{
			"term": map[string]any{"object_type": objectType},
		},
		"_source": []string{"object_id"},
		"size":    pageSize,
	}
	body, _ := json.Marshal(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osURL+"/resources/_search?scroll=2m", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read scroll response body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", nil, fmt.Errorf("OpenSearch returned status %d: %s", resp.StatusCode, string(raw))
	}
	var result osScrollResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", nil, fmt.Errorf("unmarshal scroll response: %w", err)
	}
	return result.ScrollID, result.Hits.Hits, nil
}

// nextScrollPage fetches the next page using the scroll ID and returns the updated scroll ID.
func nextScrollPage(ctx context.Context, client *http.Client, osURL, scrollID string) ([]osHit, string, error) {
	payload := map[string]string{"scroll": "2m", "scroll_id": scrollID}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osURL+"/_search/scroll", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read scroll page body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("OpenSearch returned status %d: %s", resp.StatusCode, string(raw))
	}
	var result osScrollResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, "", fmt.Errorf("unmarshal scroll page: %w", err)
	}
	return result.Hits.Hits, result.ScrollID, nil
}

// deleteScroll cleans up the scroll context in OpenSearch.
func deleteScroll(ctx context.Context, client *http.Client, osURL, scrollID string) error {
	payload := map[string]string{"scroll_id": scrollID}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, osURL+"/_search/scroll", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close() //nolint:errcheck
	return nil
}

// supportedTypes returns the sorted list of valid object type names.
func supportedTypes() []string {
	types := make([]string, 0, len(objectTypeConfig))
	for t := range objectTypeConfig {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
