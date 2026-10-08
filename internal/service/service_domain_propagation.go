// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	indexertypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
)

// propagateServiceDomain rebuilds complete mailing-list messages from current
// source records, without writing to v1-objects or changing access/settings.
// Returns true to retry the service event if any list could not be refreshed.
func propagateServiceDomain(ctx context.Context, serviceUID, domain string, publisher port.MessagePublisher, mappings port.MappingReaderWriter, objects port.V1ObjectReader) bool {
	uids, err := mappings.ListSubgroupsByService(ctx, serviceUID)
	if err != nil {
		slog.WarnContext(ctx, "failed to list mailing lists for domain propagation, will retry", "service_uid", serviceUID, "error", err)
		return true
	}
	incomplete := false
	for _, uid := range uids {
		if err := ctx.Err(); err != nil {
			slog.WarnContext(ctx, "domain propagation context ended before all lists were refreshed", "service_uid", serviceUID, "error", err)
			return true
		}
		data, present, err := objects.GetSubgroup(ctx, uid)
		if err != nil {
			if errors.Is(err, port.ErrV1ObjectDecode) {
				slog.ErrorContext(ctx, "mailing list source is unreadable, domain propagation incomplete; will retry", "uid", uid, "service_uid", serviceUID, "error", err)
				incomplete = true
				continue
			}
			slog.WarnContext(ctx, "failed to read mailing list for domain propagation, will retry", "uid", uid, "service_uid", serviceUID, "error", err)
			return true
		}
		if !present {
			// A soft-deleted source record may still have an index entry until
			// its deletion event has finished cleaning up the lookup.
			continue
		}
		if mapconv.StringVal(data, "parent_id") != serviceUID {
			continue // concurrent move; never index it under the old service
		}
		projectSFID := mapconv.StringVal(data, "project_id")
		if projectSFID == "" {
			slog.WarnContext(ctx, "mailing list missing project_id, domain propagation incomplete; will retry", "uid", uid, "service_uid", serviceUID)
			incomplete = true
			continue
		}
		projectUID, ok := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixProjectBySFID, projectSFID))
		if !ok {
			slog.WarnContext(ctx, "project mapping unavailable during domain propagation, will retry", "uid", uid, "project_sfid", projectSFID)
			return true
		}
		data["project_id"] = projectUID
		if committeeSFID := mapconv.StringVal(data, "committee"); committeeSFID != "" {
			committeeUID, ok := mappings.GetMappingValue(ctx, fmt.Sprintf("%s.%s", constants.KVMappingPrefixCommitteeBySFID, committeeSFID))
			if !ok {
				slog.WarnContext(ctx, "committee mapping unavailable during domain propagation, will retry", "uid", uid, "committee_sfid", committeeSFID)
				return true
			}
			data["committee"] = committeeUID
		}
		list := transformV1ToGrpsIOMailingList(uid, data)
		list.Domain = domain
		if err := publishMailingListIndex(ctx, list, model.ActionUpdated, publisher); err != nil {
			slog.WarnContext(ctx, "failed to publish mailing list domain update, will retry", "uid", uid, "service_uid", serviceUID, "error", err)
			return true
		}
	}
	return incomplete
}

func publishMailingListIndex(ctx context.Context, list *model.GroupsIOMailingList, action model.MessageAction, publisher port.MessagePublisher) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	public := list.Public
	listRef := fmt.Sprintf("groupsio_mailing_list:%s", list.UID)
	config := &indexertypes.IndexingConfig{
		ObjectID:             list.UID,
		Public:               &public,
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
	built, err := msg.BuildWithIndexingConfig(ctx, list, config)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, built)
}
