// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	indexertypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
)

// HandleDataStreamServiceUpdate transforms the v1 payload into a GrpsIOService and publishes
// indexer + access control messages. Returns true to NAK on transient errors.
func HandleDataStreamServiceUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	// Resolve v1 project SFID → v2 project UID via the shared project.sfid.{sfid} mapping
	// written by lfx-v1-sync-helper. NAK if the project hasn't been processed yet.
	projectSFID := mapconv.StringVal(data, "project_id")
	if projectSFID == "" {
		slog.ErrorContext(ctx, "missing project_id in service event, discarding", "uid", uid)
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

	references := map[string][]string{
		constants.RelationProject: {svc.ProjectUID},
	}
	if settings != nil {
		if writers := userInfoUsernames(settings.Writers); len(writers) > 0 {
			references[constants.RelationWriter] = writers
		}
		if auditors := userInfoUsernames(settings.Auditors); len(auditors) > 0 {
			references[constants.RelationAuditor] = auditors
		}
	}
	accessMsg := &model.AccessMessage{
		UID:        uid,
		ObjectType: constants.ObjectTypeGroupsIOService,
		Public:     svc.Public,
		References: references,
	}
	if err := publisher.Access(ctx, constants.UpdateAccessGroupsIOServiceSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish service access message", "uid", uid, "error", err)
	}

	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", mKey, "error", err)
	}
	return false
}

// HandleDataStreamServiceDelete publishes a delete indexer message and tombstones the mapping.
// Returns true to NAK on transient errors.
func HandleDataStreamServiceDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixService, uid)

	if mappings.IsTombstoned(ctx, mKey) {
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

	if err := publisher.Access(ctx, constants.DeleteAllAccessGroupsIOServiceSubject, uid); err != nil {
		slog.WarnContext(ctx, "failed to publish service delete access message", "uid", uid, "error", err)
	}

	if err := mappings.PutTombstone(ctx, mKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
	}
	return false
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
