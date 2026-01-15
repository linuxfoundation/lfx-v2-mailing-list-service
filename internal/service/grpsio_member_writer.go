// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"
)

// groupsioMailingListMemberStub represents the minimal data needed for member access control
type groupsioMailingListMemberStub struct {
	// UID is the mailing list member ID.
	UID string `json:"uid"`
	// Username is the username (i.e. LFID) of the member. This is the identity of the user object in FGA.
	Username string `json:"username"`
	// MailingListUID is the mailing list ID for the mailing list the member belongs to.
	MailingListUID string `json:"mailing_list_uid"`
}

// ensureMemberIdempotent checks if member already exists by Groups.io member ID or email
// Returns existing entity if found, nil if not found, error on failure
// Pattern mirrors ensureMailingListIdempotent
func (o *grpsIOWriterOrchestrator) ensureMemberIdempotent(
	ctx context.Context,
	member *model.GrpsIOMember,
) (*model.GrpsIOMember, uint64, error) {

	// Strategy 1: Lookup by Groups.io member ID (webhook path)
	if member.MemberID != nil {
		memberID := uint64(*member.MemberID)

		slog.DebugContext(ctx, "checking idempotency by Groups.io member ID",
			"member_id", memberID,
			"source", member.Source)

		existing, revision, err := o.grpsIOReader.GetMemberByGroupsIOMemberID(ctx, memberID)
		if err != nil {
			// Use helper to handle idempotency lookup errors consistently
			shouldContinue, handledErr := handleIdempotencyLookupError(ctx, err, "member_id", fmt.Sprintf("%d", memberID))
			if !shouldContinue {
				return nil, 0, handledErr
			}
			// NotFound - fall through to Strategy 2 (email lookup)
		} else if existing != nil {
			// Found existing member - idempotent success
			slog.InfoContext(ctx, "member already exists by Groups.io member ID (idempotent)",
				"member_uid", existing.UID,
				"member_id", memberID,
				"existing_source", existing.Source,
				"request_source", member.Source)
			return existing, revision, nil
		}
	}

	// Strategy 2: Lookup by email (API retry or webhook without member ID)
	existing, revision, err := o.grpsIOReader.GetMemberByEmail(
		ctx, member.MailingListUID, member.Email)
	if err != nil {
		// Use helper to handle idempotency lookup errors consistently
		shouldContinue, handledErr := handleIdempotencyLookupError(ctx, err, "email", redaction.RedactEmail(member.Email))
		if !shouldContinue {
			return nil, 0, handledErr
		}
		// NotFound - proceed with creation
		slog.DebugContext(ctx, "no existing member found, proceeding with creation")
		return nil, 0, nil
	}

	if existing != nil {
		slog.InfoContext(ctx, "member already exists by email (idempotent)",
			"member_uid", existing.UID,
			"email", redaction.RedactEmail(member.Email),
			"existing_source", existing.Source,
			"request_source", member.Source)
		return existing, revision, nil
	}

	// Not found - proceed with creation
	slog.DebugContext(ctx, "no existing member found, proceeding with creation")
	return nil, 0, nil
}

// CreateGrpsIOMember creates a new member with transactional operations and rollback following service pattern
func (o *grpsIOWriterOrchestrator) CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "executing create member use case",
		"member_email", redaction.RedactEmail(member.Email),
		"mailing_list_uid", member.MailingListUID,
	)

	// LAYER 1: Early idempotency check (prevents wasted work)
	// Pattern matches: ensureMailingListIdempotent in CreateGrpsIOMailingList
	if existing, revision, err := o.ensureMemberIdempotent(ctx, member); err != nil {
		return nil, 0, err
	} else if existing != nil {
		return existing, revision, nil // Already exists - idempotent success
	}

	// Step 1: Validate timestamps
	if err := member.ValidateLastReviewedAt(); err != nil {
		slog.ErrorContext(ctx, "invalid LastReviewedAt timestamp",
			"error", err,
			"last_reviewed_at", member.LastReviewedAt,
		)
		return nil, 0, errs.NewValidation(fmt.Sprintf("invalid LastReviewedAt: %s", err.Error()))
	}

	// Step 2: Generate UID and set timestamps (server-side generation for security)
	now := time.Now()
	member.UID = uuid.New().String() // Always generate server-side, never trust client
	member.CreatedAt = now
	member.UpdatedAt = now

	// For rollback purposes
	var (
		keys                   []string
		rollbackRequired       bool
		rollbackMemberID       *uint64
		rollbackGroupsIODomain string
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			o.deleteKeys(ctx, keys, true)

			// Clean up GroupsIO member if created
			if rollbackMemberID != nil && o.groupsClient != nil {
				if deleteErr := o.groupsClient.RemoveMember(ctx, rollbackGroupsIODomain, uint64(*member.GroupID), *rollbackMemberID); deleteErr != nil {
					slog.WarnContext(ctx, "failed to cleanup GroupsIO member during rollback", "error", deleteErr, "group_id", *member.GroupID, "member_id", *rollbackMemberID)
				}
			}
		}
	}()

	// Step 3: Validate mailing list exists and populate metadata
	if err := o.validateAndPopulateMailingList(ctx, member); err != nil {
		slog.ErrorContext(ctx, "mailing list validation failed",
			"error", err,
			"mailing_list_uid", member.MailingListUID,
		)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "mailing list validation successful",
		"mailing_list_uid", member.MailingListUID,
	)

	// Step 4: Set default status if not provided
	if member.Status == "" {
		member.Status = "pending"
	}

	// Step 5: Reserve unique constraints (member email per mailing list)
	constraintKey, err := o.grpsIOWriter.UniqueMember(ctx, member)
	if err != nil {
		rollbackRequired = true
		return nil, 0, err
	}
	if constraintKey != "" {
		keys = append(keys, constraintKey)
	}

	// Step 6: Get mailing list (needed for all sources)
	mailingList, _, err := o.grpsIOReader.GetGrpsIOMailingList(ctx, member.MailingListUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list for Groups.io sync", "error", err, "mailing_list_uid", member.MailingListUID)
		rollbackRequired = true
		return nil, 0, err
	}

	// Step 7: Handle member creation based on source (API, webhook, or mock)
	memberID, groupID, requiresCleanup, err := o.handleMemberCreationBySource(ctx, member, mailingList)
	if err != nil {
		rollbackRequired = true
		return nil, 0, err
	}

	// Set rollback information if cleanup is required
	if requiresCleanup {
		rollbackMemberID = utils.Int64PtrToUint64Ptr(memberID)
		// Get parent service domain for rollback (only when needed)
		parentService, _, svcErr := o.grpsIOReader.GetGrpsIOService(ctx, mailingList.ServiceUID)
		if svcErr == nil {
			rollbackGroupsIODomain = parentService.GetDomain()
		}
	}

	member.MemberID = memberID
	member.GroupID = groupID

	// Step 9: Create member in storage (with Groups.io IDs already set)
	createdMember, revision, err := o.grpsIOWriter.CreateGrpsIOMember(ctx, member)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create member",
			"error", err,
			"member_email", redaction.RedactEmail(member.Email),
			"mailing_list_uid", member.MailingListUID,
		)
		rollbackRequired = true
		return nil, 0, err
	}
	keys = append(keys, createdMember.UID)

	slog.DebugContext(ctx, "member created successfully",
		"member_uid", createdMember.UID,
		"revision", revision,
	)

	// Refresh subscriber count asynchronously after DirectAdd and NATS item creation
	// Run in background so NATS secondary indices creation and message sending can complete concurrently
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.updateMailingListSubscriberCount(ctx, mailingList.UID)
	}()

	// Step 9.5: Create secondary indices for Groups.io ID lookups
	// Only create if member has Groups.io IDs (skip for mock/pending members)
	// Pattern matches: createMailingListSecondaryIndices in CreateGrpsIOMailingList
	if createdMember.MemberID != nil || createdMember.GroupID != nil {
		secondaryKeys, err := o.grpsIOWriter.CreateMemberSecondaryIndices(ctx, createdMember)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create member secondary indices",
				"error", err,
				"member_uid", createdMember.UID,
			)
			rollbackRequired = true
			return nil, 0, err
		}
		keys = append(keys, secondaryKeys...)

		slog.DebugContext(ctx, "member secondary indices created",
			"member_uid", createdMember.UID,
			"indices_count", len(secondaryKeys))
	}

	// Step 10: Publish messages (indexer and access control)
	if o.publisher != nil {
		if err := o.publishMemberMessages(ctx, createdMember, model.ActionCreated); err != nil {
			slog.ErrorContext(ctx, "failed to publish member messages", "error", err)
			// Don't fail the operation on message failure, member creation succeeded
		}
	}

	wg.Wait()

	return createdMember, revision, nil
}

// createMemberInGroupsIO handles Groups.io member creation and returns the IDs
func (o *grpsIOWriterOrchestrator) createMemberInGroupsIO(ctx context.Context, member *model.GrpsIOMember, mailingList *model.GrpsIOMailingList, parentService *model.GrpsIOService) (*int64, *int64, error) {
	if o.groupsClient == nil || mailingList == nil || mailingList.GroupID == nil || parentService == nil || parentService.GroupID == nil {
		return nil, nil, nil // Skip Groups.io creation
	}

	domain := parentService.GetDomain()

	slog.InfoContext(ctx, "creating member in Groups.io",
		"domain", domain,
		"group_id", *mailingList.GroupID,
		"email", member.Email,
	)

	// Prepare email slice (single email for our use case)
	emails := []string{member.Email}

	// Prepare subgroup IDs slice (if adding to subgroup rather than main group)
	var subgroupIDs []uint64
	if *parentService.GroupID != *mailingList.GroupID {
		subgroupIDs = []uint64{uint64(*mailingList.GroupID)}
	}

	result, err := o.groupsClient.DirectAdd(
		ctx,
		domain,
		utils.Int64PtrToUint64(parentService.GroupID),
		emails,
		subgroupIDs,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Groups.io member creation failed",
			"error", err,
			"domain", domain,
			"group_id", *mailingList.GroupID,
			"email", member.Email,
		)
		return nil, nil, fmt.Errorf("groups.io member creation failed: %w", err)
	}

	// Check for errors in the response
	if len(result.Errors) > 0 {
		firstError := result.Errors[0]
		slog.ErrorContext(ctx, "Groups.io direct_add returned error",
			"email", firstError.Email,
			"status", firstError.Status,
			"group_id", firstError.GroupID,
			"domain", domain,
		)
		return nil, nil, fmt.Errorf("failed to add member %s: %s", firstError.Email, firstError.Status)
	}

	// Check if any members were added
	if len(result.AddedMembers) == 0 {
		slog.ErrorContext(ctx, "no members added via direct_add",
			"email", member.Email,
			"group_id", *mailingList.GroupID,
			"domain", domain,
		)
		return nil, nil, fmt.Errorf("no members were added for email %s", member.Email)
	}

	// Get the first (and only) added member
	addedMember := result.AddedMembers[0]
	memberID := int64(addedMember.ID)
	groupID := int64(addedMember.GroupID)

	slog.InfoContext(ctx, "Groups.io member created successfully",
		"member_id", memberID,
		"group_id", groupID,
		"domain", domain,
		"email", addedMember.Email,
	)

	return &memberID, &groupID, nil
}

// UpdateGrpsIOMember updates an existing member following the service pattern with pre-fetch and validation
func (o *grpsIOWriterOrchestrator) UpdateGrpsIOMember(ctx context.Context, uid string, member *model.GrpsIOMember, expectedRevision uint64) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "executing update member use case",
		"member_uid", uid,
		"expected_revision", expectedRevision,
	)

	// Step 1: Validate timestamps in input
	if err := member.ValidateLastReviewedAt(); err != nil {
		slog.ErrorContext(ctx, "invalid LastReviewedAt timestamp",
			"error", err,
			"last_reviewed_at", member.LastReviewedAt,
			"member_uid", uid,
		)
		return nil, 0, errs.NewValidation(fmt.Sprintf("invalid LastReviewedAt: %s", err.Error()))
	}

	// Step 2: Retrieve existing member to validate and merge data
	existing, existingRevision, err := o.grpsIOReader.GetGrpsIOMember(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing member",
			"error", err,
			"member_uid", uid,
		)
		return nil, 0, err
	}

	// Step 3: Verify revision matches to ensure optimistic locking
	if existingRevision != expectedRevision {
		slog.WarnContext(ctx, "revision mismatch during member update",
			"expected_revision", expectedRevision,
			"current_revision", existingRevision,
			"member_uid", uid,
		)
		return nil, 0, errs.NewConflict("member has been modified by another process")
	}

	// Step 4: Protect immutable fields
	if member.MailingListUID != "" && member.MailingListUID != existing.MailingListUID {
		return nil, 0, errs.NewValidation("field 'mailing_list_uid' is immutable")
	}
	if member.Email != "" && member.Email != existing.Email {
		return nil, 0, errs.NewValidation("field 'email' is immutable")
	}

	// Step 5: Merge existing data with updated fields
	o.mergeMemberData(ctx, existing, member)

	// Step 6: Update member in storage with optimistic concurrency control
	updatedMember, revision, err := o.grpsIOWriter.UpdateGrpsIOMember(ctx, uid, member, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update member",
			"error", err,
			"member_uid", uid,
		)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "member updated successfully",
		"member_uid", uid,
		"revision", revision,
	)

	// Step 6.1: Handle member update sync based on source (API, webhook, or mock)
	if err := o.handleMemberUpdateBySource(ctx, updatedMember); err != nil {
		slog.WarnContext(ctx, "Groups.io sync failed but local update succeeded",
			"error", err, "member_uid", uid)
		// Don't fail the operation - local update succeeded
	}

	// Step 7: Publish messages (indexer and access control)
	if o.publisher != nil {
		if err := o.publishMemberMessages(ctx, updatedMember, model.ActionUpdated); err != nil {
			slog.ErrorContext(ctx, "failed to publish member update messages", "error", err)
			// Don't fail the operation on message failure, update succeeded
		}
	}

	return updatedMember, revision, nil
}

// DeleteGrpsIOMember deletes a member following the service pattern
func (o *grpsIOWriterOrchestrator) DeleteGrpsIOMember(ctx context.Context, uid string, expectedRevision uint64, member *model.GrpsIOMember) error {
	slog.DebugContext(ctx, "executing delete member use case",
		"member_uid", uid,
		"expected_revision", expectedRevision,
	)

	if member == nil {
		return errs.NewValidation("member is required for deletion")
	}

	logger := slog.With("member_uid", uid, "source", member.Source)
	if member.GroupID != nil {
		logger = logger.With("group_id", *member.GroupID)
	}
	if member.MemberID != nil {
		logger = logger.With("member_id", *member.MemberID)
	}

	// Handle member deletion based on source (API, webhook, or mock)
	err := o.handleMemberDeletionBySource(ctx, member)
	if err != nil {
		logger.ErrorContext(ctx, "failed to remove member from source",
			"error", err,
		)
		return fmt.Errorf("failed to remove member from source: %w", err)
	}

	// Delete member from storage with optimistic concurrency control
	err = o.grpsIOWriter.DeleteGrpsIOMember(ctx, uid, expectedRevision, member)
	if err != nil {
		logger.ErrorContext(ctx, "failed to delete member",
			"error", err,
		)
		return err
	}

	logger.DebugContext(ctx, "member deleted successfully")

	// Update the mailing list subscriber count asynchronously
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.updateMailingListSubscriberCount(ctx, member.MailingListUID)
	}()

	// Publish delete messages (indexer and access control)
	if o.publisher != nil {
		if err := o.publishMemberDeleteMessages(ctx, uid, *member); err != nil {
			logger.ErrorContext(ctx, "failed to publish member delete messages", "error", err)
			// Don't fail the operation on message failure, delete succeeded
		}
	}

	wg.Wait()

	return nil
}

// validateAndPopulateMailingList validates mailing list exists and populates metadata
func (o *grpsIOWriterOrchestrator) validateAndPopulateMailingList(ctx context.Context, member *model.GrpsIOMember) error {
	if member.MailingListUID == "" {
		return errs.NewValidation("mailing_list_uid is required")
	}

	// Validate mailing list exists
	_, _, err := o.grpsIOReader.GetGrpsIOMailingList(ctx, member.MailingListUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve mailing list",
			"error", err,
			"mailing_list_uid", member.MailingListUID,
		)
		return errs.NewNotFound("mailing list not found")
	}

	return nil
}

// publishMemberMessages publishes indexer and access control messages for member operations
func (o *grpsIOWriterOrchestrator) publishMemberMessages(ctx context.Context, member *model.GrpsIOMember, action model.MessageAction) error {
	if o.publisher == nil {
		slog.WarnContext(ctx, "publisher not available, skipping member message publishing")
		return nil
	}

	slog.DebugContext(ctx, "publishing messages for member",
		"action", action,
		"member_uid", member.UID)

	// Build indexer message
	indexerMessage, err := o.buildMemberIndexerMessage(ctx, member, action)
	if err != nil {
		return fmt.Errorf("failed to build %s indexer message: %w", action, err)
	}

	// Prepare messages to publish
	messages := []func() error{
		func() error {
			return o.publisher.Indexer(ctx, constants.IndexGroupsIOMemberSubject, indexerMessage)
		},
	}

	// Only publish access control message if member has a username (required for FGA identity)
	if member.Username != "" {
		accessMessage := o.buildMemberAccessMessage(member)
		messages = append(messages, func() error {
			return o.publisher.Access(ctx, constants.PutMemberGroupsIOMailingListSubject, accessMessage)
		})
	} else {
		slog.DebugContext(ctx, "skipping access control message - member has no username",
			"member_uid", member.UID)
	}

	// Execute all messages concurrently
	errPublishingMessage := concurrent.NewWorkerPool(len(messages)).Run(ctx, messages...)
	if errPublishingMessage != nil {
		slog.ErrorContext(ctx, "failed to publish member messages",
			"error", errPublishingMessage,
			"member_uid", member.UID,
		)
		return errPublishingMessage
	}

	slog.DebugContext(ctx, "messages published successfully",
		"member_uid", member.UID,
		"action", action,
	)

	return nil
}

// publishMemberDeleteMessages publishes member delete messages concurrently
func (o *grpsIOWriterOrchestrator) publishMemberDeleteMessages(ctx context.Context, uid string, member model.GrpsIOMember) error {
	if o.publisher == nil {
		slog.WarnContext(ctx, "publisher not available, skipping member delete message publishing")
		return nil
	}

	indexerMessage := &model.IndexerMessage{
		Action: model.ActionDeleted,
		Tags:   []string{},
	}

	builtMessage, err := indexerMessage.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build member delete indexer message", "error", err, "member_uid", uid)
		return fmt.Errorf("failed to build member delete indexer message: %w", err)
	}

	// Prepare messages to publish
	messages := []func() error{
		func() error {
			return o.publisher.Indexer(ctx, constants.IndexGroupsIOMemberSubject, builtMessage)
		},
	}

	// Only publish access control message if member has a username (required for FGA identity)
	if member.Username != "" {
		accessMessage := o.buildMemberAccessMessage(&member)
		messages = append(messages, func() error {
			return o.publisher.Access(ctx, constants.RemoveMemberGroupsIOMailingListSubject, accessMessage)
		})
	} else {
		slog.DebugContext(ctx, "skipping access control delete message - member has no username",
			"member_uid", uid)
	}

	// Execute all messages concurrently
	errPublishingMessage := concurrent.NewWorkerPool(len(messages)).Run(ctx, messages...)
	if errPublishingMessage != nil {
		slog.ErrorContext(ctx, "failed to publish member delete messages",
			"error", errPublishingMessage,
			"member_uid", uid,
		)
		return errPublishingMessage
	}

	slog.DebugContext(ctx, "member delete messages published successfully", "member_uid", uid)
	return nil
}

// buildMemberIndexerMessage creates the indexer message using proper IndexerMessage.Build method
func (o *grpsIOWriterOrchestrator) buildMemberIndexerMessage(ctx context.Context, member *model.GrpsIOMember, action model.MessageAction) (*model.IndexerMessage, error) {
	indexerMessage := &model.IndexerMessage{
		Action: action,
		Tags:   member.Tags(),
	}

	// Build the message with proper context and authorization headers
	return indexerMessage.Build(ctx, member)
}

// buildMemberAccessMessage creates the access control message stub for OpenFGA integration
func (o *grpsIOWriterOrchestrator) buildMemberAccessMessage(member *model.GrpsIOMember) *groupsioMailingListMemberStub {
	return &groupsioMailingListMemberStub{
		UID:            member.UID,
		Username:       member.Username,
		MailingListUID: member.MailingListUID,
	}
}

// mergeMemberData merges existing member data with updated fields, preserving immutable fields
func (o *grpsIOWriterOrchestrator) mergeMemberData(ctx context.Context, existing *model.GrpsIOMember, updated *model.GrpsIOMember) {
	// Preserve immutable fields
	updated.UID = existing.UID
	updated.CreatedAt = existing.CreatedAt
	updated.MailingListUID = existing.MailingListUID // Parent reference is immutable
	updated.Email = existing.Email                   // Email is immutable due to unique constraint

	// Update timestamp
	updated.UpdatedAt = time.Now()

	slog.DebugContext(ctx, "member data merged",
		"member_uid", existing.UID,
		"mutable_fields", []string{"status", "display_name"},
	)
}

// handleMemberCreationBySource handles member creation based on source type (API, webhook, or mock)
func (o *grpsIOWriterOrchestrator) handleMemberCreationBySource(
	ctx context.Context,
	member *model.GrpsIOMember,
	mailingList *model.GrpsIOMailingList,
) (memberID *int64, groupID *int64, requiresCleanup bool, err error) {
	if member == nil || mailingList == nil {
		return nil, nil, false, errs.NewValidation("member and mailing list are required for creation")
	}

	if member.Source == constants.SourceMock {
		// Mock source: Skip Groups.io creation for testing
		slog.InfoContext(ctx, "skipping Groups.io creation",
			"member_uid", member.UID)
		return nil, nil, false, nil
	}

	if member.Source == constants.SourceWebhook {
		// Webhook source: Skip Groups.io creation (webhook is source of truth)
		// Use pre-provided IDs from webhook event
		slog.InfoContext(ctx, "skipping Groups.io creation for webhook source",
			"member_uid", member.UID, "source", member.Source)
		return member.MemberID, member.GroupID, false, nil
	}

	// Build logger with safe group_id handling
	logger := slog.With("member_uid", member.UID, "source", member.Source)
	if mailingList.GroupID != nil {
		logger = logger.With("group_id", *mailingList.GroupID)
	}

	// API source: Create member in Groups.io via API
	// Guard: Skip if client not available or mailing list not synced
	if o.groupsClient == nil || mailingList.GroupID == nil {
		logger.InfoContext(ctx, "Groups.io client unavailable or mailing list not synced, treating as mock",
			"email", redaction.RedactEmail(member.Email))
		return nil, nil, false, nil
	}

	// Get parent service for Groups.io operations
	parentService, _, err := o.grpsIOReader.GetGrpsIOService(ctx, mailingList.ServiceUID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get parent service for Groups.io sync", "error", err, "service_uid", mailingList.ServiceUID)
		return nil, nil, false, err
	}

	logger.InfoContext(ctx, "creating member in Groups.io",
		"email", redaction.RedactEmail(member.Email))

	// Create member in Groups.io
	memberID, groupID, err = o.createMemberInGroupsIO(ctx, member, mailingList, parentService)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create member in Groups.io",
			"error", err,
			"email", redaction.RedactEmail(member.Email))
		return nil, nil, false, err
	}

	// Determine if cleanup is required for rollback
	requiresCleanup = memberID != nil && parentService.GroupID != nil

	if memberID != nil {
		logger.InfoContext(ctx, "member created successfully in Groups.io",
			"member_id", *memberID)
	}

	return memberID, groupID, requiresCleanup, nil
}

// handleMemberUpdateBySource handles member updates based on source type (API, webhook, or mock)
// Returns error if Groups.io sync fails, but does not fail the overall update operation
func (o *grpsIOWriterOrchestrator) handleMemberUpdateBySource(ctx context.Context, member *model.GrpsIOMember) error {
	if member == nil {
		return errs.NewValidation("member is required for update")
	}

	if member.Source == constants.SourceMock {
		// Mock source: Skip Groups.io sync for testing
		slog.InfoContext(ctx, "skipping Groups.io sync",
			"member_uid", member.UID, "source", member.Source)
		return nil
	}

	logger := slog.With("member_uid", member.UID, "source", member.Source)

	// API source: Sync updates to Groups.io
	// Guard: Skip if client not available or member not synced
	if o.groupsClient == nil || member.MemberID == nil {
		logger.InfoContext(ctx, "Groups.io integration disabled or member not synced - skipping Groups.io update")
		return nil
	}
	logger = logger.With("member_id", *member.MemberID)

	// Get domain using helper method through member lookup chain
	domain, err := o.getGroupsIODomainForResource(ctx, member.UID, constants.ResourceTypeMember)
	if err != nil {
		logger.WarnContext(ctx, "Groups.io member sync skipped due to domain lookup failure",
			"error", err)
		return err
	}
	logger = logger.With("domain", domain)

	groupsioModStatus := ""
	switch member.ModStatus {
	case "moderator":
		groupsioModStatus = "sub_modstatus_moderator"
	case "owner":
		groupsioModStatus = "sub_modstatus_owner"
	case "none":
		groupsioModStatus = "sub_modstatus_none"
	default:
		return errs.NewValidation(fmt.Sprintf("invalid mod status: %s", member.ModStatus))
	}

	groupsioDeliveryMode := ""
	switch member.DeliveryMode {
	case "normal":
		groupsioDeliveryMode = "email_delivery_single"
	case "digest":
		groupsioDeliveryMode = "email_delivery_digest"
	case "none":
		groupsioDeliveryMode = "email_delivery_none"
	default:
		return errs.NewValidation(fmt.Sprintf("invalid delivery mode: %s", member.DeliveryMode))
	}

	memberUpdates := groupsio.MemberUpdateOptions{
		FullName:     fmt.Sprintf("%s %s", member.FirstName, member.LastName),
		ModStatus:    groupsioModStatus,
		DeliveryMode: groupsioDeliveryMode,
	}

	// Perform Groups.io member update
	err = o.groupsClient.UpdateMember(ctx, domain, utils.Int64PtrToUint64(member.MemberID), memberUpdates)
	if err != nil {
		logger.WarnContext(ctx, "Groups.io member update failed",
			"error", err)
		return err
	}

	logger.InfoContext(ctx, "Groups.io member updated successfully")
	return nil
}

// handleMemberDeletionBySource handles member deletion based on source type (API, webhook, or mock)
// Returns error if Groups.io deletion fails
func (o *grpsIOWriterOrchestrator) handleMemberDeletionBySource(ctx context.Context, member *model.GrpsIOMember) error {
	if member == nil {
		return errs.NewValidation("member is required for deletion")
	}

	if member.Source == constants.SourceMock {
		// Mock source: Skip Groups.io deletion for testing
		slog.InfoContext(ctx, "skipping Groups.io deletion",
			"member_uid", member.UID)
		return nil
	}

	logger := slog.With("member_uid", member.UID, "source", member.Source)
	if member.GroupID != nil {
		logger = logger.With("group_id", *member.GroupID)
	}
	if member.MemberID != nil {
		logger = logger.With("member_id", *member.MemberID)
	}

	logger.InfoContext(ctx, "removing member from Groups.io")
	return o.removeMemberFromGroupsIO(ctx, member)
}

// updateMailingListSubscriberCount refreshes the subscriber count from Groups.io and falls back to NATS member item count,
// with retry logic for concurrent updates (max 3 attempts)
// This is a best-effort operation - failures are logged but don't fail the member operation
func (o *grpsIOWriterOrchestrator) updateMailingListSubscriberCount(
	ctx context.Context,
	mailingListUID string,
) {
	const maxRetries = 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Read current mailing list with revision
		mailingList, revision, err := o.grpsIOReader.GetGrpsIOMailingList(ctx, mailingListUID)
		if err != nil {
			slog.WarnContext(ctx, "failed to read mailing list for subscriber count update",
				"error", err, "mailing_list_uid", mailingListUID, "attempt", attempt)
			return
		}

		// Fetch fresh count from Groups.io (or NATS fallback) instead of incrementing
		oldCount := mailingList.SubscriberCount
		newCount := o.refreshSubscriberCount(ctx, mailingList)

		// Update subscriber count
		mailingList.SubscriberCount = newCount

		// Update with revision check (optimistic concurrency control)
		_, _, err = o.grpsIOWriter.UpdateGrpsIOMailingList(ctx, mailingListUID, mailingList, revision)
		if err != nil {
			var conflictErr errs.Conflict
			if stdErrors.As(err, &conflictErr) && attempt < maxRetries {
				slog.InfoContext(ctx, "concurrent update detected for subscriber count, retrying",
					"mailing_list_uid", mailingListUID, "attempt", attempt)
				continue
			}
			slog.WarnContext(ctx, "failed to update subscriber count after retries",
				"error", err, "mailing_list_uid", mailingListUID, "attempt", attempt)
			return
		}

		slog.InfoContext(ctx, "subscriber count updated successfully",
			"mailing_list_uid", mailingListUID, "old_count", oldCount, "new_count", newCount)

		if o.publisher != nil {
			// Publish indexer message with updated subscriber count (best-effort)
			indexerMessage := &model.IndexerMessage{
				Action: model.ActionUpdated,
				Tags:   mailingList.Tags(),
			}

			builtMessage, err := indexerMessage.Build(ctx, mailingList)
			if err != nil {
				slog.WarnContext(ctx, "failed to build indexer message for subscriber count update",
					"error", err, "mailing_list_uid", mailingListUID)
				return
			}
			if err := o.publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, builtMessage); err != nil {
				slog.WarnContext(ctx, "failed to publish indexer message for subscriber count update",
					"error", err, "mailing_list_uid", mailingListUID)
				// Don't fail - the count update succeeded, indexer message is best-effort
			}
		}

		return
	}
}
