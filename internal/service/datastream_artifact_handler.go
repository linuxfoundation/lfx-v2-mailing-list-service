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

// HandleDataStreamArtifactUpdate transforms the v1 payload into a GroupsIOArtifact and publishes
// an indexer message. No FGA access control is published for artifacts.
// Returns true to NAK when the parent subgroup mapping is absent (ordering guarantee)
// or on transient errors.
func HandleDataStreamArtifactUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	// Artifacts carry group_id (Groups.io numeric group ID). Resolve the parent
	// subgroup UID via the reverse index written by the subgroup handler.
	groupID := mapconv.Int64Ptr(data, "group_id")
	if groupID == nil {
		slog.ErrorContext(ctx, "artifact has no group_id, cannot determine parent mailing list — ACKing", "uid", uid)
		return false // ACK — malformed data, retrying won't help
	}

	gidKey := fmt.Sprintf("%s.%d", constants.KVMappingPrefixSubgroupByGroupID, *groupID)
	_, ok := mappings.GetMappingValue(ctx, gidKey)
	if !ok {
		slog.WarnContext(ctx, "parent subgroup not yet processed, NAKing artifact for retry",
			"uid", uid, "group_id", *groupID)
		return true // NAK — retry with backoff
	}

	// Resolve v1 project SFID → v2 project UID. NAK if not yet available.
	if projectSFID := mapconv.StringVal(data, "project_id"); projectSFID != "" {
		projectUID, ok := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixProjectBySFID, projectSFID))
		if !ok {
			slog.WarnContext(ctx, "project mapping not yet available, NAKing artifact for retry",
				"uid", uid, "project_sfid", projectSFID)
			return true // NAK — retry with backoff
		}
		data["project_id"] = projectUID
	}

	// Resolve optional v1 committee SFID → v2 committee UID. NAK if specified but not yet synced.
	if committeeSFID := mapconv.StringVal(data, "committee_id"); committeeSFID != "" {
		committeeUID, ok := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixCommitteeBySFID, committeeSFID))
		if !ok {
			slog.WarnContext(ctx, "committee mapping not yet available, NAKing artifact for retry",
				"uid", uid, "committee_sfid", committeeSFID)
			return true // NAK — retry with backoff
		}
		data["committee_id"] = committeeUID
	}

	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixArtifact, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "artifact mapping is tombstoned, skipping update", "uid", uid)
		return false
	}

	action := mappings.ResolveAction(ctx, mKey)

	artifact := transformV1ToGroupsIOArtifact(uid, data)

	isPublic := false
	groupRef := fmt.Sprintf("groupsio_artifact:%s", uid)
	indexingConfig := &indexertypes.IndexingConfig{
		ObjectID:             uid,
		Public:               &isPublic,
		AccessCheckObject:    groupRef,
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   groupRef,
		HistoryCheckRelation: "auditor",
		ParentRefs:           artifact.ParentRefs(),
		NameAndAliases:       artifact.NameAndAliases(),
		SortName:             artifact.SortName(),
		Fulltext:             artifact.Fulltext(),
		Tags:                 artifact.Tags(),
	}

	msg := &model.IndexerMessage{Action: action, Tags: artifact.Tags()}
	built, err := msg.BuildWithIndexingConfig(ctx, artifact, indexingConfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build artifact indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOArtifactSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish artifact indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", mKey, "error", err)
	}

	return false
}

// HandleDataStreamArtifactDelete publishes a delete indexer message and tombstones the mapping.
func HandleDataStreamArtifactDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixArtifact, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "artifact already deleted, ACKing duplicate", "uid", uid)
		return false
	}

	// If there is no mapping entry, this record was never indexed — nothing to delete.
	if !mappings.IsMappingPresent(ctx, mKey) {
		slog.InfoContext(ctx, "artifact was never indexed, skipping OpenSearch delete", "uid", uid)
		if err := mappings.PutTombstone(ctx, mKey); err != nil {
			slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
		}
		return false
	}

	msg := &model.IndexerMessage{Action: model.ActionDeleted}
	built, err := msg.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build artifact delete indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOArtifactSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish artifact delete indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	if err := mappings.PutTombstone(ctx, mKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
	}
	return false
}

// transformV1ToGroupsIOArtifact maps v1 DynamoDB fields to the GroupsIOArtifact domain model.
func transformV1ToGroupsIOArtifact(uid string, data map[string]any) *model.GroupsIOArtifact {
	artifact := &model.GroupsIOArtifact{
		ArtifactID:       uid,
		ProjectUID:  mapconv.StringVal(data, "project_id"),
		CommitteeUID: mapconv.StringVal(data, "committee_id"),
		Type:             mapconv.StringVal(data, "type"),
		MediaType:        mapconv.StringVal(data, "media_type"),
		Filename:         mapconv.StringVal(data, "filename"),
		LinkURL:          mapconv.StringVal(data, "link_url"),
		DownloadURL:      mapconv.StringVal(data, "download_url"),
		S3Key:            mapconv.StringVal(data, "s3_key"),
		FileUploadStatus: mapconv.StringVal(data, "file_upload_status"),
		Description:      mapconv.StringVal(data, "description"),
	}

	if gid := mapconv.Int64Ptr(data, "group_id"); gid != nil {
		artifact.GroupID = uint64(*gid)
	}

	if fu := mapconv.StringVal(data, "file_uploaded"); fu != "" {
		v := fu == "true"
		artifact.FileUploaded = &v
	}

	if ts := mapconv.StringVal(data, "created_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			artifact.CreatedAt = t
		}
	}
	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			artifact.UpdatedAt = t
		}
	}
	if ts := mapconv.StringVal(data, "file_uploaded_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			artifact.FileUploadedAt = &t
		}
	}
	if ts := mapconv.StringVal(data, "last_posted_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			artifact.LastPostedAt = &t
		}
	}

	return artifact
}
