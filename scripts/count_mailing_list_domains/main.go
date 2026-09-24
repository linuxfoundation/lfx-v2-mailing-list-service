// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// count_mailing_list_domains pages through query-service mailing-list resources
// and reports how many have a non-empty domain.
//
// Usage:
//
//	go run ./scripts/count_mailing_list_domains/
//	QUERY_SERVICE_TOKEN="$QUERY_SERVICE_TOKEN" go run ./scripts/count_mailing_list_domains/
//
// QUERY_SERVICE_TOKEN is optional and adds a bearer token when set.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultEndpoint = "https://lfx-api.v2.cluster.lfx.dev/query/resources"

type resource struct {
	Data struct {
		Domain string `json:"domain"`
	} `json:"data"`
}

type page struct {
	Resources json.RawMessage `json:"resources"`
	PageToken string          `json:"page_token"`
}

func main() {
	endpoint := flag.String("endpoint", defaultEndpoint, "query-service resources endpoint")
	flag.Parse()

	client := &http.Client{Timeout: 30 * time.Second}
	if err := run(context.Background(), client, *endpoint, os.Getenv("QUERY_SERVICE_TOKEN"), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, client *http.Client, endpoint, token string, out io.Writer) error {
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}
	if token != "" && !strings.EqualFold(parsedEndpoint.Scheme, "https") {
		return errors.New("query-service endpoint must use HTTPS when QUERY_SERVICE_TOKEN is set")
	}
	requestClient := client
	if token != "" {
		clientCopy := *client
		previousCheckRedirect := client.CheckRedirect
		clientCopy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
			if !strings.EqualFold(request.URL.Scheme, "https") || !strings.EqualFold(request.URL.Host, parsedEndpoint.Host) {
				return fmt.Errorf("refusing token-bearing redirect to %s", request.URL.Redacted())
			}
			if previousCheckRedirect != nil {
				return previousCheckRedirect(request, via)
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		}
		requestClient = &clientCopy
	}
	query := parsedEndpoint.Query()
	query.Set("v", "1")
	query.Set("type", "groupsio_mailing_list")
	parsedEndpoint.RawQuery = query.Encode()

	var total, withDomain, withoutDomain, pages int
	seenTokens := make(map[string]struct{})
	pageToken := ""
	for {
		requestURL := *parsedEndpoint
		requestQuery := requestURL.Query()
		if pageToken != "" {
			requestQuery.Set("page_token", pageToken)
		} else {
			requestQuery.Del("page_token")
		}
		requestURL.RawQuery = requestQuery.Encode()

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return fmt.Errorf("create request for page %d: %w", pages+1, err)
		}
		request.Header.Set("Accept", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}

		response, err := requestClient.Do(request)
		if err != nil {
			return fmt.Errorf("request page %d: %w", pages+1, err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read page %d response: %w", pages+1, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close page %d response: %w", pages+1, closeErr)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("query service returned %s for page %d: %s", response.Status, pages+1, strings.TrimSpace(string(body)))
		}

		var currentPage page
		if err := json.Unmarshal(body, &currentPage); err != nil {
			return fmt.Errorf("decode page %d response: %w", pages+1, err)
		}
		if len(currentPage.Resources) == 0 {
			return fmt.Errorf("decode page %d response: missing resources field", pages+1)
		}
		if currentPage.Resources[0] != '[' {
			return fmt.Errorf("decode page %d response: resources must be an array", pages+1)
		}
		var resources []resource
		if err := json.Unmarshal(currentPage.Resources, &resources); err != nil {
			return fmt.Errorf("decode resources on page %d: %w", pages+1, err)
		}
		pages++
		for _, item := range resources {
			total++
			if strings.TrimSpace(item.Data.Domain) == "" {
				withoutDomain++
				continue
			}
			withDomain++
		}

		if currentPage.PageToken == "" {
			break
		}
		if _, exists := seenTokens[currentPage.PageToken]; exists {
			return fmt.Errorf("query service repeated pagination token after page %d", pages)
		}
		seenTokens[currentPage.PageToken] = struct{}{}
		pageToken = currentPage.PageToken
	}

	_, err = fmt.Fprintf(out, "Pages: %d\nVisible mailing lists: %d\nVisible mailing lists with domain: %d\nVisible mailing lists without domain: %d\n", pages, total, withDomain, withoutDomain)
	return err
}
