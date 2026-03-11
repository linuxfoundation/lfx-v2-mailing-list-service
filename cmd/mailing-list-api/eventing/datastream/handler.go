// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package datastream implements port.DataEventHandler for GroupsIO v1 DynamoDB
// change events sourced from the lfx-v1-sync-helper pipeline.
package datastream

import (
	"context"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// KV key prefixes matching lfx-v1-sync-helper's naming convention.
	kvPrefixService  = "itx-groupsio-v2-service."
	kvPrefixSubgroup = "itx-groupsio-v2-subgroup."
	kvPrefixMember   = "itx-groupsio-v2-member."

	// sdcDeletedAt is the field injected by lfx-v1-sync-helper on DynamoDB REMOVE events.
	sdcDeletedAt = "_sdc_deleted_at"
)

// eventHandler implements port.DataEventHandler and routes KV events to the
// appropriate per-entity handler based on the key prefix.
type eventHandler struct {
	publisher  port.MessagePublisher
	mappingsKV jetstream.KeyValue
}

// NewEventHandler constructs a DataEventHandler for GroupsIO entities.
// publisher is used to emit indexer and access control messages.
// mappingsKV is the v1-mappings KV bucket used for idempotency tracking.
func NewEventHandler(publisher port.MessagePublisher, mappingsKV jetstream.KeyValue) port.DataEventHandler {
	return &eventHandler{
		publisher:  publisher,
		mappingsKV: mappingsKV,
	}
}

// HandleChange dispatches a PUT event to the correct entity handler.
// Payloads containing _sdc_deleted_at are treated as soft-deletes.
func (h *eventHandler) HandleChange(ctx context.Context, key string, data map[string]any) bool {
	_, isSoftDelete := data[sdcDeletedAt]

	switch {
	case strings.HasPrefix(key, kvPrefixService):
		uid := key[len(kvPrefixService):]
		if isSoftDelete {
			return handleServiceDelete(ctx, uid, h.publisher, h.mappingsKV)
		}
		return handleServiceUpdate(ctx, uid, data, h.publisher, h.mappingsKV)

	case strings.HasPrefix(key, kvPrefixSubgroup):
		uid := key[len(kvPrefixSubgroup):]
		if isSoftDelete {
			return handleSubgroupDelete(ctx, uid, h.publisher, h.mappingsKV)
		}
		return handleSubgroupUpdate(ctx, uid, data, h.publisher, h.mappingsKV)

	case strings.HasPrefix(key, kvPrefixMember):
		uid := key[len(kvPrefixMember):]
		if isSoftDelete {
			return handleMemberDelete(ctx, uid, h.publisher, h.mappingsKV)
		}
		return handleMemberUpdate(ctx, uid, data, h.publisher, h.mappingsKV)

	default:
		slog.WarnContext(ctx, "unrecognized KV key prefix in HandleChange, ACKing", "key", key)
		return false
	}
}

// HandleRemoval dispatches a hard DELETE or PURGE event to the correct entity handler.
func (h *eventHandler) HandleRemoval(ctx context.Context, key string) bool {
	switch {
	case strings.HasPrefix(key, kvPrefixService):
		return handleServiceDelete(ctx, key[len(kvPrefixService):], h.publisher, h.mappingsKV)

	case strings.HasPrefix(key, kvPrefixSubgroup):
		return handleSubgroupDelete(ctx, key[len(kvPrefixSubgroup):], h.publisher, h.mappingsKV)

	case strings.HasPrefix(key, kvPrefixMember):
		return handleMemberDelete(ctx, key[len(kvPrefixMember):], h.publisher, h.mappingsKV)

	default:
		slog.WarnContext(ctx, "unrecognized KV key prefix in HandleRemoval, ACKing", "key", key)
		return false
	}
}
