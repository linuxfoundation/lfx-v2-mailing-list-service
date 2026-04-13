// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	indexertypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
)

// HandleDataStreamSubgroupUpdate transforms the v1 payload into a GrpsIOMailingList and publishes
// indexer + access control messages. Returns true to NAK when the parent service mapping
// is absent (ordering guarantee), the project slug lookup fails (transient), or on transient errors.
func HandleDataStreamSubgroupUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter, projectLookup port.ProjectLookup) bool {
	// Resolve v1 project SFID → v2 project UID via the shared project.sfid.{sfid} mapping
	// written by lfx-v1-sync-helper. NAK if the project hasn't been processed yet.
	projectSFID := mapconv.StringVal(data, "project_id")
	if projectSFID == "" {
		slog.ErrorContext(ctx, "missing project_id in subgroup event, discarding", "uid", uid)
		return false // ACK — malformed data, retrying won't help
	}
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

	if list.ServiceUID == "" {
		slog.ErrorContext(ctx, "missing parent_id in subgroup event, discarding", "uid", uid)
		return false // ACK — malformed data, retrying won't help
	}

	// Parent dependency check: the indexer must have the parent service record before
	// the child mailing list to avoid orphaned documents in OpenSearch.
	serviceKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixService, list.ServiceUID)
	if !mappings.IsMappingPresent(ctx, serviceKey) {
		slog.WarnContext(ctx, "parent service not yet processed, NAKing subgroup for retry",
			"uid", uid, "service_uid", list.ServiceUID)
		return true // NAK — retry with backoff
	}

	// Look up project slug from the project service. NAK on transient errors so the
	// subgroup is retried once the project service is available. This is done after
	// dependency checks to avoid unnecessary RPCs when the record will NAK anyway.
	projectSlug, err := projectLookup.GetProjectSlug(ctx, projectUID)
	if err != nil {
		slog.WarnContext(ctx, "project slug lookup failed, NAKing subgroup for retry",
			"uid", uid, "project_uid", projectUID, "error", err)
		return true // NAK — retry with backoff
	}

	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroup, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "subgroup mapping is tombstoned, skipping update", "uid", uid)
		return false
	}

	action := mappings.ResolveAction(ctx, mKey)

	isPublic := list.Public
	listRef := fmt.Sprintf("groupsio_mailing_list:%s", uid)
	indexingConfig := &indexertypes.IndexingConfig{
		ObjectID:             uid,
		Public:               &isPublic,
		AccessCheckObject:    listRef,
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   listRef,
		HistoryCheckRelation: "auditor",
		ParentRefs:           list.ParentRefs(),
		NameAndAliases:       list.NameAndAliases(),
		SortName:             list.SortName(),
		Fulltext:             list.Fulltext(),
		Tags:                 list.Tags(),
	}

	msg := &model.IndexerMessage{Action: action, Tags: list.Tags()}
	built, err := msg.BuildWithIndexingConfig(ctx, list, indexingConfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build subgroup indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish subgroup indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	// Publish settings indexer message when writers or auditors are present.
	settings := buildMailingListSettings(uid, data)
	if settings != nil {
		settingsRef := fmt.Sprintf("groupsio_mailing_list:%s", uid)
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
			slog.ErrorContext(ctx, "failed to build subgroup settings indexer message", "uid", uid, "error", errSettings)
		}
		if errSettings == nil {
			if errPublish := publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSettingsSubject, builtSettings); errPublish != nil {
				slog.ErrorContext(ctx, "failed to publish subgroup settings indexer message", "uid", uid, "error", errPublish)
			}
		}
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
		UID:        uid,
		Public:     list.Public,
		References: references,
		// member relations are managed separately via member_put and must not be overwritten here
		ExcludeRelations: []string{constants.RelationMember},
	}
	if len(relations) > 0 {
		accessData.Relations = relations
	}
	accessMsg := fgatypes.GenericFGAMessage{
		ObjectType: constants.ObjectTypeGroupsIOMailingList,
		Operation:  "update_access",
		Data:       accessData,
	}
	if err := publisher.Access(ctx, constants.FGASyncUpdateAccessSubject, accessMsg); err != nil {
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

	// Store project mapping: project_uid and project_slug for the member handler.
	// Value format: "{project_uid}|{project_slug}"
	// NAK on failure — member events depend on this mapping to resolve project fields.
	projectKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupProject, uid)
	if err := mappings.PutMapping(ctx, projectKey, projectUID+"|"+projectSlug); err != nil {
		slog.ErrorContext(ctx, "failed to put project mapping key, NAKing for retry", "mapping_key", projectKey, "error", err)
		return pkgerrors.IsTransient(err)
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

	// If there is no mapping entry, this record was never indexed — nothing to delete.
	if !mappings.IsMappingPresent(ctx, mKey) {
		slog.InfoContext(ctx, "subgroup was never indexed, skipping OpenSearch delete", "uid", uid)
		if err := mappings.PutTombstone(ctx, mKey); err != nil {
			slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
		}
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

	deleteMsg := fgatypes.GenericFGAMessage{
		ObjectType: constants.ObjectTypeGroupsIOMailingList,
		Operation:  "delete_access",
		Data:       fgatypes.GenericDeleteData{UID: uid},
	}
	if err := publisher.Access(ctx, constants.FGASyncDeleteAccessSubject, deleteMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish subgroup delete access message", "uid", uid, "error", err)
	}

	if err := mappings.PutTombstone(ctx, mKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
	}
	return false
}

// buildMailingListSettings constructs a GrpsIOMailingListSettings from v1 writers/auditors.
// Returns nil when both slices are empty (no settings message needed).
func buildMailingListSettings(uid string, data map[string]any) *model.GroupsIOMailingListSettings {
	writers := toUserInfoSlice(mapconv.StringSliceVal(data, "writers"))
	auditors := toUserInfoSlice(mapconv.StringSliceVal(data, "auditors"))
	if len(writers) == 0 && len(auditors) == 0 {
		return nil
	}
	return &model.GroupsIOMailingListSettings{
		UID:      uid,
		Writers:  writers,
		Auditors: auditors,
	}
}

// toUserInfoSlice converts a slice of username strings to UserInfo values.
func toUserInfoSlice(usernames []string) []model.UserInfo {
	if len(usernames) == 0 {
		return nil
	}
	out := make([]model.UserInfo, len(usernames))
	for i, u := range usernames {
		username := u
		out[i] = model.UserInfo{Username: &username}
	}
	return out
}

// userInfoUsernames extracts the non-empty Username pointers from a []UserInfo slice.
func userInfoUsernames(users []model.UserInfo) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		if u.Username != nil && *u.Username != "" {
			out = append(out, *u.Username)
		}
	}
	return out
}

// transformV1ToGrpsIOMailingList maps v1 DynamoDB fields to the GrpsIOMailingList domain model.
func transformV1ToGrpsIOMailingList(uid string, data map[string]any) *model.GroupsIOMailingList {
	list := &model.GroupsIOMailingList{
		UID:         uid,
		GroupID:     mapconv.Int64Ptr(data, "group_id"),
		GroupName:   mapconv.StringVal(data, "group_name"),
		Public:      mapconv.StringVal(data, "visibility") == "Public",
		Type:        mapconv.StringVal(data, "type"),
		Description: mapconv.StringVal(data, "description"),
		Title:       mapconv.StringVal(data, "title"),
		SubjectTag:  mapconv.StringVal(data, "subject_tag"),
		URL:         mapconv.StringVal(data, "url"),
		Flags:       mapconv.StringSliceVal(data, "flags"),
		ServiceUID:  mapconv.StringVal(data, "parent_id"),
		ProjectUID:  mapconv.StringVal(data, "project_id"),
		Source:      "v1-sync",
	}

	if n := mapconv.Int64Ptr(data, "subscriber_count"); n != nil {
		list.SubscriberCount = int(*n)
	}

	if committeeUID := mapconv.StringVal(data, "committee"); committeeUID != "" {
		list.Committees = []model.Committee{{
			UID:                   committeeUID,
			AllowedVotingStatuses: mapconv.StringSliceVal(data, "committee_filters"),
		}}
	}

	if ts := mapconv.StringVal(data, "created_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			list.CreatedAt = t
		}
	}

	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			list.UpdatedAt = t
		}
	}

	if ts := mapconv.StringVal(data, "last_system_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			list.SystemUpdatedAt = t
		}
	}

	return list
}
