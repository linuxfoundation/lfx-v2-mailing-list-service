// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	indexertypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
	"github.com/nats-io/nats.go/jetstream"
)

// HandleDataStreamServiceUpdate transforms the v1 payload into a GrpsIOService and publishes
// indexer + access control messages. Returns true to NAK on transient errors.
func HandleDataStreamServiceUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter, subgroups ...port.SubgroupObjectReader) bool {
	if len(subgroups) > 0 && subgroups[0] != nil {
		current, present, err := subgroups[0].GetService(ctx, uid)
		if err != nil {
			slog.WarnContext(ctx, "failed to read current service object, will retry", "uid", uid, "error", err)
			return true
		}
		if !present {
			return HandleDataStreamServiceDelete(ctx, uid, publisher, mappings)
		}
		data = current // a delayed delivery must not overwrite a newer service event
	}
	// Resolve v1 project SFID → v2 project UID via the shared project.sfid.{sfid} mapping
	// written by lfx-v1-sync-helper. NAK if the project hasn't been processed yet.
	projectSFID := mapconv.StringVal(data, "project_id")
	if projectSFID == "" {
		slog.WarnContext(ctx, "missing project_id in service event, discarding", "uid", uid)
		return false // ACK — malformed data, retrying won't help
	}
	projectUID, ok := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixProjectBySFID, projectSFID))
	if !ok {
		slog.WarnContext(ctx, "project mapping not yet available, NAKing service for retry",
			"uid", uid, "project_sfid", projectSFID)
		return true // NAK — retry with backoff
	}
	data["project_id"] = projectUID

	svc := transformV1ToGrpsIOService(uid, data)
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixService, uid)
	action := mappings.ResolveAction(ctx, mKey)
	domainKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixServiceDomain, uid)
	indexedDomainKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixServiceDomainIndexed, uid)
	indexedDomain, indexedPresent, err := mappings.GetMappingValueWithError(ctx, indexedDomainKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read propagated domain checkpoint, will retry", "uid", uid, "error", err)
		return true
	}
	// Invalidate before changing the cached domain. A later failed mapping
	// write can still allow subgroup indexing to see a new/cleared domain; a
	// revert must not mistake the old checkpoint for a completed fan-out.
	if !indexedPresent || indexedDomain != svc.Domain {
		if err := mappings.PutTombstone(ctx, indexedDomainKey); err != nil {
			slog.WarnContext(ctx, "failed to invalidate domain propagation checkpoint, will retry", "uid", uid, "mapping_key", indexedDomainKey, "error", err)
			return true
		}
	}

	isPublic := svc.Public
	svcRef := fmt.Sprintf("groupsio_service:%s", uid)
	indexingConfig := &indexertypes.IndexingConfig{
		ObjectID:             uid,
		Public:               &isPublic,
		AccessCheckObject:    svcRef,
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   svcRef,
		HistoryCheckRelation: "auditor",
		ParentRefs:           svc.ParentRefs(),
		NameAndAliases:       svc.NameAndAliases(),
		SortName:             svc.SortName(),
		Fulltext:             svc.Fulltext(),
		Tags:                 svc.Tags(),
	}

	msg := &model.IndexerMessage{Action: action, Tags: svc.Tags()}
	built, err := msg.BuildWithIndexingConfig(ctx, svc, indexingConfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build service indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOServiceSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish service indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	// Publish settings indexer message when writers or auditors are present.
	settings := buildServiceSettings(uid, data)
	if settings != nil {
		settingsRef := fmt.Sprintf("groupsio_service:%s", uid)
		settingsConfig := &indexertypes.IndexingConfig{
			ObjectID:             uid,
			AccessCheckObject:    settingsRef,
			AccessCheckRelation:  "auditor",
			HistoryCheckObject:   settingsRef,
			HistoryCheckRelation: "auditor",
			ParentRefs:           settings.ParentRefs(),
			Tags:                 settings.Tags(),
		}
		settingsMsg := &model.IndexerMessage{Action: action, Tags: settings.Tags()}
		builtSettings, errSettings := settingsMsg.BuildWithIndexingConfig(ctx, settings, settingsConfig)
		if errSettings != nil {
			slog.ErrorContext(ctx, "failed to build service settings indexer message", "uid", uid, "error", errSettings)
		}
		if errSettings == nil {
			if errPublish := publisher.Indexer(ctx, constants.IndexGroupsIOServiceSettingsSubject, builtSettings); errPublish != nil {
				slog.ErrorContext(ctx, "failed to publish service settings indexer message", "uid", uid, "error", errPublish)
			}
		}
	}

	relations := map[string][]string{}
	if settings != nil {
		if writers := userInfoUsernames(settings.Writers); len(writers) > 0 {
			relations[constants.RelationWriter] = writers
		}
		if auditors := userInfoUsernames(settings.Auditors); len(auditors) > 0 {
			relations[constants.RelationAuditor] = auditors
		}
	}
	accessData := fgatypes.GenericAccessData{
		UID:    uid,
		Public: svc.Public,
		References: map[string][]string{
			constants.RelationProject: {svc.ProjectUID},
		},
	}
	if len(relations) > 0 {
		accessData.Relations = relations
	}
	accessMsg := fgatypes.GenericFGAMessage{
		ObjectType: constants.ObjectTypeGroupsIOService,
		Operation:  "update_access",
		Data:       accessData,
	}
	if err := publisher.Access(ctx, fgaconstants.GenericUpdateAccessSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish service access message", "uid", uid, "error", err)
	}

	// Keep the service domain available to subgroup events, which denormalize it onto
	// mailing-list documents so consumers do not need access to the parent service.
	// Write it before publishing the service mapping, which makes the parent visible
	// to subgroup processing.
	domainWritten := false
	if err := mappings.PutMapping(ctx, domainKey, svc.Domain); err != nil {
		// An existing service mapping may already be visible to subgroups. Check
		// and clear a stale domain on every failed write, including transient ones.
		storedDomain, present, readErr := mappings.GetMappingValueWithError(ctx, domainKey)
		if readErr != nil {
			return retryUnsafeDomainMapping(ctx, uid, mKey, domainKey, err, readErr, mappings)
		}
		if present && storedDomain != svc.Domain {
			if clearErr := mappings.PutTombstone(ctx, domainKey); clearErr != nil {
				return retryUnsafeDomainMapping(ctx, uid, mKey, domainKey, err, clearErr, mappings)
			}
		}
		if !isPermanentDomainMappingError(err) {
			slog.WarnContext(ctx, "failed to put service domain mapping, will retry (repair required if deliveries are exhausted)",
				"uid", uid, "mapping_key", domainKey, "error", err)
			return true
		}
		slog.ErrorContext(ctx, "permanent service domain mapping failure, indexing service without a domain mapping",
			"uid", uid, "mapping_key", domainKey, "error", err)
	} else {
		domainWritten = true
	}
	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		if pkgerrors.IsTransient(err) {
			slog.WarnContext(ctx, "failed to put mapping key, will retry", "mapping_key", mKey, "error", err)
			return true
		}
		slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", mKey, "error", err)
		return false
	}
	if !domainWritten {
		return true // the cached domain and mailing-list documents still need repair
	}
	// The checkpoint is deliberately independent of the service domain mapping:
	// redelivery must retry the fan-out even after the domain mapping was updated.
	// Services predating the checkpoint get one refresh on their next event,
	// regardless of the create/update action inferred from the service mapping.
	if len(subgroups) == 0 || subgroups[0] == nil {
		slog.WarnContext(ctx, "subgroup object reader unavailable for domain propagation, will retry", "uid", uid)
		return true
	}
	if !indexedPresent || indexedDomain != svc.Domain {
		if propagateServiceDomain(ctx, uid, svc.Domain, publisher, mappings, subgroups[0]) {
			return true
		}
		if ctx.Err() != nil {
			return true
		}
		if err := mappings.PutMapping(ctx, indexedDomainKey, svc.Domain); err != nil {
			slog.WarnContext(ctx, "failed to checkpoint propagated service domain, will retry", "uid", uid, "mapping_key", indexedDomainKey, "error", err)
			return true
		}
	}
	return false
}

// isPermanentDomainMappingError recognizes NATS errors which cannot be fixed by
// immediate redelivery. Unknown errors are retried rather than ACKed as permanent.
func isPermanentDomainMappingError(err error) bool {
	if errors.Is(err, jetstream.ErrInvalidKey) {
		return true
	}
	var apiErr *jetstream.APIError
	return errors.As(err, &apiErr) && apiErr.Code >= 400 && apiErr.Code < 500 &&
		apiErr.Code != 408 && apiErr.Code != 429
}

// retryUnsafeDomainMapping prevents existing subgroups from reading a stale
// domain when its replacement or cleanup cannot be verified.
func retryUnsafeDomainMapping(ctx context.Context, uid, serviceKey, domainKey string, writeErr, repairErr error, mappings port.MappingReaderWriter) bool {
	if err := mappings.PutTombstone(ctx, serviceKey); err != nil {
		slog.ErrorContext(ctx, "failed to hide service mapping with unverified domain", "uid", uid,
			"mapping_key", serviceKey, "error", err)
	}
	slog.ErrorContext(ctx, "service domain mapping could not be repaired; NAKing for operational repair",
		"uid", uid, "mapping_key", domainKey, "write_error", writeErr, "repair_error", repairErr)
	return true
}

// HandleDataStreamServiceDelete publishes a delete indexer message and tombstones the mapping.
// Returns true to NAK on transient errors.
func HandleDataStreamServiceDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixService, uid)
	domainKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixServiceDomain, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		if err := mappings.PutTombstone(ctx, domainKey); err != nil {
			slog.ErrorContext(ctx, "failed to tombstone service domain mapping, will retry", "mapping_key", domainKey, "error", err)
			return true
		}
		if err := mappings.PutTombstone(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixServiceDomainIndexed, uid)); err != nil {
			slog.ErrorContext(ctx, "failed to tombstone propagated domain checkpoint, will retry", "uid", uid, "error", err)
			return true
		}
		slog.InfoContext(ctx, "service already deleted, ACKing duplicate", "uid", uid)
		return false
	}

	msg := &model.IndexerMessage{Action: model.ActionDeleted}
	built, err := msg.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build service delete indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOServiceSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish service delete indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	deleteMsg := fgatypes.GenericFGAMessage{
		ObjectType: constants.ObjectTypeGroupsIOService,
		Operation:  "delete_access",
		Data:       fgatypes.GenericDeleteData{UID: uid},
	}
	if err := publisher.Access(ctx, fgaconstants.GenericDeleteAccessSubject, deleteMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish service delete access message", "uid", uid, "error", err)
	}

	// Tombstone the domain first so a failed write cannot leave a live domain after
	// the service tombstone causes subsequent deliveries to take the duplicate path.
	domainTombstoneErr := mappings.PutTombstone(ctx, domainKey)
	if domainTombstoneErr != nil {
		slog.ErrorContext(ctx, "failed to tombstone service domain mapping, will retry", "mapping_key", domainKey, "error", domainTombstoneErr)
	}
	checkpointKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixServiceDomainIndexed, uid)
	checkpointErr := mappings.PutTombstone(ctx, checkpointKey)
	if checkpointErr != nil {
		slog.ErrorContext(ctx, "failed to tombstone propagated domain checkpoint, will retry", "mapping_key", checkpointKey, "error", checkpointErr)
	}
	serviceTombstoneErr := mappings.PutTombstone(ctx, mKey)
	if serviceTombstoneErr != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", serviceTombstoneErr)
	}
	return domainTombstoneErr != nil || checkpointErr != nil || serviceTombstoneErr != nil
}

// buildServiceSettings constructs a GrpsIOServiceSettings from v1 writers/auditors.
// Returns nil when both slices are empty (no settings message needed).
func buildServiceSettings(uid string, data map[string]any) *model.GrpsIOServiceSettings {
	writers := toUserInfoSlice(mapconv.StringSliceVal(data, "writers"))
	auditors := toUserInfoSlice(mapconv.StringSliceVal(data, "auditors"))
	if len(writers) == 0 && len(auditors) == 0 {
		return nil
	}
	return &model.GrpsIOServiceSettings{
		UID:      uid,
		Writers:  writers,
		Auditors: auditors,
	}
}

// transformV1ToGrpsIOService maps v1 DynamoDB fields to the GrpsIOService domain model.
// Source is always "v1-sync" to distinguish these from API-created records.
func transformV1ToGrpsIOService(uid string, data map[string]any) *model.GroupsIOService {
	svc := &model.GroupsIOService{
		UID:         uid,
		Type:        mapconv.StringVal(data, "group_service_type"),
		Domain:      mapconv.StringVal(data, "domain"),
		GroupID:     mapconv.Int64Ptr(data, "group_id"),
		Prefix:      mapconv.StringVal(data, "prefix"),
		ProjectUID:  mapconv.StringVal(data, "project_id"),
		ProjectSlug: mapconv.StringVal(data, "proj_id"),
		Source:      "v1-sync",
	}

	if ts := mapconv.StringVal(data, "created_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			svc.CreatedAt = t
		}
	}
	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			svc.UpdatedAt = t
		}
	}
	if ts := mapconv.StringVal(data, "last_system_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			svc.SystemUpdatedAt = t
		}
	}

	return svc
}
