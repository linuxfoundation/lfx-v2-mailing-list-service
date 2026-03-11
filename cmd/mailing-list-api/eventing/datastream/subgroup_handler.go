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

// handleSubgroupUpdate transforms the v1 payload into a GrpsIOMailingList and publishes
// indexer + access control messages. Returns true to NAK when the parent service mapping
// is absent (ordering guarantee) or on transient errors.
func handleSubgroupUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappingsKV jetstream.KeyValue) bool {
	list := transformToGrpsIOMailingList(uid, data)

	// Parent dependency check: the indexer must have the parent service record before
	// the child mailing list to avoid orphaned documents in OpenSearch.
	serviceKey := buildMappingKey(constants.KVMappingPrefixService, list.ServiceUID)
	if !isMappingPresent(ctx, mappingsKV, serviceKey) {
		slog.WarnContext(ctx, "parent service not yet processed, NAKing subgroup for retry",
			"uid", uid, "service_uid", list.ServiceUID)
		return true // NAK — retry with backoff
	}

	mKey := buildMappingKey(constants.KVMappingPrefixSubgroup, uid)
	action := resolveAction(ctx, mappingsKV, mKey)

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

	accessMsg := &model.AccessMessage{
		UID:        uid,
		ObjectType: "groupsio_mailing_list",
		Public:     list.Public,
		References: map[string][]string{
			"project":          {list.ProjectUID},
			"groupsio_service": {list.ServiceUID},
		},
	}
	if err := publisher.Access(ctx, constants.UpdateAccessGroupsIOMailingListSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish subgroup access message", "uid", uid, "error", err)
	}

	putMapping(ctx, mappingsKV, mKey, uid)
	return false
}

// handleSubgroupDelete publishes a delete indexer message and tombstones the mapping.
func handleSubgroupDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappingsKV jetstream.KeyValue) bool {
	mKey := buildMappingKey(constants.KVMappingPrefixSubgroup, uid)

	if isTombstoned(ctx, mappingsKV, mKey) {
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

	accessMsg := &model.AccessMessage{UID: uid, ObjectType: "groupsio_mailing_list"}
	if err := publisher.Access(ctx, constants.DeleteAllAccessGroupsIOMailingListSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish subgroup delete access message", "uid", uid, "error", err)
	}

	putTombstone(ctx, mappingsKV, mKey)
	return false
}

// transformToGrpsIOMailingList maps v1 DynamoDB fields to the GrpsIOMailingList domain model.
func transformToGrpsIOMailingList(uid string, data map[string]any) *model.GrpsIOMailingList {
	list := &model.GrpsIOMailingList{
		UID:             uid,
		GroupID:         mapconv.Int64Ptr(data, "group_id"),
		GroupName:       mapconv.StringVal(data, "group_name"),
		Public:          mapconv.BoolVal(data, "public"),
		AudienceAccess:  mapconv.StringVal(data, "audience_access"),
		Type:            mapconv.StringVal(data, "type"),
		Description:     mapconv.StringVal(data, "description"),
		Title:           mapconv.StringVal(data, "title"),
		SubjectTag:      mapconv.StringVal(data, "subject_tag"),
		ServiceUID:      mapconv.StringVal(data, "service_uid"),
		ProjectUID:      mapconv.StringVal(data, "project_uid"),
		SubscriberCount: mapconv.IntVal(data, "subscriber_count"),
		Source:          "v1-sync",
	}

	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			list.UpdatedAt = t
		}
	}

	return list
}
