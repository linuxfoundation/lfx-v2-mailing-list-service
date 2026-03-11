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

// handleServiceUpdate transforms the v1 payload into a GrpsIOService and publishes
// indexer + access control messages. Returns true to NAK on transient errors.
func handleServiceUpdate(ctx context.Context, uid string, data map[string]any, publisher port.MessagePublisher, mappingsKV jetstream.KeyValue) bool {
	svc := transformToGrpsIOService(uid, data)
	mKey := buildMappingKey(constants.KVMappingPrefixService, uid)
	action := resolveAction(ctx, mappingsKV, mKey)

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
		ObjectType: "groupsio_service",
		Public:     svc.Public,
		References: map[string][]string{"project": {svc.ProjectUID}},
	}
	if err := publisher.Access(ctx, constants.UpdateAccessGroupsIOServiceSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish service access message", "uid", uid, "error", err)
	}

	putMapping(ctx, mappingsKV, mKey, uid)
	return false
}

// handleServiceDelete publishes a delete indexer message and tombstones the mapping.
// Returns true to NAK on transient errors.
func handleServiceDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappingsKV jetstream.KeyValue) bool {
	mKey := buildMappingKey(constants.KVMappingPrefixService, uid)

	if isTombstoned(ctx, mappingsKV, mKey) {
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

	accessMsg := &model.AccessMessage{UID: uid, ObjectType: "groupsio_service"}
	if err := publisher.Access(ctx, constants.DeleteAllAccessGroupsIOServiceSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish service delete access message", "uid", uid, "error", err)
	}

	putTombstone(ctx, mappingsKV, mKey)
	return false
}

// transformToGrpsIOService maps v1 DynamoDB fields to the GrpsIOService domain model.
// Source is always "v1-sync" to distinguish these from API-created records.
func transformToGrpsIOService(uid string, data map[string]any) *model.GrpsIOService {
	svc := &model.GrpsIOService{
		UID:              uid,
		Type:             mapconv.StringVal(data, "type"),
		Domain:           mapconv.StringVal(data, "domain"),
		GroupID:          mapconv.Int64Ptr(data, "group_id"),
		Status:           mapconv.StringVal(data, "status"),
		Source:           "v1-sync",
		GlobalOwners:     mapconv.StringSliceVal(data, "global_owners"),
		Prefix:           mapconv.StringVal(data, "prefix"),
		ParentServiceUID: mapconv.StringVal(data, "parent_service_uid"),
		ProjectSlug:      mapconv.StringVal(data, "project_slug"),
		ProjectUID:       mapconv.StringVal(data, "project_uid"),
		URL:              mapconv.StringVal(data, "url"),
		GroupName:        mapconv.StringVal(data, "group_name"),
		Public:           mapconv.BoolVal(data, "public"),
	}

	if ts := mapconv.StringVal(data, "last_modified_at"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			svc.UpdatedAt = t
		}
	}

	return svc
}
