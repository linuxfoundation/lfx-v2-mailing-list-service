// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
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
		slog.WarnContext(ctx, "missing project_id in subgroup event, discarding", "uid", uid)
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

	// The parent service is access-restricted for some callers. Copy its domain onto
	// the mailing-list resource when available, without blocking indexing while the
	// parent service is being fully configured.
	domainKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixServiceDomain, list.ServiceUID)
	serviceDomain, domainMappingPresent, err := mappings.GetMappingValueWithError(ctx, domainKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read parent service domain mapping, NAKing subgroup for retry",
			"uid", uid, "service_uid", list.ServiceUID, "error", err)
		return true
	}
	if domainMappingPresent && serviceDomain != "" {
		list.Domain = serviceDomain
	} else {
		slog.InfoContext(ctx, "parent service domain absent or empty, indexing subgroup without domain",
			"uid", uid, "service_uid", list.ServiceUID)
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

	unassociatedKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociated, uid)
	completedUID, wasUnassociated, err := mappings.GetMappingValueWithError(ctx, unassociatedKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read subgroup unassociation marker, will retry", "uid", uid, "mapping_key", unassociatedKey, "error", err)
		return true
	}
	if wasUnassociated && completedUID != uid {
		slog.ErrorContext(ctx, "unexpected subgroup unassociation marker", "uid", uid, "mapping_value", completedUID)
		return true
	}
	unassociationPendingKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociationPending, uid)
	pendingUID, wasPending, err := mappings.GetMappingValueWithError(ctx, unassociationPendingKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read subgroup unassociation progress, will retry", "uid", uid, "mapping_key", unassociationPendingKey, "error", err)
		return true
	}
	if wasPending && pendingUID != uid {
		slog.ErrorContext(ctx, "unexpected subgroup unassociation progress marker", "uid", uid, "mapping_value", pendingUID)
		return true
	}
	action := mappings.ResolveAction(ctx, mKey)
	// Persist a recovery pointer before publishing. If the parent/index writes
	// later fail and the source loses parent_id, unassociation can still find
	// and remove the document and inherited service access.
	publishedParentKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupPublishedParent, uid)
	if err := mappings.PutMapping(ctx, publishedParentKey, list.ServiceUID); err != nil {
		return retrySubgroupIndexError(ctx, uid, publishedParentKey, "record subgroup publication parent", err)
	}

	if err := publishMailingListIndex(ctx, list, action, publisher); err != nil {
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
	if err := publisher.Access(ctx, fgaconstants.GenericUpdateAccessSubject, accessMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish subgroup access message, will retry", "uid", uid, "error", err)
		return true
	}

	if err := mappings.PutMapping(ctx, mKey, uid); err != nil {
		if pkgerrors.IsTransient(err) {
			slog.WarnContext(ctx, "failed to put mapping key, will retry", "mapping_key", mKey, "error", err)
			return true
		}
		slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", mKey, "error", err)
		return false
	}

	// Store reverse index: group_id → subgroup UID so member events can resolve MailingListUID.
	if list.GroupID != nil {
		gidKey := fmt.Sprintf("%s.%d", constants.KVMappingPrefixSubgroupByGroupID, *list.GroupID)
		if err := mappings.PutMapping(ctx, gidKey, uid); err != nil {
			if pkgerrors.IsTransient(err) {
				slog.WarnContext(ctx, "failed to put mapping key, will retry", "mapping_key", gidKey, "error", err)
				return true
			}
			slog.ErrorContext(ctx, "failed to put mapping key", "mapping_key", gidKey, "error", err)
			return false
		}
	}

	// Store project mapping: project_uid and project_slug for the member handler.
	// Value format: "{project_uid}|{project_slug}"
	// NAK on failure — member events depend on this mapping to resolve project fields.
	projectKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupProject, uid)
	if err := mappings.PutMapping(ctx, projectKey, projectUID+"|"+projectSlug); err != nil {
		if pkgerrors.IsTransient(err) {
			slog.WarnContext(ctx, "failed to put project mapping key, will retry", "mapping_key", projectKey, "error", err)
			return true
		}
		slog.ErrorContext(ctx, "failed to put project mapping key", "mapping_key", projectKey, "error", err)
		return false
	}

	// Store committee mapping for message handler: committeeUID and visibility.
	// Value format: "{committeeUID}|{isPublic}" where isPublic is "true" or "false".
	// Written on every update (including when no committee) so message handlers can distinguish
	// "subgroup processed, no committee" (mapping present, empty committeeUID) from
	// "subgroup not yet processed with current code" (mapping absent → NAK).
	// This also overwrites stale committee data when a committee association is removed.
	committeeUID := ""
	if len(list.Committees) > 0 && list.Committees[0].UID != "" {
		committeeUID = list.Committees[0].UID
	}
	isPublicStr := "false"
	if list.Public {
		isPublicStr = "true"
	}
	committeeKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupCommittee, uid)
	if err := mappings.PutMapping(ctx, committeeKey, committeeUID+"|"+isPublicStr); err != nil {
		if pkgerrors.IsTransient(err) {
			slog.WarnContext(ctx, "failed to put committee mapping key, will retry", "mapping_key", committeeKey, "error", err)
			return true
		}
		slog.ErrorContext(ctx, "failed to put committee mapping key", "mapping_key", committeeKey, "error", err)
		return false
	}

	// Index each subgroup under its parent service after the mappings needed by
	// member and message events. Concurrent subgroups write separate index keys.
	if !constants.ValidKVKeySegment(uid) || !constants.ValidKVKeySegment(list.ServiceUID) {
		slog.ErrorContext(ctx, "invalid subgroup or service UID for service index", "uid", uid, "service_uid", list.ServiceUID)
		return false
	}
	parentKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupParent, uid)
	previousService, _, err := mappings.GetMappingValueWithError(ctx, parentKey)
	if err != nil {
		return retrySubgroupIndexError(ctx, uid, parentKey, "read subgroup parent mapping", err)
	}
	pendingKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupPreviousService, uid)
	pendingService, _, err := mappings.GetMappingValueWithError(ctx, pendingKey)
	if err != nil {
		return retrySubgroupIndexError(ctx, uid, pendingKey, "read pending subgroup move", err)
	}
	serviceIndexKey := fmt.Sprintf("%s.%s.%s", constants.KVMappingPrefixSubgroupByService, list.ServiceUID, uid)
	if err := mappings.PutMapping(ctx, serviceIndexKey, uid); err != nil {
		return retrySubgroupIndexError(ctx, uid, serviceIndexKey, "index subgroup by service", err)
	}
	if previousService != "" && previousService != list.ServiceUID {
		if !constants.ValidKVKeySegment(previousService) {
			slog.ErrorContext(ctx, "invalid previous service UID for index cleanup", "uid", uid, "service_uid", previousService)
			return false
		}
		// Persist the old service before switching the parent, so a retry can
		// finish cleanup even after the parent has already changed.
		if err := mappings.PutMapping(ctx, pendingKey, previousService); err != nil {
			return retrySubgroupIndexError(ctx, uid, pendingKey, "record pending subgroup move", err)
		}
		pendingService = previousService
	}
	if err := mappings.PutMapping(ctx, parentKey, list.ServiceUID); err != nil {
		return retrySubgroupIndexError(ctx, uid, parentKey, "store subgroup parent service", err)
	}
	if pendingService != "" {
		if !constants.ValidKVKeySegment(pendingService) {
			slog.ErrorContext(ctx, "invalid pending service UID for index cleanup", "uid", uid, "service_uid", pendingService)
			return false
		}
		if pendingService != list.ServiceUID {
			oldIndexKey := fmt.Sprintf("%s.%s.%s", constants.KVMappingPrefixSubgroupByService, pendingService, uid)
			if err := mappings.PutTombstone(ctx, oldIndexKey); err != nil {
				return retrySubgroupIndexError(ctx, uid, oldIndexKey, "remove previous service subgroup index", err)
			}
		}
		if err := mappings.PutTombstone(ctx, pendingKey); err != nil {
			return retrySubgroupIndexError(ctx, uid, pendingKey, "clear pending subgroup move", err)
		}
	}
	// Clear the old completion marker only after index, access, and parent
	// mappings are reconciled. Otherwise backfill might restore an index for
	// a still-inaccessible mailing list without replaying its access event.
	if err := mappings.PutTombstone(ctx, unassociatedKey); err != nil {
		slog.WarnContext(ctx, "failed to invalidate subgroup unassociation marker, will retry", "uid", uid, "mapping_key", unassociatedKey, "error", err)
		return true
	}
	if wasPending {
		if err := mappings.PutTombstone(ctx, unassociationPendingKey); err != nil {
			slog.WarnContext(ctx, "failed to clear subgroup unassociation progress, will retry", "uid", uid, "mapping_key", unassociationPendingKey, "error", err)
			return true
		}
	}

	return false
}

func retrySubgroupIndexError(ctx context.Context, uid, key, operation string, err error) bool {
	if pkgerrors.IsTransient(err) {
		slog.WarnContext(ctx, "subgroup service index failure, will retry", "uid", uid, "mapping_key", key, "operation", operation, "error", err)
		return true
	}
	slog.ErrorContext(ctx, "permanent subgroup service index failure", "uid", uid, "mapping_key", key, "operation", operation, "error", err)
	return false
}

// HandleDataStreamSubgroupUnassociated removes a list from its old service
// without tombstoning its subgroup mapping, allowing future reparenting.
func HandleDataStreamSubgroupUnassociated(ctx context.Context, uid, previous string, source map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	publishedParentKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupPublishedParent, uid)
	publishedParent, published, err := mappings.GetMappingValueWithError(ctx, publishedParentKey)
	if err != nil {
		return true
	}
	pendingKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupPreviousService, uid)
	pendingParent, pending, err := mappings.GetMappingValueWithError(ctx, pendingKey)
	if err != nil {
		return true
	}
	if !constants.ValidKVKeySegment(uid) || previous != "" && !constants.ValidKVKeySegment(previous) ||
		published && !constants.ValidKVKeySegment(publishedParent) || pending && !constants.ValidKVKeySegment(pendingParent) {
		slog.ErrorContext(ctx, "invalid subgroup or parent UID for unassociation cleanup", "uid", uid, "service_uid", previous, "published_service_uid", publishedParent, "pending_service_uid", pendingParent)
		return true
	}
	// A partial move can publish under a new parent while the stored parent
	// still names the old service. Keep both index keys until deletion and
	// access revocation succeed, so redelivery can finish either cleanup.
	parents := []string{}
	if previous != "" {
		parents = append(parents, previous)
	}
	if published && publishedParent != "" && publishedParent != previous {
		parents = append(parents, publishedParent)
	}
	if pending && pendingParent != "" && pendingParent != previous && pendingParent != publishedParent {
		parents = append(parents, pendingParent)
	}
	indexKeys := make([]string, 0, len(parents))
	indexed := false
	for _, parent := range parents {
		indexKey := fmt.Sprintf("%s.%s.%s", constants.KVMappingPrefixSubgroupByService, parent, uid)
		_, present, readErr := mappings.GetMappingValueWithError(ctx, indexKey)
		if readErr != nil {
			slog.WarnContext(ctx, "failed to read subgroup service index", "uid", uid, "mapping_key", indexKey, "error", readErr)
			return true
		}
		if present {
			indexed = true
			indexKeys = append(indexKeys, indexKey)
		}
	}
	// Older indexed lists may predate all service-index pointers. A live
	// forward mapping still means their document may need UID-based deletion.
	forwardKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroup, uid)
	forwardUID, forwardPresent, err := mappings.GetMappingValueWithError(ctx, forwardKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read subgroup mapping for unassociation, will retry", "uid", uid, "error", err)
		return true
	}
	if forwardPresent && forwardUID != uid {
		slog.ErrorContext(ctx, "unexpected subgroup mapping for unassociation", "uid", uid, "mapping_value", forwardUID)
		return true
	}
	// Persist progress before either destructive publication. Completion is
	// recorded later, so a failure between access revocation and that final
	// write cannot be mistaken for a never-unassociated legacy list.
	unassociationPendingKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociationPending, uid)
	progressUID, progressPresent, err := mappings.GetMappingValueWithError(ctx, unassociationPendingKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read subgroup unassociation progress, will retry", "uid", uid, "error", err)
		return true
	}
	if progressPresent && progressUID != uid {
		slog.ErrorContext(ctx, "unexpected subgroup unassociation progress marker", "uid", uid, "mapping_value", progressUID)
		return true
	}
	if err := mappings.PutMapping(ctx, unassociationPendingKey, uid); err != nil {
		slog.WarnContext(ctx, "failed to record subgroup unassociation progress, will retry", "uid", uid, "error", err)
		return true
	}
	if indexed || published || forwardPresent {
		if err := ctx.Err(); err != nil {
			return true
		}
		isPublic := false
		msg := &model.IndexerMessage{Action: model.ActionDeleted}
		built, buildErr := msg.BuildWithIndexingConfig(ctx, uid, &indexertypes.IndexingConfig{
			ObjectID:             uid,
			Public:               &isPublic,
			AccessCheckObject:    "groupsio_mailing_list:" + uid,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   "groupsio_mailing_list:" + uid,
			HistoryCheckRelation: "auditor",
		})
		if buildErr != nil {
			slog.ErrorContext(ctx, "failed to build subgroup unassociation delete message, will retry", "uid", uid, "service_uid", previous, "error", buildErr)
			return true
		}
		if err := publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, built); err != nil {
			slog.ErrorContext(ctx, "failed to publish subgroup unassociation delete message", "uid", uid, "error", err)
			return pkgerrors.IsTransient(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return true
	}
	if HandleDataStreamSubgroupUnassociatedAccess(ctx, uid, source, publisher, mappings) {
		return true
	}
	for _, indexKey := range indexKeys {
		if err := mappings.PurgeMapping(ctx, indexKey); err != nil {
			slog.WarnContext(ctx, "failed to purge subgroup service index", "uid", uid, "mapping_key", indexKey, "error", err)
			return true
		}
	}
	if pending {
		if err := mappings.PutTombstone(ctx, pendingKey); err != nil {
			slog.WarnContext(ctx, "failed to clear pending subgroup move", "uid", uid, "mapping_key", pendingKey, "error", err)
			return true
		}
	}
	if published {
		if err := mappings.PutTombstone(ctx, publishedParentKey); err != nil {
			slog.WarnContext(ctx, "failed to clear published subgroup parent", "uid", uid, "mapping_key", publishedParentKey, "error", err)
			return true
		}
	}
	parentKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupParent, uid)
	if err := mappings.PutTombstone(ctx, parentKey); err != nil {
		slog.WarnContext(ctx, "failed to clear subgroup parent mapping", "uid", uid, "mapping_key", parentKey, "error", err)
		return true
	}
	// Write completion last: a failed delete, access sync, or index cleanup
	// must never be mistaken for an already-unassociated subgroup.
	unassociatedKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociated, uid)
	if err := mappings.PutMapping(ctx, unassociatedKey, uid); err != nil {
		slog.WarnContext(ctx, "failed to record subgroup unassociation completion, will retry", "uid", uid, "mapping_key", unassociatedKey, "error", err)
		return true
	}
	if err := mappings.PutTombstone(ctx, unassociationPendingKey); err != nil {
		slog.WarnContext(ctx, "failed to clear subgroup unassociation progress, will retry", "uid", uid, "mapping_key", unassociationPendingKey, "error", err)
		return true
	}
	return false
}

// HandleDataStreamSubgroupUnassociatedAccess reconciles public viewer access
// and the message handler's cached visibility from the latest source record.
// Separately managed member, writer, auditor, and committee relations are kept.
func HandleDataStreamSubgroupUnassociatedAccess(ctx context.Context, uid string, source map[string]any, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	if ctx.Err() != nil {
		return true
	}
	public := strings.EqualFold(mapconv.StringVal(source, "visibility"), "public")
	committeeKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupCommittee, uid)
	committeeMapping, present, err := mappings.GetMappingValueWithError(ctx, committeeKey)
	if err != nil {
		slog.WarnContext(ctx, "failed to read subgroup committee visibility, will retry", "uid", uid, "mapping_key", committeeKey, "error", err)
		return true
	}
	committeeUID := ""
	if present {
		committeeUID = strings.SplitN(committeeMapping, "|", 2)[0]
	} else if committeeSFID := mapconv.StringVal(source, "committee"); committeeSFID != "" {
		mappingKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixCommitteeBySFID, committeeSFID)
		committeeUID, present, err = mappings.GetMappingValueWithError(ctx, mappingKey)
		if err != nil || !present {
			slog.WarnContext(ctx, "committee mapping unavailable for parentless mailing list, will retry", "uid", uid, "mapping_key", mappingKey, "error", err)
			return true
		}
	}
	access := fgatypes.GenericFGAMessage{
		ObjectType: constants.ObjectTypeGroupsIOMailingList,
		Operation:  "update_access",
		Data: fgatypes.GenericAccessData{
			UID:    uid,
			Public: public,
			References: map[string][]string{
				constants.RelationGroupsIOService: {},
			},
			ExcludeRelations: []string{constants.RelationMember, constants.RelationWriter, constants.RelationAuditor, constants.RelationCommittee},
		},
	}
	if err := publisher.Access(ctx, fgaconstants.GenericUpdateAccessSubject, access); err != nil {
		slog.WarnContext(ctx, "failed to reconcile subgroup public and service access, will retry", "uid", uid, "error", err)
		return true
	}
	if ctx.Err() != nil {
		return true
	}
	if err := mappings.PutMapping(ctx, committeeKey, committeeUID+"|"+strconv.FormatBool(public)); err != nil {
		slog.WarnContext(ctx, "failed to update subgroup committee visibility, will retry", "uid", uid, "mapping_key", committeeKey, "error", err)
		return true
	}
	return ctx.Err() != nil
}

// HandleDataStreamSubgroupDelete publishes a delete indexer message and tombstones the mapping.
func HandleDataStreamSubgroupDelete(ctx context.Context, uid string, publisher port.MessagePublisher, mappings port.MappingReaderWriter) bool {
	mKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroup, uid)
	parentKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupParent, uid)

	if mappings.IsTombstoned(ctx, mKey) {
		if removeSubgroupServiceIndex(ctx, uid, mKey, parentKey, mappings) {
			return true
		}
		slog.InfoContext(ctx, "subgroup already deleted, ACKing duplicate", "uid", uid)
		return false
	}

	// Publication can succeed before the forward mapping is stored. In that
	// case the recovery pointer still requires a document and access delete.
	publicationKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupPublishedParent, uid)
	_, published, err := mappings.GetMappingValueWithError(ctx, publicationKey)
	if err != nil {
		return true
	}
	forwardUID, forwardPresent, err := mappings.GetMappingValueWithError(ctx, mKey)
	if err != nil || forwardPresent && forwardUID != uid {
		return true
	}
	completionKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociated, uid)
	completedUID, complete, err := mappings.GetMappingValueWithError(ctx, completionKey)
	if err != nil || complete && completedUID != uid {
		return true
	}
	progressKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociationPending, uid)
	progressUID, pendingCleanup, err := mappings.GetMappingValueWithError(ctx, progressKey)
	if err != nil || pendingCleanup && progressUID != uid {
		return true
	}
	if !forwardPresent && !published && !complete && !pendingCleanup {
		slog.InfoContext(ctx, "subgroup was never indexed, skipping OpenSearch delete", "uid", uid)
		return removeSubgroupServiceIndex(ctx, uid, mKey, parentKey, mappings)
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
	if err := publisher.Access(ctx, fgaconstants.GenericDeleteAccessSubject, deleteMsg); err != nil {
		slog.WarnContext(ctx, "failed to publish subgroup delete access message, will retry", "uid", uid, "error", err)
		return true
	}

	return removeSubgroupServiceIndex(ctx, uid, mKey, parentKey, mappings)
}

// removeSubgroupServiceIndex clears all known service index entries before the
// parent pointers, so redelivery can finish cleanup after an intermediate failure.
func removeSubgroupServiceIndex(ctx context.Context, uid, subgroupKey, parentKey string, mappings port.MappingReaderWriter) bool {
	publishedParentKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupPublishedParent, uid)
	parentUID, present, err := mappings.GetMappingValueWithError(ctx, parentKey)
	if err != nil {
		return retrySubgroupIndexError(ctx, uid, parentKey, "read subgroup parent for deletion", err)
	}
	publishedService, published, err := mappings.GetMappingValueWithError(ctx, publishedParentKey)
	if err != nil {
		return retrySubgroupIndexError(ctx, uid, publishedParentKey, "read published subgroup parent for deletion", err)
	}
	pendingKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupPreviousService, uid)
	pendingService, pending, err := mappings.GetMappingValueWithError(ctx, pendingKey)
	if err != nil {
		return retrySubgroupIndexError(ctx, uid, pendingKey, "read pending subgroup move for deletion", err)
	}
	parents := make(map[string]struct{}, 3)
	for _, candidate := range []struct {
		uid     string
		present bool
	}{{parentUID, present}, {publishedService, published}, {pendingService, pending}} {
		if !candidate.present {
			continue
		}
		if !constants.ValidKVKeySegment(uid) || !constants.ValidKVKeySegment(candidate.uid) {
			slog.ErrorContext(ctx, "invalid subgroup or parent UID for service index cleanup", "uid", uid, "parent_uid", candidate.uid)
			return true
		}
		parents[candidate.uid] = struct{}{}
	}
	for serviceUID := range parents {
		indexKey := fmt.Sprintf("%s.%s.%s", constants.KVMappingPrefixSubgroupByService, serviceUID, uid)
		if err := mappings.PutTombstone(ctx, indexKey); err != nil {
			return retrySubgroupIndexError(ctx, uid, indexKey, "remove subgroup service index", err)
		}
	}
	if present {
		if err := mappings.PutTombstone(ctx, parentKey); err != nil {
			return retrySubgroupIndexError(ctx, uid, parentKey, "remove subgroup parent mapping", err)
		}
	}
	if pending {
		if err := mappings.PutTombstone(ctx, pendingKey); err != nil {
			return retrySubgroupIndexError(ctx, uid, pendingKey, "clear pending subgroup move", err)
		}
	}
	if err := mappings.PutTombstone(ctx, subgroupKey); err != nil {
		slog.ErrorContext(ctx, "failed to put tombstone, will retry", "mapping_key", subgroupKey, "error", err)
		return true
	}
	if err := mappings.PutTombstone(ctx, publishedParentKey); err != nil {
		return retrySubgroupIndexError(ctx, uid, publishedParentKey, "remove published subgroup parent", err)
	}
	unassociatedKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociated, uid)
	if err := mappings.PutTombstone(ctx, unassociatedKey); err != nil {
		return retrySubgroupIndexError(ctx, uid, unassociatedKey, "remove subgroup unassociation marker", err)
	}
	unassociationPendingKey := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupUnassociationPending, uid)
	if err := mappings.PutTombstone(ctx, unassociationPendingKey); err != nil {
		return retrySubgroupIndexError(ctx, uid, unassociationPendingKey, "remove subgroup unassociation progress", err)
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
	// visibility is the legacy ITX/DynamoDB field (itx-groupsio-v2-subgroup.visibility), synced
	// verbatim via lfx-v1-sync-helper with no casing normalization upstream — hence EqualFold below.
	visibility := mapconv.StringVal(data, "visibility")
	list := &model.GroupsIOMailingList{
		UID:            uid,
		GroupID:        mapconv.Int64Ptr(data, "group_id"),
		GroupName:      mapconv.StringVal(data, "group_name"),
		Public:         strings.EqualFold(visibility, "public"),
		AudienceAccess: visibility,
		Type:           mapconv.StringVal(data, "type"),
		Description:    mapconv.StringVal(data, "description"),
		Title:          mapconv.StringVal(data, "title"),
		SubjectTag:     mapconv.StringVal(data, "subject_tag"),
		URL:            mapconv.StringVal(data, "url"),
		Flags:          mapconv.StringSliceVal(data, "flags"),
		ServiceUID:     mapconv.StringVal(data, "parent_id"),
		ProjectUID:     mapconv.StringVal(data, "project_id"),
		Source:         "v1-sync",
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
			list.SystemUpdatedAt = &t
		}
	}

	return list
}
