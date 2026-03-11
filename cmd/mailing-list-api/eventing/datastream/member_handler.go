// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package datastream

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
	"github.com/nats-io/nats.go/jetstream"
)

// handleMemberUpdate transforms the v1 payload into a GrpsIOMember and publishes an
// indexer message. Returns true to NAK when the parent subgroup mapping is absent
// (ordering guarantee) or on transient errors.
//
// No FGA access message is published — member access is inherited from the parent
// mailing list's access record.
func handleMemberUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappingsKV jetstream.KeyValue) bool {
	// Members carry group_id (Groups.io numeric ID) rather than a direct mailing_list_uid.
	// Resolve the parent subgroup UID via the reverse index written by the subgroup handler.
	groupID := mapconv.Int64Ptr(data, "group_id")
	if groupID == nil {
		slog.ErrorContext(ctx, "member has no group_id, cannot determine parent mailing list — ACKing", "uid", uid)
		return false
	}

	gidKey := buildMappingKey(constants.KVMappingPrefixSubgroupByGroupID, fmt.Sprintf("%d", *groupID))
	gidEntry, err := mappingsKV.Get(ctx, gidKey)
	if err != nil || gidEntry == nil || string(gidEntry.Value()) == constants.KVTombstoneMarker {
		slog.WarnContext(ctx, "parent subgroup not yet processed, NAKing member for retry",
			"uid", uid, "group_id", *groupID)
		return true // NAK — retry with backoff
	}
	mailingListUID := string(gidEntry.Value())

	member := transformToGrpsIOMember(uid, mailingListUID, data)

	mKey := buildMappingKey(constants.KVMappingPrefixMember, uid)
	action := resolveAction(ctx, mappingsKV, mKey)

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

	putMapping(ctx, mappingsKV, mKey, uid)
	return false
}

// handleMemberDelete publishes a delete indexer message and tombstones the mapping.
func handleMemberDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappingsKV jetstream.KeyValue) bool {
	mKey := buildMappingKey(constants.KVMappingPrefixMember, uid)

	if isTombstoned(ctx, mappingsKV, mKey) {
		slog.InfoContext(ctx, "member already deleted, ACKing duplicate", "uid", uid)
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

	putTombstone(ctx, mappingsKV, mKey)
	return false
}

// transformToGrpsIOMember maps v1 DynamoDB fields to the GrpsIOMember domain model.
// mailingListUID is resolved from the reverse group_id index before calling this function.
//
// DynamoDB fields available: member_id, committee_id, created_at, created_by,
// delivery_mode, delivery_mode_list, email, full_name, group_id, groups_email,
// groups_full_name, job_title, last_modified_at, last_modified_by,
// last_system_modified_at, member_type, mod_status, organization, status,
// sync_status, user_id.
func transformToGrpsIOMember(uid, mailingListUID string, data map[string]any) *model.GrpsIOMember {
	firstName, lastName := splitFullName(mapconv.StringVal(data, "full_name"))

	member := &model.GrpsIOMember{
		UID:            uid,
		MailingListUID: mailingListUID,
		MemberID:       mapconv.Int64Ptr(data, "member_id"),
		GroupID:        mapconv.Int64Ptr(data, "group_id"),
		FirstName:      firstName,
		LastName:       lastName,
		Email:          mapconv.StringVal(data, "email"),
		Organization:   mapconv.StringVal(data, "organization"),
		JobTitle:       mapconv.StringVal(data, "job_title"),
		MemberType:     mapconv.StringVal(data, "member_type"),
		DeliveryMode:   mapconv.StringVal(data, "delivery_mode"),
		ModStatus:      mapconv.StringVal(data, "mod_status"),
		Status:         mapconv.StringVal(data, "status"),
		Source:         "v1-sync",
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
