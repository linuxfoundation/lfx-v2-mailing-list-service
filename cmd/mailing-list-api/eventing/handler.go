// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
)

// EventHandlerOption is a functional option for configuring eventHandler.
type EventHandlerOption func(*eventHandler)

const (
	// KV key prefixes matching lfx-v1-sync-helper's naming convention.
	kvPrefixService  = "itx-groupsio-v2-service."
	kvPrefixSubgroup = "itx-groupsio-v2-subgroup."
	kvPrefixMember   = "itx-groupsio-v2-member."
	kvPrefixArtifact = "itx-groupsio-v2-artifact."
	kvPrefixMessage  = "itx-groupsio-v2-message."

	// sdcDeletedAt is the field injected by lfx-v1-sync-helper on DynamoDB REMOVE events.
	sdcDeletedAt = "_sdc_deleted_at"
)

// eventHandler implements port.DataEventHandler and routes KV events to the
// appropriate per-entity handler based on the key prefix.
type eventHandler struct {
	publisher     port.MessagePublisher
	mappings      port.MappingReaderWriter
	projectLookup port.ProjectLookup
	memberInvite  *service.MemberInviteHandler
}

// WithMemberInviteHandler returns an EventHandlerOption that wires up the LFID invite
// handler for mailing-list members. When this option is not provided (or the handler
// is nil), invite sending is disabled and the event processor runs without it.
func WithMemberInviteHandler(h *service.MemberInviteHandler) EventHandlerOption {
	return func(eh *eventHandler) {
		eh.memberInvite = h
	}
}

// NewEventHandler constructs a DataEventHandler for GroupsIO entities.
// publisher is used to emit indexer and access control messages.
// mappings is the v1-mappings abstraction used for idempotency tracking.
// projectLookup is used by the subgroup handler to fetch the project slug.
// Additional behaviour (e.g. LFID invite sending) can be added via options.
func NewEventHandler(publisher port.MessagePublisher, mappings port.MappingReaderWriter, projectLookup port.ProjectLookup, opts ...EventHandlerOption) port.DataEventHandler {
	eh := &eventHandler{
		publisher:     publisher,
		mappings:      mappings,
		projectLookup: projectLookup,
	}
	for _, opt := range opts {
		opt(eh)
	}
	return eh
}

// HandleChange dispatches a PUT event to the correct entity handler.
// Payloads containing _sdc_deleted_at are treated as soft-deletes.
func (h *eventHandler) HandleChange(ctx context.Context, key string, data map[string]any) bool {
	_, isSoftDelete := data[sdcDeletedAt]

	uid, prefix := extractUID(key)
	switch prefix {
	case kvPrefixService:
		if isSoftDelete {
			return service.HandleDataStreamServiceDelete(ctx, uid, h.publisher, h.mappings)
		}
		return service.HandleDataStreamServiceUpdate(ctx, uid, data, h.publisher, h.mappings)

	case kvPrefixSubgroup:
		if isSoftDelete {
			return service.HandleDataStreamSubgroupDelete(ctx, uid, h.publisher, h.mappings)
		}
		return service.HandleDataStreamSubgroupUpdate(ctx, uid, data, h.publisher, h.mappings, h.projectLookup)

	case kvPrefixMember:
		if isSoftDelete {
			return service.HandleDataStreamMemberDelete(ctx, uid, h.publisher, h.mappings)
		}
		return service.HandleDataStreamMemberUpdate(ctx, uid, data, h.publisher, h.mappings, h.memberInvite)

	case kvPrefixArtifact:
		if isSoftDelete {
			return service.HandleDataStreamArtifactDelete(ctx, uid, h.publisher, h.mappings)
		}
		return service.HandleDataStreamArtifactUpdate(ctx, uid, data, h.publisher, h.mappings)

	case kvPrefixMessage:
		if isSoftDelete {
			return service.HandleDataStreamMessageDelete(ctx, uid, h.publisher, h.mappings)
		}
		return service.HandleDataStreamMessageUpdate(ctx, uid, data, h.publisher, h.mappings)

	default:
		slog.WarnContext(ctx, "unrecognized KV key prefix in HandleChange, ACKing", "key", key)
		return false
	}
}

// HandleRemoval dispatches a hard DELETE or PURGE event to the correct entity handler.
func (h *eventHandler) HandleRemoval(ctx context.Context, key string) bool {
	uid, prefix := extractUID(key)
	switch prefix {
	case kvPrefixService:
		return service.HandleDataStreamServiceDelete(ctx, uid, h.publisher, h.mappings)
	case kvPrefixSubgroup:
		return service.HandleDataStreamSubgroupDelete(ctx, uid, h.publisher, h.mappings)
	case kvPrefixMember:
		return service.HandleDataStreamMemberDelete(ctx, uid, h.publisher, h.mappings)
	case kvPrefixArtifact:
		return service.HandleDataStreamArtifactDelete(ctx, uid, h.publisher, h.mappings)
	case kvPrefixMessage:
		return service.HandleDataStreamMessageDelete(ctx, uid, h.publisher, h.mappings)
	default:
		slog.WarnContext(ctx, "unrecognized KV key prefix in HandleRemoval, ACKing", "key", key)
		return false
	}
}

// extractUID returns the entity UID and matched prefix for the given KV key.
// If no known prefix matches, prefix is "" and uid equals the full key.
func extractUID(key string) (uid, prefix string) {
	for _, p := range []string{
		kvPrefixService,
		kvPrefixSubgroup,
		kvPrefixMember,
		kvPrefixArtifact,
		kvPrefixMessage,
	} {
		if strings.HasPrefix(key, p) {
			return key[len(p):], p
		}
	}
	return key, ""
}
