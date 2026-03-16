// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
)

// HandleDataStreamMemberUpdate transforms the v1 payload into a GrpsIOMember and publishes an
// indexer message. Returns true to NAK when the parent subgroup mapping is absent
// (ordering guarantee) or on transient errors.
//
// No FGA access message is published — member access is inherited from the parent
// mailing list's access record.
func HandleDataStreamMemberUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	// Members carry group_id (Groups.io numeric ID) rather than a direct mailing_list_uid.
	// Resolve the parent subgroup UID via the reverse index written by the subgroup handler.
	groupID := mapconv.Int64Ptr(data, "group_id")
	if groupID == nil {
		slog.ErrorContext(ctx, "member has no group_id, cannot determine parent mailing list — ACKing", "uid", uid)
		return false
	}

	gidKey := fmt.Sprintf("%s.%d", constants.KVMappingPrefixSubgroupByGroupID, *groupID)
	mailingListUID, ok := mappings.GetMappingValue(ctx, gidKey)
	if !ok {
		slog.WarnContext(ctx, "parent subgroup not yet processed, NAKing member for retry",
			"uid", uid, "group_id", *groupID)
		return true // NAK — retry with backoff
	}

	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixMember, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "member mapping is tombstoned, skipping update", "uid", uid)
		return false
	}

	action := mappings.ResolveAction(ctx, mKey)

	member := transformV1ToGrpsIOMember(uid, mailingListUID, data)

	msg := &model.IndexerMessage{Action: action, Tags: member.Tags()}
	built, err := msg.Build(ctx, member)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build member indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOMemberSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish member indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", mKey, "error", err)
	}
	return false
}

// HandleDataStreamMemberDelete publishes a delete indexer message and tombstones the mapping.
func HandleDataStreamMemberDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixMember, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "member already deleted, ACKing duplicate", "uid", uid)
		return false
	}

	// If there is no mapping entry, this record was never indexed — nothing to delete.
	if !mappings.IsMappingPresent(ctx, mKey) {
		slog.InfoContext(ctx, "member was never indexed, skipping OpenSearch delete", "uid", uid)
		if err := mappings.PutTombstone(ctx, mKey); err != nil {
			slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
		}
		return false
	}

	msg := &model.IndexerMessage{Action: model.ActionDeleted}
	built, err := msg.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build member delete indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOMemberSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish member delete indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	if err := mappings.PutTombstone(ctx, mKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
	}
	return false
}

// transformV1ToGrpsIOMember maps v1 DynamoDB fields to the GrpsIOMember domain model.
// mailingListUID is resolved from the reverse group_id index before calling this function.
func transformV1ToGrpsIOMember(uid, mailingListUID string, data map[string]any) *model.GrpsIOMember {
	firstName, lastName := splitFullName(mapconv.StringVal(data, "full_name"))

	member := &model.GrpsIOMember{
		UID:               uid,
		MailingListUID:    mailingListUID,
		MemberID:          mapconv.Int64Ptr(data, "member_id"),
		GroupID:           mapconv.Int64Ptr(data, "group_id"),
		UserID:            mapconv.StringVal(data, "user_id"),
		FirstName:         firstName,
		LastName:          lastName,
		Email:             mapconv.StringVal(data, "email"),
		Organization:      mapconv.StringVal(data, "organization"),
		JobTitle:          mapconv.StringVal(data, "job_title"),
		GroupsEmail:       mapconv.StringVal(data, "groups_email"),
		GroupsFullName:    mapconv.StringVal(data, "groups_full_name"),
		CommitteeEmail:    mapconv.StringVal(data, "committee_email"),
		CommitteeFullName: mapconv.StringVal(data, "committee_full_name"),
		CommitteeID:       mapconv.StringVal(data, "committee_id"),
		Role:              mapconv.StringVal(data, "role"),
		VotingStatus:      mapconv.StringVal(data, "voting_status"),
		MemberType:        mapconv.StringVal(data, "member_type"),
		DeliveryMode:      mapconv.StringVal(data, "delivery_mode"),
		DeliveryModeList:  mapconv.StringVal(data, "delivery_mode_list"),
		ModStatus:         mapconv.StringVal(data, "mod_status"),
		Status:            mapconv.StringVal(data, "status"),
		Source:            "v1-sync",
	}

	if ts := mapconv.StringVal(data, "created_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			member.CreatedAt = t
		}
	}
	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			member.UpdatedAt = t
		}
	}
	if ts := mapconv.StringVal(data, "last_system_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			member.SystemUpdatedAt = t
		}
	}

	return member
}

// splitFullName splits "First Last" into (first, last).
// For single-token names (no space), the whole string is returned as first name.
func splitFullName(fullName string) (string, string) {
	idx := strings.Index(fullName, " ")
	if idx == -1 {
		return fullName, ""
	}
	return fullName[:idx], fullName[idx+1:]
}
