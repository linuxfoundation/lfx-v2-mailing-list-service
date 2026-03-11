// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package datastream

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
)

// buildMappingKey returns the v1-mappings KV key for a given prefix and UID.
func buildMappingKey(prefix, uid string) string {
	return fmt.Sprintf("%s.%s", prefix, uid)
}

// resolveAction queries the v1-mappings KV bucket to decide whether this event
// represents a create or an update.
//
// A missing key or a tombstone ("!del") entry both yield ActionCreated — the
// former because the record has never been seen, the latter because the entity
// was previously deleted and is now being re-created.
func resolveAction(ctx context.Context, mappingsKV jetstream.KeyValue, mappingKey string) model.MessageAction {
	entry, err := mappingsKV.Get(ctx, mappingKey)
	if err != nil || entry == nil {
		return model.ActionCreated
	}
	if string(entry.Value()) == constants.KVTombstoneMarker {
		return model.ActionCreated
	}
	return model.ActionUpdated
}

// isMappingPresent reports whether a mapping key exists and is not tombstoned.
// Used for parent dependency checks (service before subgroup, subgroup before member).
func isMappingPresent(ctx context.Context, mappingsKV jetstream.KeyValue, mappingKey string) bool {
	entry, err := mappingsKV.Get(ctx, mappingKey)
	if err != nil || entry == nil {
		return false
	}
	return string(entry.Value()) != constants.KVTombstoneMarker
}

// isTombstoned reports whether the v1-mappings entry for mappingKey has already
// been marked as deleted, so that duplicate delete events can be skipped.
func isTombstoned(ctx context.Context, mappingsKV jetstream.KeyValue, mappingKey string) bool {
	entry, err := mappingsKV.Get(ctx, mappingKey)
	if err != nil || entry == nil {
		return false
	}
	return string(entry.Value()) == constants.KVTombstoneMarker
}

// putMapping records that uid has been successfully processed so subsequent
// events for the same key are treated as updates rather than creates.
func putMapping(ctx context.Context, mappingsKV jetstream.KeyValue, mappingKey, uid string) {
	_, _ = mappingsKV.Put(ctx, mappingKey, []byte(uid))
}

// putTombstone writes the deletion marker into v1-mappings to prevent duplicate
// processing of the same delete on consumer redelivery.
func putTombstone(ctx context.Context, mappingsKV jetstream.KeyValue, mappingKey string) {
	_, _ = mappingsKV.Put(ctx, mappingKey, []byte(constants.KVTombstoneMarker))
}
