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

// HandleDataStreamSubgroupUpdate transforms the v1 payload into a GrpsIOMailingList and publishes
// indexer + access control messages. Returns true to NAK when the parent service mapping
// is absent (ordering guarantee) or on transient errors.
func HandleDataStreamSubgroupUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	// Resolve v1 project SFID → v2 project UID via the shared project.sfid.{sfid} mapping
	// written by lfx-v1-sync-helper. NAK if the project hasn't been processed yet.
	projectSFID := mapconv.StringVal(data, "project_id")
	projectUID, ok := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixProjectBySFID, projectSFID))
	if !ok {
		slog.WarnContext(ctx, "project mapping not yet available, NAKing subgroup for retry",
			"uid", uid, "project_sfid", projectSFID)
		return true // NAK — retry with backoff
	}
	data["project_id"] = projectUID

	// Resolve optional v1 committee SFID → v2 committee UID. NAK if the committee
	// has been specified but hasn't been synced yet (ordering guarantee).
	if committeeSFID := mapconv.StringVal(data, "committee"); committeeSFID != "" {
		committeeUID, ok := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixCommitteeBySFID, committeeSFID))
		if !ok {
			slog.WarnContext(ctx, "committee mapping not yet available, NAKing subgroup for retry",
				"uid", uid, "committee_sfid", committeeSFID)
			return true // NAK — retry with backoff
		}
		data["committee"] = committeeUID
	}

	list := transformV1ToGrpsIOMailingList(uid, data)

	// Parent dependency check: the indexer must have the parent service record before
	// the child mailing list to avoid orphaned documents in OpenSearch.
	serviceKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixService, list.ServiceUID)
	if !mappings.IsMappingPresent(ctx, serviceKey) {
		slog.WarnContext(ctx, "parent service not yet processed, NAKing subgroup for retry",
			"uid", uid, "service_uid", list.ServiceUID)
		return true // NAK — retry with backoff
	}

	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroup, uid)
	action := mappings.ResolveAction(ctx, mKey)

	msg := &model.IndexerMessage{Action: action, Tags: list.Tags()}
	built, err := msg.Build(ctx, list)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build subgroup indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish subgroup indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	references := map[string][]string{
		// Project access is inherited through the service — only service reference needed.
		constants.RelationGroupsIOService: {list.ServiceUID},
	}
	for _, committee := range list.Committees {
		if committee.UID != "" {
			references[constants.RelationCommittee] = append(references[constants.RelationCommittee], committee.UID)
		}
	}
	accessMsg := &model.AccessMessage{
		UID:        uid,
		ObjectType: constants.ObjectTypeGroupsIOMailingList,
		Public:     list.Public,
		References: references,
	}
	if err := publisher.Access(ctx, constants.UpdateAccessGroupsIOMailingListSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish subgroup access message", "uid", uid, "error", err)
	}

	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", mKey, "error", err)
	}

	// Store reverse index: group_id → subgroup UID so member events can resolve MailingListUID.
	if list.GroupID != nil {
		gidKey := fmt.Sprintf("%s.%d", constants.KVMappingPrefixSubgroupByGroupID, *list.GroupID)
		if err := mappings.PutMapping(ctx, gidKey, uid); err != nil {
			slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", gidKey, "error", err)
		}
	}

	return false
}

// HandleDataStreamSubgroupDelete publishes a delete indexer message and tombstones the mapping.
func HandleDataStreamSubgroupDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroup, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "subgroup already deleted, ACKing duplicate", "uid", uid)
		return false
	}

	msg := &model.IndexerMessage{Action: model.ActionDeleted}
	built, err := msg.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build subgroup delete indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish subgroup delete indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	accessMsg := &model.AccessMessage{UID: uid, ObjectType: constants.ObjectTypeGroupsIOMailingList}
	if err := publisher.Access(ctx, constants.DeleteAllAccessGroupsIOMailingListSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish subgroup delete access message", "uid", uid, "error", err)
	}

	if err := mappings.PutTombstone(ctx, mKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
	}
	return false
}

// transformV1ToGrpsIOMailingList maps v1 DynamoDB fields to the GrpsIOMailingList domain model.
func transformV1ToGrpsIOMailingList(uid string, data map[string]any) *model.GrpsIOMailingList {
	list := &model.GrpsIOMailingList{
		UID:         uid,
		GroupID:     mapconv.Int64Ptr(data, "group_id"),
		GroupName:   mapconv.StringVal(data, "group_name"),
		Public:      mapconv.StringVal(data, "visibility") == "Public",
		Type:        mapconv.StringVal(data, "type"),
		Description: mapconv.StringVal(data, "description"),
		SubjectTag:  mapconv.StringVal(data, "subject_tag"),
		ServiceUID:  mapconv.StringVal(data, "parent_id"),
		ProjectUID:  mapconv.StringVal(data, "project_id"),
		Source:      "v1-sync",
	}

	if committeeUID := mapconv.StringVal(data, "committee"); committeeUID != "" {
		list.Committees = []model.Committee{{
			UID:                   committeeUID,
			AllowedVotingStatuses: mapconv.StringSliceVal(data, "committee_filters"),
		}}
	}

	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			list.UpdatedAt = t
		}
	}

	return list
}
