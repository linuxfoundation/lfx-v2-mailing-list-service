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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// CreateGrpsIOMailingList creates a new mailing list with comprehensive validation and messaging
func (ml *grpsIOWriterOrchestrator) CreateGrpsIOMailingList(ctx context.Context, request *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "orchestrator: creating mailing list",
		"group_name", request.GroupName,
		"parent_uid", request.ServiceUID,
		"committee_uid", request.CommitteeUID)

	// For rollback purposes
	var (
		keys             []string
		rollbackRequired bool
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			ml.deleteKeys(ctx, keys, true)
		}
	}()

	// Step 1: Generate UID and set timestamps
	request.UID = uuid.New().String()
	now := time.Now()
	request.CreatedAt = now
	request.UpdatedAt = now

	// Step 2: Validate basic fields
	if err := request.ValidateBasicFields(); err != nil {
		slog.WarnContext(ctx, "basic field validation failed", "error", err)
		return nil, err
	}

	// Step 3: Validate committee fields
	if err := request.ValidateCommitteeFields(); err != nil {
		slog.WarnContext(ctx, "committee field validation failed", "error", err)
		return nil, err
	}

	// Step 4: Validate parent service and inherit metadata
	parentService, err := ml.validateAndInheritFromParent(ctx, request)
	if err != nil {
		return nil, err
	}

	// Step 5: Validate committee and populate metadata (if specified)
	if err := ml.validateAndPopulateCommittee(ctx, request, parentService.ProjectUID); err != nil {
		return nil, err
	}

	// Step 6: Validate group name prefix for non-primary services
	if err := request.ValidateGroupNamePrefix(parentService.Type, parentService.Prefix); err != nil {
		slog.WarnContext(ctx, "group name prefix validation failed", "error", err)
		return nil, err
	}

	// Step 7: Reserve unique constraints (like service pattern)
	constraintKey, err := ml.reserveMailingListConstraints(ctx, request)
	if err != nil {
		rollbackRequired = true
		return nil, err
	}
	if constraintKey != "" {
		keys = append(keys, constraintKey)
	}

	// Step 8: Create mailing list in storage
	createdMailingList, _, err := ml.grpsIOWriter.CreateGrpsIOMailingList(ctx, request)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list in storage", "error", err)
		rollbackRequired = true
		return nil, err
	}
	keys = append(keys, createdMailingList.UID)

	// Step 8.1: Create secondary indices for the mailing list
	secondaryKeys, err := ml.createMailingListSecondaryIndices(ctx, createdMailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list secondary indices", "error", err)
		rollbackRequired = true
		return nil, err
	}
	// Add secondary keys to rollback list
	keys = append(keys, secondaryKeys...)

	// Step 9: Publish messages concurrently (indexer + access control)
	if err := ml.publishMailingListMessages(ctx, createdMailingList); err != nil {
		// Log warning but don't fail the operation - mailing list is already created
		slog.WarnContext(ctx, "failed to publish messages", "error", err, "mailing_list_uid", createdMailingList.UID)
	}

	slog.InfoContext(ctx, "mailing list created successfully",
		"mailing_list_uid", createdMailingList.UID,
		"group_name", createdMailingList.GroupName,
		"parent_uid", createdMailingList.ServiceUID,
		"public", createdMailingList.Public,
		"committee_based", createdMailingList.IsCommitteeBased())

	return createdMailingList, nil
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

// publishMailingListMessages publishes indexer and access control messages concurrently
func (ml *grpsIOWriterOrchestrator) publishMailingListMessages(ctx context.Context, mailingList *model.GrpsIOMailingList) error {
	slog.DebugContext(ctx, "publishing messages for mailing list",
		"mailing_list_uid", mailingList.UID)

	// Build indexer message
	indexerMessage, err := ml.buildMailingListIndexerMessage(ctx, mailingList)
	if err != nil {
		return fmt.Errorf("failed to build indexer message: %w", err)
	}

	// Build access control message
	accessMessage := ml.buildMailingListAccessControlMessage(mailingList)

	// Publish indexer message
	if err := ml.publisher.Indexer(ctx, constants.IndexGroupsIOMailingListSubject, indexerMessage); err != nil {
		slog.ErrorContext(ctx, "failed to publish indexer message", "error", err)
		return fmt.Errorf("failed to publish indexer message: %w", err)
	}

	// Publish access control message
	if err := ml.publisher.Access(ctx, constants.UpdateAccessGroupsIOMailingListSubject, accessMessage); err != nil {
		slog.ErrorContext(ctx, "failed to publish access control message", "error", err)
		return fmt.Errorf("failed to publish access control message: %w", err)
	}

	slog.DebugContext(ctx, "messages published successfully",
		"mailing_list_uid", mailingList.UID,
		"indexer_published", true,
		"access_control_published", true)

	return nil
}

// buildMailingListIndexerMessage builds an indexer message for search capabilities
func (ml *grpsIOWriterOrchestrator) buildMailingListIndexerMessage(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.IndexerMessage, error) {
	indexerMessage := &model.IndexerMessage{
		Action: model.ActionCreated,
		Tags:   mailingList.Tags(),
	}

	// Build the message with proper context and authorization headers
	return indexerMessage.Build(ctx, mailingList)
}

// buildMailingListAccessControlMessage builds an access control message for OpenFGA
func (ml *grpsIOWriterOrchestrator) buildMailingListAccessControlMessage(mailingList *model.GrpsIOMailingList) *model.AccessMessage {
	references := map[string]string{
		constants.RelationProject: mailingList.ProjectUID, // Required for project inheritance
		constants.RelationService: mailingList.ServiceUID, // Required for service-level permission inheritance
	}

	// Add committee reference for committee-based lists (enables committee-level authorization)
	if mailingList.CommitteeUID != "" {
		references[constants.RelationCommittee] = mailingList.CommitteeUID
	}

	return &model.AccessMessage{
		UID:        mailingList.UID,
		ObjectType: "groupsio_mailing_list",
		Public:     mailingList.Public,    // Using Public bool instead of Visibility
		Relations:  map[string][]string{}, // Reserved for future use
		References: references,
	}
}

// UpdateGrpsIOMailingList updates an existing mailing list (TODO: implement in future PR)
func (ml *grpsIOWriterOrchestrator) UpdateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	// TODO: Implement in future PR for PUT endpoint
	return nil, errors.NewServiceUnavailable("update mailing list not implemented yet")
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

// DeleteGrpsIOMailingList deletes a mailing list (TODO: implement in future PR)
func (ml *grpsIOWriterOrchestrator) DeleteGrpsIOMailingList(ctx context.Context, uid string) error {
	// TODO: Implement in future PR for DELETE endpoint
	return errors.NewServiceUnavailable("delete mailing list not implemented yet")
}
