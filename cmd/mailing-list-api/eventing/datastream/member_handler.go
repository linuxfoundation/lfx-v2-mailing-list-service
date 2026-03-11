// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package datastream

import (
	"context"
	"log/slog"
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
	member := transformToGrpsIOMember(uid, data)

	// Parent dependency check: ensure the parent mailing list is already indexed.
	subgroupKey := buildMappingKey(constants.KVMappingPrefixSubgroup, member.MailingListUID)
	if !isMappingPresent(ctx, mappingsKV, subgroupKey) {
		slog.WarnContext(ctx, "parent subgroup not yet processed, NAKing member for retry",
			"uid", uid, "mailing_list_uid", member.MailingListUID)
		return true // NAK — retry with backoff
	}

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
func transformToGrpsIOMember(uid string, data map[string]any) *model.GrpsIOMember {
	member := &model.GrpsIOMember{
		UID:            uid,
		MemberID:       mapconv.Int64Ptr(data, "member_id"),
		GroupID:        mapconv.Int64Ptr(data, "group_id"),
		MailingListUID: mapconv.StringVal(data, "mailing_list_uid"),
		Username:       mapconv.StringVal(data, "username"),
		FirstName:      mapconv.StringVal(data, "first_name"),
		LastName:       mapconv.StringVal(data, "last_name"),
		Email:          mapconv.StringVal(data, "email"),
		Organization:   mapconv.StringVal(data, "organization"),
		JobTitle:       mapconv.StringVal(data, "job_title"),
		MemberType:     mapconv.StringVal(data, "member_type"),
		DeliveryMode:   mapconv.StringVal(data, "delivery_mode"),
		ModStatus:      mapconv.StringVal(data, "mod_status"),
		Status:         mapconv.StringVal(data, "status"),
		Source:         "v1-sync",
	}

	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			member.UpdatedAt = t
		}
	}

	return member
}
