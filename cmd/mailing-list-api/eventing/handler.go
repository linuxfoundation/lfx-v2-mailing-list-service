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
	objects       port.V1ObjectReader
	domainLock    port.ServiceDomainLock
}

// WithServiceDomainLock serializes service and subgroup processing by service UID.
func WithServiceDomainLock(lock port.ServiceDomainLock) EventHandlerOption {
	return func(eh *eventHandler) { eh.domainLock = lock }
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
// objects reads the current service and subgroup source records.
// Additional behaviour (e.g. LFID invite sending) can be added via options.
func NewEventHandler(publisher port.MessagePublisher, mappings port.MappingReaderWriter, projectLookup port.ProjectLookup, objects port.V1ObjectReader, opts ...EventHandlerOption) port.DataEventHandler {
	eh := &eventHandler{
		publisher:     publisher,
		mappings:      mappings,
		projectLookup: projectLookup,
		objects:       objects,
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
				// Both updates and delayed removals reconcile from the current source.
				return service.HandleDataStreamServiceUpdate(lockedCtx, uid, data, h.publisher, h.mappings, h.objects)
			})
		}
		return service.HandleDataStreamServiceUpdate(ctx, uid, data, h.publisher, h.mappings, h.objects)

	case kvPrefixSubgroup:
		if h.domainLock != nil {
			return h.domainLock.WithService(ctx, "subgroup-"+uid, func(lockedCtx context.Context) bool {
				return h.handleSubgroupChange(lockedCtx, uid, data, isSoftDelete)
			})
		}
		return h.handleSubgroupChange(ctx, uid, data, isSoftDelete)

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

func (h *eventHandler) handleSubgroupChange(ctx context.Context, uid string, data map[string]any, isSoftDelete bool) bool {
	if isSoftDelete {
		return h.removeSubgroupLocked(ctx, uid)
	}
	if h.domainLock != nil {
		if h.objects == nil {
			return true
		}
		current, present, err := h.objects.GetSubgroup(ctx, uid)
		if err != nil {
			slog.WarnContext(ctx, "failed to read current subgroup before locking", "uid", uid, "error", err)
			return true
		}
		if !present {
			return h.removeSubgroupLocked(ctx, uid)
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
				fresh, stillPresent, readErr := h.objects.GetSubgroup(lockedCtx, uid)
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
}

// clearSubgroupParent removes a stale service index without tombstoning the
// subgroup mapping, so a subsequent reparenting event can still be processed.
func (h *eventHandler) clearSubgroupParent(ctx context.Context, uid string) bool {
	if h.objects == nil || h.domainLock == nil {
		return true
	}
	parentKey := constants.KVMappingPrefixSubgroupParent + "." + uid
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + "." + uid
	pendingKey := constants.KVMappingPrefixSubgroupPreviousService + "." + uid
	previous, _, err := h.mappings.GetMappingValueWithError(ctx, parentKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read subgroup parent for cleanup", "uid", uid, "error", err)
		return true
	}
	published, _, err := h.mappings.GetMappingValueWithError(ctx, publishedKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read published subgroup parent for cleanup", "uid", uid, "error", err)
		return true
	}
	pending, _, err := h.mappings.GetMappingValueWithError(ctx, pendingKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read pending subgroup move for cleanup", "uid", uid, "error", err)
		return true
	}
	if previous == "" && published == "" && pending == "" {
		fresh, exists, readErr := h.objects.GetSubgroup(ctx, uid)
		if readErr != nil {
			return true
		}
		if !exists {
			return service.HandleDataStreamSubgroupDelete(ctx, uid, h.publisher, h.mappings)
		}
		if mapconv.StringVal(fresh, "parent_id") != "" {
			return true // retry under the current parent's service lock
		}
		// Legacy lists can have a live forward mapping and indexed document
		// without any service-index pointers. Only a completion marker proves
		// their UID-based unassociation cleanup already ran.
		unassociatedKey := constants.KVMappingPrefixSubgroupUnassociated + "." + uid
		completedUID, complete, readErr := h.mappings.GetMappingValueWithError(ctx, unassociatedKey)
		if readErr != nil {
			return true
		}
		progressKey := constants.KVMappingPrefixSubgroupUnassociationPending + "." + uid
		progressUID, pendingCleanup, readErr := h.mappings.GetMappingValueWithError(ctx, progressKey)
		if readErr != nil {
			return true
		}
		if pendingCleanup && progressUID != uid {
			slog.ErrorContext(ctx, "unexpected subgroup unassociation progress marker", "uid", uid, "mapping_value", progressUID)
			return true
		}
		if complete {
			if completedUID != uid {
				slog.ErrorContext(ctx, "unexpected subgroup unassociation marker", "uid", uid, "mapping_value", completedUID)
				return true
			}
			if !pendingCleanup {
				return service.HandleDataStreamSubgroupUnassociatedAccess(ctx, uid, fresh, h.publisher, h.mappings)
			}
		}
		if pendingCleanup {
			return service.HandleDataStreamSubgroupUnassociated(ctx, uid, "", fresh, h.publisher, h.mappings)
		}
		forwardKey := constants.KVMappingPrefixSubgroup + "." + uid
		forwardUID, live, readErr := h.mappings.GetMappingValueWithError(ctx, forwardKey)
		if readErr != nil {
			return true
		}
		if !live {
			return false
		}
		if forwardUID != uid {
			slog.ErrorContext(ctx, "unexpected subgroup mapping for unassociation", "uid", uid, "mapping_value", forwardUID)
			return true
		}
		return service.HandleDataStreamSubgroupUnassociated(ctx, uid, "", fresh, h.publisher, h.mappings)
	}
	parents := []string{}
	if previous != "" {
		parents = append(parents, previous)
	}
	if published != "" && published != previous {
		parents = append(parents, published)
	}
	if pending != "" && pending != previous && pending != published {
		parents = append(parents, pending)
	}
	return h.domainLock.WithServices(ctx, parents, func(lockedCtx context.Context) bool {
		fresh, exists, readErr := h.objects.GetSubgroup(lockedCtx, uid)
		if readErr != nil {
			return true
		}
		if !exists {
			return service.HandleDataStreamSubgroupDelete(lockedCtx, uid, h.publisher, h.mappings)
		}
		if mapconv.StringVal(fresh, "parent_id") != "" {
			return true
		}
		currentParent, _, readErr := h.mappings.GetMappingValueWithError(lockedCtx, parentKey)
		if readErr != nil || currentParent != previous {
			return true
		}
		currentPublished, _, readErr := h.mappings.GetMappingValueWithError(lockedCtx, publishedKey)
		if readErr != nil || currentPublished != published {
			return true
		}
		currentPending, _, readErr := h.mappings.GetMappingValueWithError(lockedCtx, pendingKey)
		if readErr != nil || currentPending != pending {
			return true
		}
		return service.HandleDataStreamSubgroupUnassociated(lockedCtx, uid, previous, fresh, h.publisher, h.mappings)
	})
}

// HandleRemoval dispatches a hard DELETE or PURGE event to the correct entity handler.
func (h *eventHandler) HandleRemoval(ctx context.Context, key string) bool {
	uid, prefix := extractUID(key)
	switch prefix {
	case kvPrefixService:
		if h.domainLock != nil {
			return h.domainLock.WithService(ctx, uid, func(lockedCtx context.Context) bool {
				return service.HandleDataStreamServiceUpdate(lockedCtx, uid, nil, h.publisher, h.mappings, h.objects)
			})
		}
		return service.HandleDataStreamServiceUpdate(ctx, uid, nil, h.publisher, h.mappings, h.objects)
	case kvPrefixSubgroup:
		if h.domainLock != nil {
			return h.domainLock.WithService(ctx, "subgroup-"+uid, func(lockedCtx context.Context) bool {
				return h.removeSubgroupLocked(lockedCtx, uid)
			})
		}
		return h.removeSubgroupLocked(ctx, uid)
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

// removeSubgroup locks every known parent and reconciles with the current source.
// A delayed removal must not tombstone a live subgroup that can be reparented.
func (h *eventHandler) removeSubgroupLocked(ctx context.Context, uid string) bool {
	if h.objects == nil {
		return true
	}
	if h.domainLock != nil {
		keys := []string{
			constants.KVMappingPrefixSubgroupParent + "." + uid,
			constants.KVMappingPrefixSubgroupPublishedParent + "." + uid,
			constants.KVMappingPrefixSubgroupPreviousService + "." + uid,
		}
		parents := make([]string, 0, len(keys))
		values := make([]string, len(keys))
		for i, key := range keys {
			value, _, err := h.mappings.GetMappingValueWithError(ctx, key)
			if err != nil {
				slog.WarnContext(ctx, "failed to read subgroup parent before deletion", "uid", uid, "mapping_key", key, "error", err)
				return true
			}
			values[i] = value
			if value != "" {
				parents = append(parents, value)
			}
		}
		if len(parents) > 0 {
			return h.domainLock.WithServices(ctx, parents, func(lockedCtx context.Context) bool {
				for i, key := range keys {
					current, _, readErr := h.mappings.GetMappingValueWithError(lockedCtx, key)
					if readErr != nil || current != values[i] {
						return true
					}
				}
				fresh, exists, err := h.objects.GetSubgroup(lockedCtx, uid)
				if err != nil {
					return true
				}
				if exists {
					if mapconv.StringVal(fresh, "parent_id") != "" {
						return false // stale removal; the current subgroup is associated
					}
					return service.HandleDataStreamSubgroupUnassociated(lockedCtx, uid, values[0], fresh, h.publisher, h.mappings)
				}
				return service.HandleDataStreamSubgroupDelete(lockedCtx, uid, h.publisher, h.mappings)
			})
		}
	}
	fresh, exists, err := h.objects.GetSubgroup(ctx, uid)
	if err != nil {
		return true
	}
	if exists {
		if mapconv.StringVal(fresh, "parent_id") != "" {
			return false // stale removal; do not tombstone a live subgroup
		}
		if h.domainLock != nil {
			return h.clearSubgroupParent(ctx, uid)
		}
		return false // a live parentless subgroup has no service index to clean
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
