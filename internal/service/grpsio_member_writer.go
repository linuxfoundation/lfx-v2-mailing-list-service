// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
)

// CreateGrpsIOMember creates a new member with transactional operations and rollback following service pattern
func (o *grpsIOWriterOrchestrator) CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "executing create member use case",
		"member_email", member.Email,
		"mailing_list_uid", member.MailingListUID,
	)

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
				if deleteErr := o.groupsClient.RemoveMember(ctx, rollbackGroupsIODomain, *rollbackMemberID); deleteErr != nil {
					slog.WarnContext(ctx, "failed to cleanup GroupsIO member during rollback", "error", deleteErr, "member_id", *rollbackMemberID)
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

	// Step 6: Create in Groups.io FIRST (if enabled)
	mailingList, _, err := o.grpsIOReader.GetGrpsIOMailingList(ctx, member.MailingListUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list for Groups.io sync", "error", err, "mailing_list_uid", member.MailingListUID)
		rollbackRequired = true
		return nil, 0, err
	}

	if o.groupsClient != nil && mailingList.SubgroupID != nil {
		// Get parent service domain
		parentService, _, err := o.grpsIOReader.GetGrpsIOService(ctx, mailingList.ServiceUID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get parent service for Groups.io sync", "error", err, "service_uid", mailingList.ServiceUID)
			rollbackRequired = true
			return nil, 0, err
		}

		memberID, groupID, err := o.createMemberInGroupsIO(ctx, member, mailingList, parentService)
		if err != nil {
			rollbackRequired = true
			return nil, 0, err
		}

		// Groups.io creation successful - track for rollback cleanup
		if memberID != nil {
			rollbackMemberID = convertInt64PtrToUint64Ptr(memberID)
			rollbackGroupsIODomain = parentService.Domain
		}

		// Set Groups.io IDs on member before storage creation
		member.GroupsIOMemberID = memberID
		member.GroupsIOGroupID = groupID
		member.SyncStatus = "synced"
	} else {
		// Mock/disabled mode or mailing list not synced - set appropriate status
		member.SyncStatus = "pending"
		slog.InfoContext(ctx, "Groups.io integration disabled or mailing list not synced - member will be in pending state")
	}

	// Step 7: Create member in storage (with Groups.io IDs already set)
	createdMember, revision, err := o.grpsIOWriter.CreateGrpsIOMember(ctx, member)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create member",
			"error", err,
			"member_email", member.Email,
			"mailing_list_uid", member.MailingListUID,
		)
		rollbackRequired = true
		return nil, 0, err
	}

	slog.DebugContext(ctx, "member created successfully",
		"member_uid", createdMember.UID,
		"revision", revision,
	)

	// Step 8: Publish messages (indexer and access control)
	if o.publisher != nil {
		if err := o.publishMemberMessages(ctx, createdMember, model.ActionCreated); err != nil {
			slog.ErrorContext(ctx, "failed to publish member messages", "error", err)
			// Don't fail the operation on message failure, member creation succeeded
		}
	}

	return createdMember, revision, nil
}

// createMemberInGroupsIO handles Groups.io member creation and returns the IDs
func (o *grpsIOWriterOrchestrator) createMemberInGroupsIO(ctx context.Context, member *model.GrpsIOMember, mailingList *model.GrpsIOMailingList, parentService *model.GrpsIOService) (*int64, *int64, error) {
	if o.groupsClient == nil || mailingList.SubgroupID == nil {
		return nil, nil, nil // Skip Groups.io creation
	}

	slog.InfoContext(ctx, "creating member in Groups.io",
		"domain", parentService.Domain,
		"subgroup_id", *mailingList.SubgroupID,
		"email", member.Email,
	)

	memberResult, err := o.groupsClient.AddMember(
		ctx,
		parentService.Domain,
		uint64(*mailingList.SubgroupID),
		member.Email,
		fmt.Sprintf("%s %s", member.FirstName, member.LastName),
	)
	if err != nil {
		slog.ErrorContext(ctx, "Groups.io member creation failed",
			"error", err,
			"domain", parentService.Domain,
			"subgroup_id", *mailingList.SubgroupID,
			"email", member.Email,
		)
		return nil, nil, fmt.Errorf("groups.io member creation failed: %w", err)
	}

	memberID := int64(memberResult.ID)
	slog.InfoContext(ctx, "Groups.io member created successfully",
		"member_id", memberResult.ID,
		"domain", parentService.Domain,
		"subgroup_id", *mailingList.SubgroupID,
	)

	return &memberID, mailingList.SubgroupID, nil
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

	// Step 6.1: Sync member updates to Groups.io (if client available and member has GroupsIOMemberID)
	memberUpdates := groupsio.MemberUpdateOptions{
		FirstName: updatedMember.FirstName,
		LastName:  updatedMember.LastName,
		// Note: Email cannot be updated in Groups.io API
		// ModStatus and other settings can be added here as needed
	}
	o.syncMemberToGroupsIO(ctx, updatedMember, memberUpdates)

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

	if member != nil {
		slog.DebugContext(ctx, "member data provided for deletion",
			"member_uid", member.UID,
			"email", redaction.RedactEmail(member.Email),
			"mailing_list_uid", member.MailingListUID,
		)
	} else {
		slog.DebugContext(ctx, "no member data provided for deletion - will rely on storage layer for validation")
	}

	// Step 1: Remove member from Groups.io (if client available and member has GroupsIOMemberID)
	o.removeMemberFromGroupsIO(ctx, member)

	// Delete member from storage with optimistic concurrency control
	err := o.grpsIOWriter.DeleteGrpsIOMember(ctx, uid, expectedRevision, member)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete member",
			"error", err,
			"member_uid", uid,
		)
		return err
	}

	slog.DebugContext(ctx, "member deleted successfully",
		"member_uid", uid,
	)

	// Publish delete messages (indexer and access control)
	if o.publisher != nil {
		if err := o.publishMemberDeleteMessages(ctx, uid); err != nil {
			slog.ErrorContext(ctx, "failed to publish member delete messages", "error", err)
			// Don't fail the operation on message failure, delete succeeded
		}
	}

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

	// TODO: LFXV2-459 - Review and implement member access control logic for OpenFGA integration
	// Access control message building and publishing will be implemented after research is complete

	// Publish messages concurrently (only indexer for now)
	messages := []func() error{
		func() error {
			return o.publisher.Indexer(ctx, constants.IndexGroupsIOMemberSubject, indexerMessage)
		},
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

// publishMemberDeleteMessages publishes member delete messages concurrently (for future use)
// nolint:unused // Reserved for future member deletion functionality
func (o *grpsIOWriterOrchestrator) publishMemberDeleteMessages(ctx context.Context, uid string) error {
	if o.publisher == nil {
		slog.WarnContext(ctx, "publisher not available, skipping member delete message publishing")
		return nil
	}

	// For delete messages, we just need the UID
	indexerMessage := &model.IndexerMessage{
		Action: model.ActionDeleted,
		Tags:   []string{},
	}

	builtMessage, err := indexerMessage.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build member delete indexer message", "error", err, "member_uid", uid)
		return fmt.Errorf("failed to build member delete indexer message: %w", err)
	}

	// Publish delete messages concurrently
	messages := []func() error{
		func() error {
			return o.publisher.Indexer(ctx, constants.IndexGroupsIOMemberSubject, builtMessage)
		},
		// TODO: LFXV2-459 Implement proper member removal from mailing list relations
		// Currently commented out to avoid deleting entire mailing list from OpenFGA
		// func() error {
		//	return o.publisher.Access(ctx, constants.DeleteAllAccessGroupsIOMemberSubject, uid)
		// },
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

// convertInt64PtrToUint64Ptr safely converts *int64 to *uint64, following the existing helper pattern
func convertInt64PtrToUint64Ptr(val *int64) *uint64 {
	if val == nil {
		return nil
	}
	converted := uint64(*val)
	return &converted
}

// syncMemberToGroupsIO handles Groups.io member update synchronization with proper error handling
func (o *grpsIOWriterOrchestrator) syncMemberToGroupsIO(ctx context.Context, member *model.GrpsIOMember, updates groupsio.MemberUpdateOptions) {
	// Guard clause: skip if Groups.io client not available or member not synced
	if o.groupsClient == nil || member.GroupsIOMemberID == nil {
		slog.InfoContext(ctx, "Groups.io integration disabled or member not synced - skipping Groups.io update")
		return
	}

	// Get domain using helper method through member lookup chain
	domain, err := o.getGroupsIODomainForResource(ctx, member.UID, constants.ResourceTypeMember)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io member sync skipped due to domain lookup failure, local update will proceed",
			"error", err, "member_uid", member.UID)
		return
	}

	// Perform Groups.io member update
	err = o.groupsClient.UpdateMember(ctx, domain, uint64(*member.GroupsIOMemberID), updates)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io member update failed, local update will proceed",
			"error", err, "domain", domain, "member_id", *member.GroupsIOMemberID)
	} else {
		slog.InfoContext(ctx, "Groups.io member updated successfully",
			"member_id", *member.GroupsIOMemberID, "domain", domain)
	}
}
