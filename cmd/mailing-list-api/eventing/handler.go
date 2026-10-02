// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
)

// EventHandlerOption is a functional option for configuring eventHandler.
type EventHandlerOption func(*eventHandler)

const (
	// KV key prefixes matching lfx-v1-sync-helper's naming convention.
	kvPrefixService  = constants.KVObjectPrefixService
	kvPrefixSubgroup = constants.KVObjectPrefixSubgroup
	kvPrefixMember   = "itx-groupsio-v2-member."
	kvPrefixArtifact = "itx-groupsio-v2-artifact."
	kvPrefixMessage  = "itx-groupsio-v2-message."

	// sdcDeletedAt is the field injected by lfx-v1-sync-helper on DynamoDB REMOVE events.
	sdcDeletedAt = constants.KVObjectSoftDeletedAt
)

// eventHandler implements port.DataEventHandler and routes KV events to the
// appropriate per-entity handler based on the key prefix.
type eventHandler struct {
	publisher     port.MessagePublisher
	mappings      port.MappingReaderWriter
	projectLookup port.ProjectLookup
	memberInvite  *service.MemberInviteHandler
	subgroups     port.SubgroupObjectReader
	domainLock    port.ServiceDomainLock
}

// WithServiceDomainLock serializes service and subgroup processing by service UID.
func WithServiceDomainLock(lock port.ServiceDomainLock) EventHandlerOption {
	return func(eh *eventHandler) { eh.domainLock = lock }
}

// WithSubgroupObjectReader enables service-domain propagation using current subgroup objects.
func WithSubgroupObjectReader(reader port.SubgroupObjectReader) EventHandlerOption {
	return func(eh *eventHandler) { eh.subgroups = reader }
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
		if h.domainLock != nil {
			return h.domainLock.WithService(ctx, uid, func(lockedCtx context.Context) bool {
				if isSoftDelete {
					return service.HandleDataStreamServiceDelete(lockedCtx, uid, h.publisher, h.mappings)
				}
				return service.HandleDataStreamServiceUpdate(lockedCtx, uid, data, h.publisher, h.mappings, h.subgroups)
			})
		}
		if isSoftDelete {
			return service.HandleDataStreamServiceDelete(ctx, uid, h.publisher, h.mappings)
		}
		return service.HandleDataStreamServiceUpdate(ctx, uid, data, h.publisher, h.mappings, h.subgroups)

	case kvPrefixSubgroup:
		if isSoftDelete {
			return h.removeSubgroup(ctx, uid)
		}
		if h.domainLock != nil && !isSoftDelete {
			if h.subgroups == nil {
				return true
			}
			current, present, err := h.subgroups.GetSubgroup(ctx, uid)
			if err != nil {
				slog.WarnContext(ctx, "failed to read current subgroup before locking", "uid", uid, "error", err)
				return true
			}
			if !present {
				return h.removeSubgroup(ctx, uid)
			}
			if parent := mapconv.StringVal(current, "parent_id"); parent != "" {
				previous, _, readErr := h.mappings.GetMappingValueWithError(ctx, constants.KVMappingPrefixSubgroupParent+"."+uid)
				if readErr != nil {
					return true
				}
				parents := []string{parent}
				if previous != "" && previous != parent {
					parents = append(parents, previous)
				}
				return h.domainLock.WithServices(ctx, parents, func(lockedCtx context.Context) bool {
					fresh, stillPresent, readErr := h.subgroups.GetSubgroup(lockedCtx, uid)
					if readErr != nil {
						return true
					}
					if !stillPresent {
						return service.HandleDataStreamSubgroupDelete(lockedCtx, uid, h.publisher, h.mappings)
					}
					if mapconv.StringVal(fresh, "parent_id") != parent {
						return true // retry with the current parent, never publish under the old lock
					}
					currentParent, _, readErr := h.mappings.GetMappingValueWithError(lockedCtx, constants.KVMappingPrefixSubgroupParent+"."+uid)
					if readErr != nil || currentParent != previous {
						return true // parent changed before both locks were acquired
					}
					return service.HandleDataStreamSubgroupUpdate(lockedCtx, uid, fresh, h.publisher, h.mappings, h.projectLookup)
				})
			}
			return h.clearSubgroupParent(ctx, uid)
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

// clearSubgroupParent removes a stale service index without tombstoning the
// subgroup mapping, so a subsequent reparenting event can still be processed.
func (h *eventHandler) clearSubgroupParent(ctx context.Context, uid string) bool {
	parentKey := constants.KVMappingPrefixSubgroupParent + "." + uid
	previous, present, err := h.mappings.GetMappingValueWithError(ctx, parentKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read subgroup parent for cleanup", "uid", uid, "error", err)
		return true
	}
	if !present {
		return false
	}
	return h.domainLock.WithService(ctx, previous, func(lockedCtx context.Context) bool {
		fresh, exists, readErr := h.subgroups.GetSubgroup(lockedCtx, uid)
		if readErr != nil || !exists || mapconv.StringVal(fresh, "parent_id") != "" {
			return true
		}
		currentParent, _, readErr := h.mappings.GetMappingValueWithError(lockedCtx, parentKey)
		if readErr != nil || currentParent != previous {
			return true
		}
		return service.HandleDataStreamSubgroupUnassociated(lockedCtx, uid, previous, h.publisher, h.mappings)
	})
}

// HandleRemoval dispatches a hard DELETE or PURGE event to the correct entity handler.
func (h *eventHandler) HandleRemoval(ctx context.Context, key string) bool {
	uid, prefix := extractUID(key)
	switch prefix {
	case kvPrefixService:
		if h.domainLock != nil {
			return h.domainLock.WithService(ctx, uid, func(lockedCtx context.Context) bool {
				return service.HandleDataStreamServiceDelete(lockedCtx, uid, h.publisher, h.mappings)
			})
		}
		return service.HandleDataStreamServiceDelete(ctx, uid, h.publisher, h.mappings)
	case kvPrefixSubgroup:
		return h.removeSubgroup(ctx, uid)
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

// removeSubgroup uses the stored parent to serialize deletion with service fan-out.
func (h *eventHandler) removeSubgroup(ctx context.Context, uid string) bool {
	if h.domainLock != nil {
		parent, present, err := h.mappings.GetMappingValueWithError(ctx, constants.KVMappingPrefixSubgroupParent+"."+uid)
		if err != nil {
			slog.WarnContext(ctx, "failed to read subgroup parent before deletion", "uid", uid, "error", err)
			return true
		}
		if present {
			return h.domainLock.WithService(ctx, parent, func(lockedCtx context.Context) bool {
				return service.HandleDataStreamSubgroupDelete(lockedCtx, uid, h.publisher, h.mappings)
			})
		}
	}
	return service.HandleDataStreamSubgroupDelete(ctx, uid, h.publisher, h.mappings)
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
