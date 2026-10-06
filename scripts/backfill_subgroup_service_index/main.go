// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// backfill_subgroup_service_index populates the dedicated
// groupsio-subgroup-service-index KV bucket from existing v1-objects records,
// checking v1-mappings to skip unprocessed subgroups. It does not replay KV
// events or send indexer messages. By default it only reports planned writes;
// pass -write to persist mappings.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const subgroupPrefix = constants.KVObjectPrefixSubgroup

type results struct {
	listed, processed, skipped, failed int
}

func main() {
	write := flag.Bool("write", false, "write missing mappings (default: dry run)")
	flag.Parse()
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	conn, err := nats.Connect(natsURL, nats.Timeout(10*time.Second))
	if err != nil {
		slog.Error("connect to NATS", "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	js, err := jetstream.New(conn)
	if err != nil {
		slog.Error("create JetStream context", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	objects, err := js.KeyValue(ctx, constants.KVBucketV1Objects)
	if err != nil {
		slog.Error("open v1-objects", "error", err)
		os.Exit(1)
	}
	mappings, err := js.KeyValue(ctx, constants.KVBucketNameV1Mappings)
	if err != nil {
		slog.Error("open v1-mappings", "error", err)
		os.Exit(1)
	}
	serviceIndex, err := js.KeyValue(ctx, constants.KVBucketSubgroupServiceIndex)
	if err != nil {
		slog.Error("open subgroup service index", "error", err)
		os.Exit(1)
	}
	counts, err := backfill(ctx, objects, mappings, serviceIndex, *write)
	slog.Info("backfill complete", "write", *write, "listed", counts.listed,
		"processed", counts.processed, "skipped", counts.skipped, "failed", counts.failed)
	if err != nil {
		slog.Error("backfill failed", "error", err)
		os.Exit(1)
	}
}

func backfill(ctx context.Context, objects, mappings, serviceIndex jetstream.KeyValue, write bool) (results, error) {
	var counts results
	lister, err := objects.ListKeysFiltered(ctx, subgroupPrefix+">")
	if errors.Is(err, jetstream.ErrNoKeysFound) {
		return counts, nil
	}
	if err != nil {
		return counts, fmt.Errorf("list subgroup keys: %w", err)
	}
	defer lister.Stop() //nolint:errcheck
	seen := make(map[string]bool)
	for key := range lister.Keys() {
		if !strings.HasPrefix(key, subgroupPrefix) || seen[key] {
			continue
		}
		seen[key] = true
		counts.listed++
		needed, err := backfillOne(ctx, objects, mappings, serviceIndex, key, write)
		if err != nil {
			counts.failed++
			slog.ErrorContext(ctx, "could not process subgroup", "key", key, "error", err)
		} else if needed {
			counts.processed++
		} else {
			counts.skipped++
		}
	}
	if err := ctx.Err(); err != nil {
		return counts, err
	}
	if counts.failed > 0 {
		return counts, fmt.Errorf("%d subgroup records failed", counts.failed)
	}
	return counts, nil
}

func backfillOne(ctx context.Context, objects, mappings, serviceIndex jetstream.KeyValue, key string, write bool) (bool, error) {
	uid := strings.TrimPrefix(key, subgroupPrefix)
	if !constants.ValidKVKeySegment(uid) {
		return false, fmt.Errorf("invalid subgroup UID %q", uid)
	}
	forwardKey := constants.KVMappingPrefixSubgroup + "." + uid
	store := infraNATS.NewMappingReaderWriter(mappings, serviceIndex)
	forward, present, err := store.GetMappingValueWithError(ctx, forwardKey)
	if err != nil {
		return false, fmt.Errorf("read mapping %s: %w", forwardKey, err)
	}
	if !present || forward != uid {
		return false, nil // not indexed (or deleted) by the service
	}
	entry, err := objects.Get(ctx, key)
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read subgroup: %w", err)
	}
	data, err := mapconv.DecodeMapData(entry.Value())
	if err != nil {
		return false, fmt.Errorf("decode subgroup JSON/msgpack: %w", err)
	}
	if _, deleted := data[constants.KVObjectSoftDeletedAt]; deleted {
		return false, nil
	}
	serviceUID, ok := data["parent_id"].(string)
	if !ok || serviceUID == "" {
		return false, fmt.Errorf("missing parent_id")
	}
	if !constants.ValidKVKeySegment(serviceUID) {
		return false, fmt.Errorf("invalid parent_id %q", serviceUID)
	}
	parentKey := constants.KVMappingPrefixSubgroupParent + "." + uid
	previousParent, present, err := store.GetMappingValueWithError(ctx, parentKey)
	if err != nil {
		return false, err
	}
	pendingKey := constants.KVMappingPrefixSubgroupPreviousService + "." + uid
	pendingService, pending, err := store.GetMappingValueWithError(ctx, pendingKey)
	if err != nil {
		return false, err
	}
	if present && previousParent != serviceUID && (!pending || pendingService != previousParent) {
		return false, fmt.Errorf("existing parent %q differs from object parent %q", previousParent, serviceUID)
	}
	if pending && !constants.ValidKVKeySegment(pendingService) {
		return false, fmt.Errorf("invalid pending service UID %q", pendingService)
	}
	indexKey := constants.KVMappingPrefixSubgroupByService + "." + serviceUID + "." + uid
	indexedUID, indexed, err := store.GetMappingValueWithError(ctx, indexKey)
	if err != nil {
		return false, err
	}
	if indexed && indexedUID != uid {
		return false, fmt.Errorf("service index %s has unexpected UID %q", indexKey, indexedUID)
	}
	if present && previousParent == serviceUID && indexed && !pending {
		return false, nil
	}
	slog.InfoContext(ctx, "backfill subgroup service mapping", "uid", uid, "service_uid", serviceUID, "write", write)
	if !write {
		return true, nil
	}
	// Write the index first; readers verify the parent pointer and forward mapping.
	if !indexed {
		if _, err := serviceIndex.Put(ctx, indexKey, []byte(uid)); err != nil {
			return false, fmt.Errorf("write service index: %w", err)
		}
	}
	if !present || previousParent != serviceUID {
		if _, err := serviceIndex.Put(ctx, parentKey, []byte(serviceUID)); err != nil {
			return false, fmt.Errorf("write subgroup parent: %w", err)
		}
	}
	if pending {
		oldIndexKey := constants.KVMappingPrefixSubgroupByService + "." + pendingService + "." + uid
		if _, err := serviceIndex.Put(ctx, oldIndexKey, []byte(constants.KVTombstoneMarker)); err != nil {
			return false, fmt.Errorf("remove previous service index: %w", err)
		}
		if _, err := serviceIndex.Put(ctx, pendingKey, []byte(constants.KVTombstoneMarker)); err != nil {
			return false, fmt.Errorf("clear pending subgroup move: %w", err)
		}
	}
	return true, nil
}
