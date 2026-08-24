// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	indexertypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
)

// HandleDataStreamMessageUpdate transforms a v1 message payload into a GroupsIOMessage and
// publishes an indexer message. No FGA access control is published for messages.
// Returns true to NAK when the parent subgroup mapping is absent (ordering guarantee)
// or on transient errors.
func HandleDataStreamMessageUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	groupID := mapconv.Int64Ptr(data, "group_id")
	if groupID == nil {
		slog.ErrorContext(ctx, "message has no group_id, cannot determine parent mailing list — ACKing", "uid", uid)
		return false // ACK — malformed data, retrying won't help
	}

	// Resolve group_id → mailingListUID via the reverse index written by the subgroup handler.
	gidKey := fmt.Sprintf("%s.%d", constants.KVMappingPrefixSubgroupByGroupID, *groupID)
	mailingListUID, ok := mappings.GetMappingValue(ctx, gidKey)
	if !ok {
		slog.WarnContext(ctx, "parent subgroup not yet processed, NAKing message for retry",
			"uid", uid, "group_id", *groupID)
		return true // NAK — retry with backoff
	}

	// Resolve committee UID and visibility from the subgroup-committee mapping.
	// Value format: "{committeeUID}|{isPublic}". Absent when the list has no committee.
	var committeeUID string
	isPrivate := false
	if v, hasCommittee := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupCommittee, mailingListUID)); hasCommittee {
		parts := strings.SplitN(v, "|", 2)
		committeeUID = parts[0]
		if len(parts) == 2 {
			isPrivate = parts[1] != "true"
		}
	}

	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixMessage, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "message mapping is tombstoned, skipping update", "uid", uid)
		return false
	}

	action := mappings.ResolveAction(ctx, mKey)

	msg := transformV1ToGroupsIOMessage(uid, mailingListUID, committeeUID, isPrivate, data)

	isPublicBool := false
	mailingListRef := fmt.Sprintf("groupsio_mailing_list:%s", mailingListUID)
	indexingConfig := &indexertypes.IndexingConfig{
		ObjectID:             uid,
		Public:               &isPublicBool,
		AccessCheckObject:    mailingListRef,
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   mailingListRef,
		HistoryCheckRelation: "auditor",
		ParentRefs:           msg.ParentRefs(),
		NameAndAliases:       msg.NameAndAliases(),
		SortName:             msg.SortName(),
		Fulltext:             msg.Fulltext(),
		Tags:                 msg.Tags(),
	}

	indexMsg := &model.IndexerMessage{Action: action, Tags: msg.Tags()}
	built, err := indexMsg.BuildWithIndexingConfig(ctx, msg, indexingConfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build message indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOMailingListMessageSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish message indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", mKey, "error", err)
	}

	return false
}

// HandleDataStreamMessageDelete publishes a delete indexer message and tombstones the mapping.
func HandleDataStreamMessageDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixMessage, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		slog.InfoContext(ctx, "message already deleted, ACKing duplicate", "uid", uid)
		return false
	}

	if !mappings.IsMappingPresent(ctx, mKey) {
		slog.InfoContext(ctx, "message was never indexed, skipping OpenSearch delete", "uid", uid)
		if err := mappings.PutTombstone(ctx, mKey); err != nil {
			slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
		}
		return false
	}

	isPublicBool := false
	deleteMsg := &model.IndexerMessage{Action: model.ActionDeleted}
	built, err := deleteMsg.BuildWithIndexingConfig(ctx, uid, &indexertypes.IndexingConfig{
		ObjectID:             uid,
		Public:               &isPublicBool,
		AccessCheckObject:    "groupsio_mailing_list:unknown", // required by indexer, not used on delete
		AccessCheckRelation:  "viewer",
		HistoryCheckObject:   "groupsio_mailing_list:unknown", // required by indexer, not used on delete
		HistoryCheckRelation: "auditor",
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to build message delete indexer message", "uid", uid, "error", err)
		return false
	}

	if err := publisher.Indexer(ctx, constants.IndexGroupsIOMailingListMessageSubject, built); err != nil {
		slog.ErrorContext(ctx, "failed to publish message delete indexer message", "uid", uid, "error", err)
		return pkgerrors.IsTransient(err)
	}

	if err := mappings.PutTombstone(ctx, mKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone", "mapping_key", mKey, "error", err)
	}
	return false
}

// transformV1ToGroupsIOMessage maps v1 DynamoDB fields to the GroupsIOMessage domain model.
func transformV1ToGroupsIOMessage(uid, mailingListUID, committeeUID string, isPrivate bool, data map[string]any) *model.GroupsIOMessage {
	msg := &model.GroupsIOMessage{
		MessageID:      uid,
		MailingListUID: mailingListUID,
		CommitteeUID:   committeeUID,
		IsPrivate:      isPrivate,
		Subject:        mapconv.StringVal(data, "subject"),
		Snippet:        mapconv.StringVal(data, "snippet"),
		SenderName:     mapconv.StringVal(data, "sender_name"),
		GroupDomain:    mapconv.StringVal(data, "group_domain"),
		GroupName:      mapconv.StringVal(data, "group_name"),
	}

	if gid := mapconv.Int64Ptr(data, "group_id"); gid != nil {
		msg.GroupID = uint64(*gid)
	}
	if tid := mapconv.Int64Ptr(data, "topic_id"); tid != nil {
		msg.TopicID = uint64(*tid)
	}
	if mn := mapconv.Int64Ptr(data, "msg_num"); mn != nil {
		msg.MsgNum = uint64(*mn)
	}
	if isReply := mapconv.StringVal(data, "is_reply"); isReply == "true" {
		msg.IsReply = true
	}
	if v, ok := data["is_reply"].(bool); ok {
		msg.IsReply = v
	}

	if ts := mapconv.StringVal(data, "created_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			msg.CreatedAt = t
		}
	}

	return msg
}
