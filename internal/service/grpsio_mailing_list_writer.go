// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"
)

// ensureMailingListIdempotent checks if mailing list with SubgroupID already exists
// Returns existing entity if found, nil if not found, error on failure
// This provides early-exit idempotency for all sources (API retries, webhooks, etc.)
func (ml *grpsIOWriterOrchestrator) ensureMailingListIdempotent(
	ctx context.Context,
	request *model.GrpsIOMailingList,
) (*model.GrpsIOMailingList, uint64, error) {
	// Only check if SubgroupID is provided (webhook or API retry after Groups.io creation)
	if request.SubgroupID == nil {
		return nil, 0, nil // No SubgroupID, proceed with normal creation
	}

	subgroupID := uint64(*request.SubgroupID)

	slog.DebugContext(ctx, "checking idempotency by subgroup_id",
		"subgroup_id", subgroupID,
		"source", request.Source)

	// Check secondary index for existing record
	existing, revision, err := ml.grpsIOReader.GetMailingListByGroupID(ctx, subgroupID)
	if err != nil {
		// Not found or error - let creation proceed
		slog.DebugContext(ctx, "no existing mailing list found by subgroup_id, proceeding with creation",
			"subgroup_id", subgroupID,
			"error", err)
		return nil, 0, nil
	}

	if existing != nil {
		// Found existing record - idempotent success
		slog.InfoContext(ctx, "mailing list already exists, returning existing record (idempotent)",
			"mailing_list_uid", existing.UID,
			"subgroup_id", subgroupID,
			"existing_source", existing.Source,
			"request_source", request.Source)
		return existing, revision, nil
	}

	return nil, 0, nil
}

// CreateGrpsIOMailingList creates a new mailing list with comprehensive validation and messaging
func (ml *grpsIOWriterOrchestrator) CreateGrpsIOMailingList(ctx context.Context, request *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "orchestrator: creating mailing list",
		"group_name", request.GroupName,
		"parent_uid", request.ServiceUID,
		"committee_uid", request.CommitteeUID,
		"source", request.Source,
		"subgroup_id", request.SubgroupID)

	// LAYER 1: Early idempotency check (prevents wasted work)
	if existing, revision, err := ml.ensureMailingListIdempotent(ctx, request); err != nil {
		return nil, 0, err
	} else if existing != nil {
		return existing, revision, nil // Already exists - idempotent success
	}

	// For rollback purposes
	var (
		keys                   []string
		rollbackRequired       bool
		rollbackSubgroupID     *int64
		rollbackGroupsIODomain string
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			ml.deleteKeys(ctx, keys, true)

			// Clean up Groups.io subgroup ONLY if we created it (not webhook)
			if rollbackSubgroupID != nil &&
				ml.groupsClient != nil &&
				request.Source == constants.SourceAPI {
				if deleteErr := ml.groupsClient.DeleteSubgroup(ctx, rollbackGroupsIODomain,
					utils.Int64PtrToUint64(rollbackSubgroupID)); deleteErr != nil {
					slog.WarnContext(ctx, "failed to cleanup GroupsIO subgroup during rollback",
						"error", deleteErr, "subgroup_id", *rollbackSubgroupID)
				}
			}
		}
	}()

	// Step 1: Validate timestamps
	if err := request.ValidateLastReviewedAt(); err != nil {
		slog.ErrorContext(ctx, "invalid LastReviewedAt timestamp",
			"error", err,
			"last_reviewed_at", request.LastReviewedAt,
		)
		return nil, 0, errors.NewValidation(fmt.Sprintf("invalid LastReviewedAt: %s", err.Error()))
	}

	// Step 2: Generate UID and set timestamps
	request.UID = uuid.New().String()
	now := time.Now()
	request.CreatedAt = now
	request.UpdatedAt = now

	// Step 3: Validate basic fields
	if err := request.ValidateBasicFields(); err != nil {
		slog.WarnContext(ctx, "basic field validation failed", "error", err)
		return nil, 0, err
	}

	// Step 4: Validate committee fields
	if err := request.ValidateCommitteeFields(); err != nil {
		slog.WarnContext(ctx, "committee field validation failed", "error", err)
		return nil, 0, err
	}

	// Step 5: Validate parent service and inherit metadata
	parentService, err := ml.validateAndInheritFromParent(ctx, request)
	if err != nil {
		return nil, 0, err
	}

	// Step 6: Validate committee and populate metadata (if specified)
	if err := ml.validateAndPopulateCommittee(ctx, request, parentService.ProjectUID); err != nil {
		return nil, 0, err
	}

	// Step 7: Validate group name prefix for non-primary services
	if err := request.ValidateGroupNamePrefix(parentService.Type, parentService.Prefix); err != nil {
		slog.WarnContext(ctx, "group name prefix validation failed", "error", err)
		return nil, 0, err
	}

	// Step 8: Reserve unique constraints (LAYER 3: Catches duplicates by name)
	constraintKey, err := ml.reserveMailingListConstraints(ctx, request)
	if err != nil {
		// LAYER 3.1: Graceful conflict handling for webhook race condition
		var conflictErr errors.Conflict
		if request.Source == constants.SourceWebhook && stdErrors.As(err, &conflictErr) {
			// Webhook arrived while API was in-flight
			// Check if existing record has same SubgroupID
			if existing, revision, checkErr := ml.ensureMailingListIdempotent(ctx, request); checkErr == nil && existing != nil {
				slog.InfoContext(ctx, "constraint conflict resolved - returning existing record (race condition)",
					"mailing_list_uid", existing.UID,
					"subgroup_id", request.SubgroupID)
				return existing, revision, nil
			}
		}
		// Genuine conflict or other error
		rollbackRequired = true
		return nil, 0, err
	}
	if constraintKey != "" {
		keys = append(keys, constraintKey)
	}

	// LAYER 2: Validate source
	if err := constants.ValidateSource(request.Source); err != nil {
		return nil, 0, err
	}

	// LAYER 3: Source-based strategy dispatch for SubgroupID resolution
	var (
		subgroupID      *int64
		requiresCleanup bool
	)

	switch request.Source {
	case constants.SourceAPI:
		subgroupID, requiresCleanup, err = ml.handleAPISourceMailingList(ctx, request, parentService)
		if err != nil {
			rollbackRequired = true
			return nil, 0, err
		}
		if requiresCleanup {
			rollbackSubgroupID = subgroupID
			rollbackGroupsIODomain = parentService.Domain
		}

	case constants.SourceWebhook:
		subgroupID, err = ml.handleWebhookSourceMailingList(ctx, request)
		if err != nil {
			return nil, 0, err
		}

	case constants.SourceMock:
		subgroupID = ml.handleMockSourceMailingList(ctx, request)
	}

	// Set SubgroupID from strategy result
	request.SubgroupID = subgroupID

	// Step 10: Create mailing list in storage (with Groups.io ID already set)
	createdMailingList, revision, err := ml.grpsIOWriter.CreateGrpsIOMailingList(ctx, request)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list in storage", "error", err)
		rollbackRequired = true
		return nil, 0, err
	}
	keys = append(keys, createdMailingList.UID)

	// Step 11: Create secondary indices for the mailing list
	secondaryKeys, err := ml.createMailingListSecondaryIndices(ctx, createdMailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list secondary indices", "error", err)
		rollbackRequired = true
		return nil, 0, err
	}

	// Add secondary keys to rollback list
	keys = append(keys, secondaryKeys...)

	// Step 12: Publish messages concurrently (indexer + access control)
	if err := ml.publishMailingListMessages(ctx, createdMailingList); err != nil {
		// Log warning but don't fail the operation - mailing list is already created
		slog.WarnContext(ctx, "failed to publish messages", "error", err, "mailing_list_uid", createdMailingList.UID)
	}

	slog.InfoContext(ctx, "mailing list created successfully",
		"mailing_list_uid", createdMailingList.UID,
		"group_name", createdMailingList.GroupName,
		"source", createdMailingList.Source,
		"subgroup_id", createdMailingList.SubgroupID,
		"parent_uid", createdMailingList.ServiceUID,
		"public", createdMailingList.Public,
		"committee_based", createdMailingList.IsCommitteeBased())

	return createdMailingList, revision, nil
}

// createMailingListInGroupsIO handles Groups.io subgroup creation and returns the ID
func (ml *grpsIOWriterOrchestrator) createMailingListInGroupsIO(ctx context.Context, mailingList *model.GrpsIOMailingList, parentService *model.GrpsIOService) (*int64, error) {
	if ml.groupsClient == nil || parentService.GroupID == nil {
		return nil, nil // Skip Groups.io creation
	}

	slog.InfoContext(ctx, "creating subgroup in Groups.io",
		"domain", parentService.Domain,
		"parent_group_id", *parentService.GroupID,
		"subgroup_name", mailingList.GroupName,
	)

	subgroupOptions := groupsio.SubgroupCreateOptions{
		ParentGroupID: utils.Int64PtrToUint64(parentService.GroupID),                                             // Production field
		GroupName:     mailingList.GroupName,                                                                     // Fixed: was SubgroupName
		Desc:          fmt.Sprintf("Mailing list for %s - %s", parentService.ProjectName, mailingList.GroupName), // Fixed: was Description
		// Privacy: leave empty to inherit from parent group (production pattern)
	}

	subgroupResult, err := ml.groupsClient.CreateSubgroup(
		ctx,
		parentService.Domain,
		utils.Int64PtrToUint64(parentService.GroupID),
		subgroupOptions,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Groups.io subgroup creation failed",
			"error", err,
			"domain", parentService.Domain,
			"parent_group_id", *parentService.GroupID,
			"subgroup_name", mailingList.GroupName,
		)
		return nil, fmt.Errorf("groups.io subgroup creation failed: %w", err)
	}

	subgroupID := int64(subgroupResult.ID)
	slog.InfoContext(ctx, "Groups.io subgroup created successfully",
		"subgroup_id", subgroupResult.ID,
		"domain", parentService.Domain,
		"parent_group_id", *parentService.GroupID,
	)

	return &subgroupID, nil
}

// validateAndInheritFromParent validates parent service exists and inherits metadata
func (ml *grpsIOWriterOrchestrator) validateAndInheritFromParent(ctx context.Context, request *model.GrpsIOMailingList) (*model.GrpsIOService, error) {
	slog.DebugContext(ctx, "validating parent service", "parent_uid", request.ServiceUID)

	// Get parent service from storage
	parentService, _, err := ml.grpsIOReader.GetGrpsIOService(ctx, request.ServiceUID)
	if err != nil {
		slog.WarnContext(ctx, "parent service validation failed", "parent_uid", request.ServiceUID, "error", err)
		return nil, errors.NewNotFound("parent service not found")
	}

	// Inherit project metadata from parent service
	request.ProjectUID = parentService.ProjectUID
	request.ProjectName = parentService.ProjectName
	request.ProjectSlug = parentService.ProjectSlug

	slog.DebugContext(ctx, "parent service validated successfully",
		"parent_uid", request.ServiceUID,
		"parent_type", parentService.Type,
		"project_uid", parentService.ProjectUID,
		"project_name", parentService.ProjectName,
		"project_slug", parentService.ProjectSlug,
		"prefix", parentService.Prefix)

	return parentService, nil
}

// validateAndPopulateCommittee validates committee exists and populates committee metadata
func (ml *grpsIOWriterOrchestrator) validateAndPopulateCommittee(ctx context.Context, request *model.GrpsIOMailingList, projectID string) error {
	if request.CommitteeUID == "" {
		// No committee specified, validation not needed
		return nil
	}

	slog.DebugContext(ctx, "validating and populating committee",
		"committee_uid", request.CommitteeUID,
		"project_uid", projectID)

	// Get committee name to validate it exists and populate metadata
	committeeName, err := ml.entityReader.CommitteeName(ctx, request.CommitteeUID)
	if err != nil {
		slog.WarnContext(ctx, "committee validation failed",
			"committee_uid", request.CommitteeUID,
			"project_uid", projectID,
			"error", err)
		return errors.NewNotFound("committee not found")
	}

	// Populate committee name
	request.CommitteeName = committeeName

	slog.DebugContext(ctx, "committee validated and populated successfully",
		"committee_uid", request.CommitteeUID,
		"committee_name", committeeName,
		"project_uid", projectID)

	return nil
}

// reserveMailingListConstraints reserves unique constraints for mailing list creation
func (ml *grpsIOWriterOrchestrator) reserveMailingListConstraints(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	// For mailing lists, we have one constraint type: unique group name within parent service
	return ml.grpsIOWriter.UniqueMailingListGroupName(ctx, mailingList)
}

// publishMailingListMessages publishes indexer and access control messages for mailing list creation
func (ml *grpsIOWriterOrchestrator) publishMailingListMessages(ctx context.Context, mailingList *model.GrpsIOMailingList) error {
	if ml.publisher == nil {
		slog.DebugContext(ctx, "publisher not configured, skipping message publishing",
			"mailing_list_uid", mailingList.UID)
		return nil
	}
	return ml.publishMailingListChange(ctx, mailingList, model.ActionCreated)
}

// publishMailingListUpdateMessages publishes update messages for indexer and access control
func (ml *grpsIOWriterOrchestrator) publishMailingListUpdateMessages(ctx context.Context, mailingList *model.GrpsIOMailingList) error {
	return ml.publishMailingListChange(ctx, mailingList, model.ActionUpdated)
}

// publishMailingListDeleteMessages publishes delete messages for indexer and access control
func (ml *grpsIOWriterOrchestrator) publishMailingListDeleteMessages(ctx context.Context, uid string) error {
	return ml.publishMailingListDeletion(ctx, uid)
}

// buildMailingListIndexerMessage builds an indexer message for search capabilities
func (ml *grpsIOWriterOrchestrator) buildMailingListIndexerMessage(ctx context.Context, mailingList *model.GrpsIOMailingList, action model.MessageAction) (*model.IndexerMessage, error) {
	indexerMessage := &model.IndexerMessage{
		Action: action,
		Tags:   mailingList.Tags(),
	}

	// Build the message with proper context and authorization headers
	return indexerMessage.Build(ctx, mailingList)
}

// buildMailingListAccessControlMessage builds an access control message for OpenFGA
func (ml *grpsIOWriterOrchestrator) buildMailingListAccessControlMessage(mailingList *model.GrpsIOMailingList) *model.AccessMessage {
	references := map[string]string{
		constants.RelationGroupsIOService: mailingList.ServiceUID, // Required for service-level permission inheritance (project inherited through service)
	}

	// Add committee reference for committee-based lists (enables committee-level authorization)
	if mailingList.CommitteeUID != "" {
		references[constants.RelationCommittee] = mailingList.CommitteeUID
	}

	relations := map[string][]string{}
	if len(mailingList.Writers) > 0 {
		relations[constants.RelationWriter] = mailingList.Writers
	}
	if len(mailingList.Auditors) > 0 {
		relations[constants.RelationAuditor] = mailingList.Auditors
	}

	return &model.AccessMessage{
		UID:        mailingList.UID,
		ObjectType: constants.ObjectTypeGroupsIOMailingList,
		Public:     mailingList.Public, // Using Public bool instead of Visibility
		Relations:  relations,
		References: references,
	}
}

// UpdateGrpsIOMailingList updates an existing mailing list with optimistic concurrency control
func (ml *grpsIOWriterOrchestrator) UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "orchestrator: updating mailing list",
		"mailing_list_uid", uid,
		"expected_revision", expectedRevision)

	// Step 1: Validate timestamps in input
	if err := mailingList.ValidateLastReviewedAt(); err != nil {
		slog.ErrorContext(ctx, "invalid LastReviewedAt timestamp",
			"error", err,
			"last_reviewed_at", mailingList.LastReviewedAt,
			"mailing_list_uid", uid,
		)
		return nil, 0, errors.NewValidation(fmt.Sprintf("invalid LastReviewedAt: %s", err.Error()))
	}

	// Step 2: Retrieve existing mailing list to validate and merge data
	existing, existingRevision, err := ml.grpsIOReader.GetGrpsIOMailingList(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing mailing list",
			"error", err,
			"mailing_list_uid", uid,
		)
		return nil, 0, err
	}

	// Step 3: Verify revision matches to ensure optimistic locking
	if existingRevision != expectedRevision {
		slog.WarnContext(ctx, "revision mismatch during update",
			"expected_revision", expectedRevision,
			"current_revision", existingRevision,
			"mailing_list_uid", uid,
		)
		return nil, 0, errors.NewConflict("mailing list has been modified by another process")
	}

	// Step 4: Merge existing data with updated fields
	ml.mergeMailingListData(ctx, existing, mailingList)

	// Step 4.1: Re-validate fields after merge to ensure data integrity
	if err := mailingList.ValidateBasicFields(); err != nil {
		slog.WarnContext(ctx, "basic field validation failed during update", "error", err)
		return nil, 0, err
	}
	if err := mailingList.ValidateCommitteeFields(); err != nil {
		slog.WarnContext(ctx, "committee field validation failed during update", "error", err)
		return nil, 0, err
	}

	// Step 3.2: Validate parent service constraints and refresh committee name if needed
	parentSvc, _, err := ml.grpsIOReader.GetGrpsIOService(ctx, mailingList.ServiceUID)
	if err != nil {
		slog.WarnContext(ctx, "parent service not found during update", "error", err, "parent_uid", mailingList.ServiceUID)
		return nil, 0, errors.NewNotFound("parent service not found")
	}
	if err := mailingList.ValidateGroupNamePrefix(parentSvc.Type, parentSvc.Prefix); err != nil {
		slog.WarnContext(ctx, "group name prefix validation failed during update", "error", err)
		return nil, 0, err
	}

	// Step 3.3: Refresh committee name if committee UID changed
	if mailingList.CommitteeUID != "" && mailingList.CommitteeUID != existing.CommitteeUID {
		if err := ml.validateAndPopulateCommittee(ctx, mailingList, parentSvc.ProjectUID); err != nil {
			return nil, 0, err
		}
	}

	// Step 4: Update in storage with revision check
	updatedMailingList, newRevision, err := ml.grpsIOWriter.UpdateGrpsIOMailingList(ctx, uid, mailingList, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update mailing list in storage",
			"error", err,
			"mailing_list_uid", uid,
			"expected_revision", expectedRevision)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "mailing list updated successfully",
		"mailing_list_uid", uid,
		"revision", newRevision,
	)

	// Sync mailing list updates to Groups.io
	ml.syncMailingListToGroupsIO(ctx, updatedMailingList)

	// Publish update messages
	if ml.publisher != nil {
		if err := ml.publishMailingListUpdateMessages(ctx, updatedMailingList); err != nil {
			slog.ErrorContext(ctx, "failed to publish update messages", "error", err)
			// Don't fail the update on message publishing errors
		}
	}

	slog.InfoContext(ctx, "mailing list updated successfully",
		"mailing_list_uid", uid,
		"group_name", updatedMailingList.GroupName,
		"new_revision", newRevision)

	return updatedMailingList, newRevision, nil
}

// createMailingListSecondaryIndices creates all secondary indices for the mailing list in the orchestrator layer
func (ml *grpsIOWriterOrchestrator) createMailingListSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	// Use CreateSecondaryIndices method from the storage layer interface
	createdKeys, err := ml.grpsIOWriter.CreateSecondaryIndices(ctx, mailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create secondary indices", "error", err)
		return nil, err
	}

	slog.DebugContext(ctx, "secondary indices created successfully",
		"mailing_list_uid", mailingList.UID,
		"indices_created", createdKeys)

	return createdKeys, nil
}

// publishIndexerMessage is a helper for indexer message publishing
func (ml *grpsIOWriterOrchestrator) publishIndexerMessage(ctx context.Context, message any, action model.MessageAction) error {
	if err := ml.publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, message); err != nil {
		slog.ErrorContext(ctx, "failed to publish indexer message", "error", err, "action", action)
		return fmt.Errorf("failed to publish %s indexer message: %w", action, err)
	}
	return nil
}

// publishMailingListChange publishes indexer and access control messages for create/update operations
func (ml *grpsIOWriterOrchestrator) publishMailingListChange(ctx context.Context, mailingList *model.GrpsIOMailingList, action model.MessageAction) error {
	slog.DebugContext(ctx, "publishing messages for mailing list",
		"action", action,
		"mailing_list_uid", mailingList.UID)

	// Build and publish indexer message
	indexerMessage, err := ml.buildMailingListIndexerMessage(ctx, mailingList, action)
	if err != nil {
		return fmt.Errorf("failed to build %s indexer message: %w", action, err)
	}

	if err := ml.publishIndexerMessage(ctx, indexerMessage, action); err != nil {
		return err
	}

	// Publish access control message
	accessMessage := ml.buildMailingListAccessControlMessage(mailingList)
	if err := ml.publisher.Access(ctx, constants.UpdateAccessGroupsIOMailingListSubject, accessMessage); err != nil {
		slog.ErrorContext(ctx, "failed to publish access control message", "error", err, "action", action)
		return fmt.Errorf("failed to publish %s access control message: %w", action, err)
	}

	slog.DebugContext(ctx, "messages published successfully",
		"action", action,
		"mailing_list_uid", mailingList.UID)
	return nil
}

// publishMailingListDeletion publishes indexer and access control messages for delete operations
func (ml *grpsIOWriterOrchestrator) publishMailingListDeletion(ctx context.Context, uid string) error {
	slog.DebugContext(ctx, "publishing delete messages for mailing list",
		"mailing_list_uid", uid)

	// Build deletion indexer message
	deleteMessage := &model.IndexerMessage{
		Action: model.ActionDeleted,
		Tags:   []string{},
	}

	indexerMessage, err := deleteMessage.Build(ctx, uid)
	if err != nil {
		return fmt.Errorf("failed to build delete indexer message: %w", err)
	}

	if err := ml.publishIndexerMessage(ctx, indexerMessage, model.ActionDeleted); err != nil {
		return err
	}

	// Publish access control deletion
	if err := ml.publisher.Access(ctx, constants.DeleteAllAccessGroupsIOMailingListSubject, uid); err != nil {
		slog.ErrorContext(ctx, "failed to publish delete access control message", "error", err)
		return fmt.Errorf("failed to publish delete access control message: %w", err)
	}

	slog.DebugContext(ctx, "delete messages published successfully",
		"mailing_list_uid", uid)
	return nil
}

// DeleteGrpsIOMailingList deletes a mailing list with optimistic concurrency control
// Note: mailingList parameter contains server-fetched data from the service layer,
// not client-supplied data. Used for cleanup of secondary indices and constraints.
func (ml *grpsIOWriterOrchestrator) DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error {
	slog.DebugContext(ctx, "orchestrator: deleting mailing list",
		"mailing_list_uid", uid,
		"expected_revision", expectedRevision)

	// Use the passed mailing list data - no need to fetch again
	mailingListData := mailingList

	// Step 2: Deletion validation
	// Validates main group protection, announcement list protection, and committee associations
	// TODO: LFXV2-353 - Enhance with Groups.io API integration to validate:
	//   - Active subscriber count thresholds
	//   - Recent message activity
	//   - Pending moderation queue items
	// TODO: LFXV2-478 - Enhance with committee event handling to:
	//   - Block deletion if active committee sync is running
	//   - Trigger committee member cleanup
	slog.DebugContext(ctx, "validating mailing list deletion",
		"mailing_list_uid", uid,
		"group_name", mailingListData.GroupName,
		"public", mailingListData.Public)

	// Step 2.1: Delete subgroup from Groups.io (if client available and mailing list has SubgroupID)
	ml.deleteSubgroupWithCleanup(ctx, mailingListData.ServiceUID, mailingListData.SubgroupID)

	// Delete from storage with revision check
	err := ml.grpsIOWriter.DeleteGrpsIOMailingList(ctx, uid, expectedRevision, mailingListData)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete mailing list from storage", "error", err, "mailing_list_uid", uid)
		return err
	}

	// Publish delete messages
	if ml.publisher != nil {
		if err := ml.publishMailingListDeleteMessages(ctx, uid); err != nil {
			slog.ErrorContext(ctx, "failed to publish delete messages", "error", err)
		}
	}

	slog.InfoContext(ctx, "mailing list deleted successfully",
		"mailing_list_uid", uid,
		"group_name", mailingListData.GroupName)

	return nil
}

// mergeMailingListData merges existing mailing list data with updated fields, preserving immutable fields
func (ml *grpsIOWriterOrchestrator) mergeMailingListData(ctx context.Context, existing *model.GrpsIOMailingList, updated *model.GrpsIOMailingList) {
	// Preserve immutable fields
	updated.UID = existing.UID
	updated.CreatedAt = existing.CreatedAt
	updated.ProjectUID = existing.ProjectUID   // Inherited from parent service
	updated.ProjectName = existing.ProjectName // Inherited from parent service
	updated.ProjectSlug = existing.ProjectSlug // Inherited from parent service
	updated.ServiceUID = existing.ServiceUID   // Parent reference is immutable
	updated.GroupName = existing.GroupName     // Group name is immutable due to unique constraint

	// Committee name handling: preserve if UID unchanged, clear if UID removed, use provided if changed
	if updated.CommitteeUID == existing.CommitteeUID {
		updated.CommitteeName = existing.CommitteeName
	} else if updated.CommitteeUID == "" {
		updated.CommitteeName = "" // Clear name when committee association removed
	}
	// If UID changed to different committee, keep client-provided name

	// Update timestamp
	updated.UpdatedAt = time.Now()

	slog.DebugContext(ctx, "mailing list data merged",
		"mailing_list_uid", existing.UID,
		"mutable_fields", []string{"public", "type", "description", "title", "committee_uid", "committee_name", "committee_filters", "subject_tag", "writers", "auditors", "last_reviewed_at", "last_reviewed_by"},
	)
}

// syncMailingListToGroupsIO handles Groups.io mailing list update with proper error handling
func (ml *grpsIOWriterOrchestrator) syncMailingListToGroupsIO(ctx context.Context, mailingList *model.GrpsIOMailingList) {
	// Guard clause: skip if Groups.io client not available or mailing list not synced
	if ml.groupsClient == nil || mailingList.SubgroupID == nil {
		slog.InfoContext(ctx, "Groups.io integration disabled or mailing list not synced - skipping Groups.io update")
		return
	}

	// Get domain using helper method
	domain, err := ml.getGroupsIODomainForResource(ctx, mailingList.UID, constants.ResourceTypeMailingList)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io mailing list sync skipped due to domain lookup failure, local update will proceed",
			"error", err, "mailing_list_uid", mailingList.UID)
		return
	}

	// Build update options from mailing list model
	updates := groupsio.SubgroupUpdateOptions{
		Title:       mailingList.Title,
		Description: mailingList.Description,
		SubjectTag:  mailingList.SubjectTag,
		Committee:   mailingList.CommitteeUID,
	}

	// Perform Groups.io mailing list update
	err = ml.groupsClient.UpdateSubgroup(ctx, domain, utils.Int64PtrToUint64(mailingList.SubgroupID), updates)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io mailing list update failed, local update will proceed",
			"error", err, "domain", domain, "subgroup_id", *mailingList.SubgroupID)
	} else {
		slog.InfoContext(ctx, "Groups.io mailing list updated successfully",
			"subgroup_id", *mailingList.SubgroupID, "domain", domain)
	}
}

// handleAPISourceMailingList handles API-initiated mailing list creation
// Preserves existing logic: calls createMailingListInGroupsIO and returns cleanup flag
func (ml *grpsIOWriterOrchestrator) handleAPISourceMailingList(
	ctx context.Context,
	request *model.GrpsIOMailingList,
	parentService *model.GrpsIOService,
) (*int64, bool, error) {
	slog.InfoContext(ctx, "source=api: creating subgroup in Groups.io",
		"group_name", request.GroupName,
		"parent_uid", parentService.UID)

	// Call existing createMailingListInGroupsIO method (preserves all existing logic)
	subgroupID, err := ml.createMailingListInGroupsIO(ctx, request, parentService)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create subgroup in Groups.io",
			"error", err,
			"group_name", request.GroupName)
		return nil, false, err
	}

	// Determine if cleanup is required (preserves existing rollback logic)
	requiresCleanup := subgroupID != nil && parentService.Domain != ""

	if subgroupID != nil {
		slog.InfoContext(ctx, "subgroup created successfully in Groups.io",
			"subgroup_id", *subgroupID)
	}

	return subgroupID, requiresCleanup, nil
}

// handleWebhookSourceMailingList handles webhook-initiated mailing list adoption
// Preserves existing logic: validates SubgroupID and returns it
func (ml *grpsIOWriterOrchestrator) handleWebhookSourceMailingList(
	ctx context.Context,
	request *model.GrpsIOMailingList,
) (*int64, error) {
	if request.SubgroupID == nil {
		return nil, errors.NewValidation("webhook source requires SubgroupID to be provided")
	}

	slog.InfoContext(ctx, "source=webhook: adopting webhook-provided subgroup",
		"subgroup_id", *request.SubgroupID,
		"group_name", request.GroupName)

	return request.SubgroupID, nil
}

// handleMockSourceMailingList handles mock/test mode mailing list creation
// Preserves existing logic: returns nil for subgroupID
func (ml *grpsIOWriterOrchestrator) handleMockSourceMailingList(
	ctx context.Context,
	request *model.GrpsIOMailingList,
) *int64 {
	slog.InfoContext(ctx, "source=mock: skipping Groups.io coordination",
		"group_name", request.GroupName)
	return nil
}
