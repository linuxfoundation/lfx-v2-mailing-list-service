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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
)

// HandleDataStreamArtifactUpdate transforms the v1 payload into a GroupsIOArtifact and publishes
// an indexer message. No FGA access control is published for artifacts.
// Returns true to NAK when the parent subgroup mapping is absent (ordering guarantee)
// or when a storage or publish step fails with anything not known to be permanent.
func HandleDataStreamArtifactUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	return handleArtifactUpdate(ctx, uid, data, publisher, mappings).requeue()
}

func handleArtifactUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) outcome {
	// Artifacts carry group_id (Groups.io numeric group ID). Resolve the parent
	// subgroup UID via the reverse index written by the subgroup handler.
	groupID := mapconv.Int64Ptr(data, "group_id")
	if groupID == nil {
		slog.ErrorContext(ctx, "artifact has no group_id, cannot determine parent mailing list — ACKing", "uid", uid)
		return ack // malformed data, retrying won't help
	}

	gidKey := fmt.Sprintf("%s.%d", constants.KVMappingPrefixSubgroupByGroupID, *groupID)
	gidEntry, err := mappings.Lookup(ctx, gidKey)
	if err != nil {
		return failed(ctx, "failed to look up parent subgroup mapping", err, "uid", uid, "group_id", *groupID)
	}
	if !gidEntry.Present() {
		slog.WarnContext(ctx, "parent subgroup not yet processed, NAKing artifact for retry",
			"uid", uid, "group_id", *groupID)
		return nak // retry with backoff
	}
	mailingListUID := gidEntry.Value

	// Resolve v1 project SFID → v2 project UID. NAK if not yet available.
	if projectSFID := mapconv.StringVal(data, "project_id"); projectSFID != "" {
		projectEntry, err := mappings.Lookup(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixProjectBySFID, projectSFID))
		if err != nil {
			return failed(ctx, "failed to look up project mapping", err, "uid", uid, "project_sfid", projectSFID)
		}
		if !projectEntry.Present() {
			slog.WarnContext(ctx, "project mapping not yet available, NAKing artifact for retry",
				"uid", uid, "project_sfid", projectSFID)
			return nak // retry with backoff
		}
		data["project_id"] = projectEntry.Value
	}

	// Resolve optional v1 committee SFID → v2 committee UID. NAK if specified but not yet synced.
	if committeeSFID := mapconv.StringVal(data, "committee_id"); committeeSFID != "" {
		committeeEntry, err := mappings.Lookup(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixCommitteeBySFID, committeeSFID))
		if err != nil {
			return failed(ctx, "failed to look up committee mapping", err, "uid", uid, "committee_sfid", committeeSFID)
		}
		if !committeeEntry.Present() {
			slog.WarnContext(ctx, "committee mapping not yet available, NAKing artifact for retry",
				"uid", uid, "committee_sfid", committeeSFID)
			return nak // retry with backoff
		}
		data["committee_id"] = committeeEntry.Value
	}

	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixArtifact, uid)

	mEntry, err := mappings.Lookup(ctx, mKey)
	if err != nil {
		return failed(ctx, "failed to look up artifact mapping", err, "uid", uid, "mapping_key", mKey)
	}
	if mEntry.Tombstoned() {
		slog.InfoContext(ctx, "artifact mapping is tombstoned, skipping update", "uid", uid)
		return ack
	}
	action := mEntry.Action()

	artifact := transformV1ToGroupsIOArtifact(uid, data)

	isPublic := false
	mailingListRef := fmt.Sprintf("groupsio_mailing_list:%s", mailingListUID)
	indexingConfig := &indexertypes.IndexingConfig{
		ObjectID:             uid,
		Public:               &isPublic,
		AccessCheckObject:    mailingListRef,
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   mailingListRef,
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
		return ack
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOArtifactSubject, built); err != nil {
		return failed(ctx, "failed to publish artifact indexer message", err, "uid", uid)
	}

	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		return failed(ctx, "failed to put mapping key", err, "mapping_key", mKey)
	}

	return ack
}

// HandleDataStreamArtifactDelete publishes a delete indexer message and tombstones the mapping.
func HandleDataStreamArtifactDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	return handleArtifactDelete(ctx, uid, publisher, mappings).requeue()
}

func handleArtifactDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) outcome {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixArtifact, uid)

	entry, err := mappings.Lookup(ctx, mKey)
	if err != nil {
		return failed(ctx, "failed to look up artifact mapping", err, "uid", uid, "mapping_key", mKey)
	}
	if entry.Tombstoned() {
		slog.InfoContext(ctx, "artifact already deleted, ACKing duplicate", "uid", uid)
		return ack
	}

	// If there is no mapping entry, this record was never indexed — nothing to delete.
	if !entry.Present() {
		slog.InfoContext(ctx, "artifact was never indexed, skipping OpenSearch delete", "uid", uid)
		if err := mappings.PutTombstone(ctx, mKey); err != nil {
			slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
		}
		return ack
	}

	// IndexingConfig is required — the indexer validates all five fields unconditionally in
	// parseIndexingConfig before any action routing. The access/history values are not used
	// for the delete (the record is being removed regardless), but must be non-empty strings.
	isPublic := false
	msg := &model.IndexerMessage{Action: model.ActionDeleted}
	built, err := msg.BuildWithIndexingConfig(ctx, uid, &indexertypes.IndexingConfig{
		ObjectID:             uid,
		Public:               &isPublic,
		AccessCheckObject:    "groupsio_mailing_list:unknown", // required by indexer, not used on delete
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   "groupsio_mailing_list:unknown", // required by indexer, not used on delete
		HistoryCheckRelation: "auditor",
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to build artifact delete indexer message", "uid", uid, "error", err)
		return ack
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOArtifactSubject, built); err != nil {
		return failed(ctx, "failed to publish artifact delete indexer message", err, "uid", uid)
	}

	if err := mappings.PutTombstone(ctx, mKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
	}
	return ack
}

// transformV1ToGroupsIOArtifact maps v1 DynamoDB fields to the GroupsIOArtifact domain model.
func transformV1ToGroupsIOArtifact(uid string, data map[string]any) *model.GroupsIOArtifact {
	artifact := &model.GroupsIOArtifact{
		ArtifactID:       uid,
		ProjectUID:       mapconv.StringVal(data, "project_id"),
		CommitteeUID:     mapconv.StringVal(data, "committee_id"),
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
