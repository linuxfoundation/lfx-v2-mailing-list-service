// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// backfill_subgroup_service_index populates the dedicated
// groupsio-subgroup-service-index KV bucket from existing v1-objects records,
// checking v1-mappings to skip unprocessed subgroups. It does not replay KV
// events or send indexer messages. Write mode invalidates affected service
// propagation checkpoints for a subsequent service replay. By default it only
// reports planned writes; pass -write to persist mappings.
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
	progressKey := constants.KVMappingPrefixSubgroupUnassociationPending + "." + uid
	progressUID, cleanupPending, err := store.GetMappingValueWithError(ctx, progressKey)
	if err != nil {
		return false, fmt.Errorf("read unassociation progress %s: %w", progressKey, err)
	}
	if cleanupPending {
		return false, fmt.Errorf("subgroup %s has unfinished unassociation %s (value %q); reconcile through subgroup event processing before backfill", uid, progressKey, progressUID)
	}
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
	if data == nil {
		return false, errors.New("decode subgroup JSON/msgpack: null object")
	}
	if _, deleted := data[constants.KVObjectSoftDeletedAt]; deleted {
		return false, nil
	}
	parentKey := constants.KVMappingPrefixSubgroupParent + "." + uid
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + "." + uid
	pendingKey := constants.KVMappingPrefixSubgroupPreviousService + "." + uid
	serviceUID, ok := data["parent_id"].(string)
	if !ok && data["parent_id"] != nil {
		return false, fmt.Errorf("invalid parent_id type %T", data["parent_id"])
	}
	if serviceUID == "" {
		// Unassociation retains the forward mapping for later reparenting.
		// Missing pointers alone do not prove a legacy indexed list was cleaned.
		for _, pointer := range []string{parentKey, publishedKey, pendingKey} {
			value, present, readErr := store.GetMappingValueWithError(ctx, pointer)
			if readErr != nil {
				return false, fmt.Errorf("read unassociated subgroup pointer %s: %w", pointer, readErr)
			}
			if present {
				return false, fmt.Errorf("missing parent_id with uncleared %s %q", pointer, value)
			}
		}
		completionKey := constants.KVMappingPrefixSubgroupUnassociated + "." + uid
		completedUID, complete, readErr := store.GetMappingValueWithError(ctx, completionKey)
		if readErr != nil {
			return false, fmt.Errorf("read unassociation completion %s: %w", completionKey, readErr)
		}
		if !complete || completedUID != uid {
			return false, fmt.Errorf("missing parent_id without confirmed unassociation cleanup for %s", uid)
		}
		return false, nil
	}
	if !constants.ValidKVKeySegment(serviceUID) {
		return false, fmt.Errorf("invalid parent_id %q", serviceUID)
	}
	previousParent, present, err := store.GetMappingValueWithError(ctx, parentKey)
	if err != nil {
		return false, err
	}
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
	completionKey := constants.KVMappingPrefixSubgroupUnassociated + "." + uid
	completedUID, unassociated, err := store.GetMappingValueWithError(ctx, completionKey)
	if err != nil {
		return false, fmt.Errorf("read unassociation completion %s: %w", completionKey, err)
	}
	if unassociated {
		return false, fmt.Errorf("associated subgroup %s still has unassociation marker %s (value %q); reconcile its access through subgroup event processing before backfill", uid, completionKey, completedUID)
	}
	if present && previousParent == serviceUID && indexed && !pending {
		return false, nil
	}
	slog.InfoContext(ctx, "backfill subgroup service mapping", "uid", uid, "service_uid", serviceUID, "write", write)
	if !write {
		return true, nil
	}
	// An earlier domain event may have checkpointed an empty service lookup.
	// Invalidate before exposing new mappings, so a replay with the unchanged
	// domain still refreshes this mailing list. Run while eventing is quiescent.
	checkpointKey := constants.KVMappingPrefixServiceDomainIndexed + "." + serviceUID
	if _, err := mappings.Put(ctx, checkpointKey, []byte(constants.KVTombstoneMarker)); err != nil {
		return false, fmt.Errorf("invalidate service domain checkpoint: %w", err)
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
