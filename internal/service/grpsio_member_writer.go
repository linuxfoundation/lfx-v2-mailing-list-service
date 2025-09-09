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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// CreateGrpsIOMember creates a new member with transactional operations and rollback following service pattern
func (o *grpsIOWriterOrchestrator) CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "executing create member use case",
		"member_email", member.Email,
		"mailing_list_uid", member.MailingListUID,
	)

	// Step 1: Generate UID and set timestamps (like mailing list pattern)
	now := time.Now()
	if member.UID == "" {
		member.UID = uuid.New().String()
	}
	member.CreatedAt = now
	member.UpdatedAt = now

	// For rollback purposes
	var (
		keys             []string
		rollbackRequired bool
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			o.deleteKeys(ctx, keys, true)
		}
	}()

	// Step 2: Validate mailing list exists and populate metadata
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

	// Step 3: Set default status if not provided
	if member.Status == "" {
		member.Status = "pending"
	}

	// Step 4: Reserve unique constraints (member email per mailing list)
	constraintKey, err := o.grpsIOWriter.UniqueMember(ctx, member)
	if err != nil {
		rollbackRequired = true
		return nil, 0, err
	}
	if constraintKey != "" {
		keys = append(keys, constraintKey)
	}

	// Step 5: Create member in storage
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
	keys = append(keys, createdMember.UID)

	slog.DebugContext(ctx, "member created successfully",
		"member_uid", createdMember.UID,
		"revision", revision,
	)

	// Step 6: Publish messages (indexer and access control)
	if o.publisher != nil {
		if err := o.publishMemberMessages(ctx, createdMember, model.ActionCreated); err != nil {
			slog.ErrorContext(ctx, "failed to publish member messages", "error", err)
			// Don't rollback on message failure, member creation succeeded
		}
	}

	return createdMember, revision, nil
}

// UpdateGrpsIOMember updates an existing member (stub for future implementation)
func (o *grpsIOWriterOrchestrator) UpdateGrpsIOMember(ctx context.Context, uid string, member *model.GrpsIOMember, expectedRevision uint64) (*model.GrpsIOMember, uint64, error) {
	// TODO: Implement in future PR following service pattern
	return nil, 0, errs.NewUnexpected("UpdateGrpsIOMember not implemented")
}

// DeleteGrpsIOMember deletes a member (stub for future implementation)
func (o *grpsIOWriterOrchestrator) DeleteGrpsIOMember(ctx context.Context, uid string, expectedRevision uint64) error {
	// TODO: Implement in future PR following service pattern
	return errs.NewUnexpected("DeleteGrpsIOMember not implemented")
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

	// Build access control message
	accessMessage := o.buildMemberAccessControlMessage(member)

	// Publish messages concurrently
	messages := []func() error{
		func() error {
			return o.publisher.Indexer(ctx, constants.IndexGroupsIOMemberSubject, indexerMessage)
		},
		func() error {
			return o.publisher.Access(ctx, constants.UpdateAccessGroupsIOMemberSubject, accessMessage)
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
		func() error {
			return o.publisher.Access(ctx, constants.DeleteAllAccessGroupsIOMemberSubject, uid)
		},
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

// buildMemberAccessControlMessage creates the access control message for OpenFGA
func (o *grpsIOWriterOrchestrator) buildMemberAccessControlMessage(member *model.GrpsIOMember) *model.AccessMessage {
	references := map[string]string{
		constants.RelationMailingList: member.MailingListUID, // Required for mailing list-level permission inheritance
	}

	return &model.AccessMessage{
		UID:        member.UID,
		ObjectType: "groupsio_member",
		Public:     false,                 // Members are typically not public
		Relations:  map[string][]string{}, // Reserved for future use
		References: references,
	}
}
