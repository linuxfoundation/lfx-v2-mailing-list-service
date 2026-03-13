// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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

	msg := &model.IndexerMessage{Action: action, Tags: svc.Tags()}
	built, err := msg.Build(ctx, svc)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build service indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOServiceSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish service indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	accessMsg := &model.AccessMessage{
		UID:        uid,
		ObjectType: constants.ObjectTypeGroupsIOService,
		Public:     svc.Public,
		References: map[string][]string{
			constants.RelationProject: {svc.ProjectUID},
		},
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

// transformV1ToGrpsIOService maps v1 DynamoDB fields to the GrpsIOService domain model.
// Source is always "v1-sync" to distinguish these from API-created records.
func transformV1ToGrpsIOService(uid string, data map[string]any) *model.GrpsIOService {
	svc := &model.GrpsIOService{
		UID:        uid,
		Type:       mapconv.StringVal(data, "group_service_type"),
		Domain:     mapconv.StringVal(data, "domain"),
		GroupID:    mapconv.Int64Ptr(data, "group_id"),
		Prefix:     mapconv.StringVal(data, "prefix"),
		ProjectUID: mapconv.StringVal(data, "project_id"),
		Source:     "v1-sync",
	}

	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			svc.UpdatedAt = t
		}
	}

	return svc
}
