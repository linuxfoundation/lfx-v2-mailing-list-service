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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// GrpsIOMailingListWriterOrchestrator orchestrates mailing list creation with comprehensive validation
type GrpsIOMailingListWriterOrchestrator struct {
	storage          port.GrpsIOReaderWriter
	entityReader     port.EntityAttributeReader
	messagePublisher port.MessagePublisher
}

// NewGrpsIOMailingListWriterOrchestrator creates a new mailing list writer orchestrator
func NewGrpsIOMailingListWriterOrchestrator(
	storage port.GrpsIOReaderWriter,
	entityReader port.EntityAttributeReader,
	messagePublisher port.MessagePublisher,
) *GrpsIOMailingListWriterOrchestrator {
	return &GrpsIOMailingListWriterOrchestrator{
		storage:          storage,
		entityReader:     entityReader,
		messagePublisher: messagePublisher,
	}
}

// CreateGrpsIOMailingList creates a new mailing list with comprehensive validation and messaging
func (ml *GrpsIOMailingListWriterOrchestrator) CreateGrpsIOMailingList(ctx context.Context, request *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "orchestrator: creating mailing list",
		"group_name", request.GroupName,
		"parent_uid", request.ParentUID,
		"committee_uid", request.CommitteeUID)

	// For rollback purposes
	var (
		keys             []string
		rollbackRequired bool
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			ml.deleteMailingListKeys(ctx, keys, true)
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
	createdMailingList, err := ml.storage.CreateGrpsIOMailingList(ctx, request)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list in storage", "error", err)
		rollbackRequired = true
		return nil, err
	}
	keys = append(keys, createdMailingList.UID)

	// Step 9: Publish messages concurrently (indexer + access control)
	if err := ml.publishMessages(ctx, createdMailingList); err != nil {
		// Log warning but don't fail the operation - mailing list is already created
		slog.WarnContext(ctx, "failed to publish messages", "error", err, "mailing_list_uid", createdMailingList.UID)
	}

	slog.InfoContext(ctx, "mailing list created successfully",
		"mailing_list_uid", createdMailingList.UID,
		"group_name", createdMailingList.GroupName,
		"parent_uid", createdMailingList.ParentUID,
		"public", createdMailingList.Public,
		"committee_based", createdMailingList.IsCommitteeBased())

	return createdMailingList, nil
}

// validateAndInheritFromParent validates parent service exists and inherits metadata
func (ml *GrpsIOMailingListWriterOrchestrator) validateAndInheritFromParent(ctx context.Context, request *model.GrpsIOMailingList) (*model.GrpsIOService, error) {
	slog.DebugContext(ctx, "validating parent service", "parent_uid", request.ParentUID)

	// Get parent service from storage
	parentService, _, err := ml.storage.GetGrpsIOService(ctx, request.ParentUID)
	if err != nil {
		slog.WarnContext(ctx, "parent service validation failed", "parent_uid", request.ParentUID, "error", err)
		return nil, errors.NewNotFound("parent service not found")
	}

	// Inherit project UID from parent service
	request.ProjectUID = parentService.ProjectUID

	slog.DebugContext(ctx, "parent service validated successfully",
		"parent_uid", request.ParentUID,
		"parent_type", parentService.Type,
		"project_uid", parentService.ProjectUID,
		"prefix", parentService.Prefix)

	return parentService, nil
}

// validateAndPopulateCommittee validates committee exists and populates committee metadata
func (ml *GrpsIOMailingListWriterOrchestrator) validateAndPopulateCommittee(ctx context.Context, request *model.GrpsIOMailingList, projectID string) error {
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
func (ml *GrpsIOMailingListWriterOrchestrator) reserveMailingListConstraints(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	// For mailing lists, we have one constraint type: unique group name within parent service
	return ml.storage.UniqueMailingListGroupName(ctx, mailingList)
}

// publishMessages publishes indexer and access control messages concurrently
func (ml *GrpsIOMailingListWriterOrchestrator) publishMessages(ctx context.Context, mailingList *model.GrpsIOMailingList) error {
	slog.DebugContext(ctx, "publishing messages for mailing list",
		"mailing_list_uid", mailingList.UID)

	// Build indexer message
	indexerMessage, err := ml.buildIndexerMessage(ctx, mailingList)
	if err != nil {
		return fmt.Errorf("failed to build indexer message: %w", err)
	}

	// Build access control message
	accessMessage := ml.buildAccessControlMessage(mailingList)

	// Publish indexer message
	if err := ml.messagePublisher.Indexer(ctx, "mailing-list.created", indexerMessage); err != nil {
		slog.ErrorContext(ctx, "failed to publish indexer message", "error", err)
		return fmt.Errorf("failed to publish indexer message: %w", err)
	}

	// Publish access control message
	if err := ml.messagePublisher.Access(ctx, "mailing-list.access", accessMessage); err != nil {
		slog.ErrorContext(ctx, "failed to publish access control message", "error", err)
		return fmt.Errorf("failed to publish access control message: %w", err)
	}

	slog.DebugContext(ctx, "messages published successfully",
		"mailing_list_uid", mailingList.UID,
		"indexer_published", true,
		"access_control_published", true)

	return nil
}

// buildIndexerMessage builds an indexer message for search capabilities
func (ml *GrpsIOMailingListWriterOrchestrator) buildIndexerMessage(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.IndexerMessage, error) {
	indexerMessage := &model.IndexerMessage{
		Action: model.ActionCreated,
		Tags:   ml.buildMessageTags(mailingList),
	}

	// Build the message with proper context and authorization headers
	return indexerMessage.Build(ctx, mailingList)
}

// buildAccessControlMessage builds an access control message for OpenFGA
func (ml *GrpsIOMailingListWriterOrchestrator) buildAccessControlMessage(mailingList *model.GrpsIOMailingList) *model.AccessMessage {
	references := map[string]string{
		constants.RelationProject: mailingList.ProjectUID, // Required for project inheritance
	}

	// Add committee reference for committee-based lists (enables committee-level authorization)
	if mailingList.CommitteeUID != "" {
		references[constants.RelationCommittee] = mailingList.CommitteeUID
	}

	return &model.AccessMessage{
		UID:        mailingList.UID,
		ObjectType: mailingList.GetAccessControlObjectType(),
		Public:     mailingList.Public,    // Using Public bool instead of Visibility
		Relations:  map[string][]string{}, // Reserved for future use
		References: references,
	}
}

// buildMessageTags builds tags for indexer message to enable faceted search
func (ml *GrpsIOMailingListWriterOrchestrator) buildMessageTags(mailingList *model.GrpsIOMailingList) []string {
	tags := []string{
		fmt.Sprintf("project_uid:%s", mailingList.ProjectUID),
		fmt.Sprintf("parent_uid:%s", mailingList.ParentUID),
		fmt.Sprintf("type:%s", mailingList.Type),
		fmt.Sprintf("public:%t", mailingList.Public),
	}

	// Add committee tag if committee-based
	if mailingList.CommitteeUID != "" {
		tags = append(tags, fmt.Sprintf("committee:%s", mailingList.CommitteeUID))
	}

	// Add committee filter tags
	for _, filter := range mailingList.CommitteeFilters {
		tags = append(tags, fmt.Sprintf("committee_filter:%s", filter))
	}

	return tags
}

// deleteMailingListKeys removes keys by getting their revision and deleting them
// This is used both for rollback scenarios and cleanup of mailing lists
func (ml *GrpsIOMailingListWriterOrchestrator) deleteMailingListKeys(ctx context.Context, keys []string, isRollback bool) {
	if len(keys) == 0 {
		return
	}

	slog.DebugContext(ctx, "deleting mailing list keys",
		"keys", keys,
		"is_rollback", isRollback,
	)

	for _, key := range keys {
		// Get revision first, then delete (same pattern as service)
		var rev uint64
		var errGet error

		// Check if this is a mailing list UID or a constraint key
		if ml.isMailingListUID(key) {
			// This is a mailing list UID - delete the mailing list directly
			err := ml.storage.DeleteGrpsIOMailingList(ctx, key)
			if err != nil {
				slog.ErrorContext(ctx, "failed to delete mailing list",
					"key", key,
					"error", err,
					"is_rollback", isRollback,
				)
			} else {
				slog.DebugContext(ctx, "successfully deleted mailing list",
					"mailing_list_uid", key,
					"is_rollback", isRollback,
				)
			}
		} else {
			// This is a constraint key - get revision and delete
			rev, errGet = ml.storage.GetKeyRevision(ctx, key)
			if errGet != nil {
				slog.ErrorContext(ctx, "failed to get revision for key deletion",
					"key", key,
					"error", errGet,
					"is_rollback", isRollback,
				)
				continue
			}

			err := ml.storage.Delete(ctx, key, rev)
			if err != nil {
				slog.ErrorContext(ctx, "failed to delete constraint key",
					"key", key,
					"error", err,
					"is_rollback", isRollback,
				)
			} else {
				slog.DebugContext(ctx, "successfully deleted constraint key",
					"constraint_key", key,
					"is_rollback", isRollback,
				)
			}
		}
	}

	slog.DebugContext(ctx, "mailing list key deletion completed",
		"keys_count", len(keys),
		"is_rollback", isRollback,
	)
}

// isMailingListUID checks if a key is a mailing list UID (UUIDs) vs constraint key (prefixed)
func (ml *GrpsIOMailingListWriterOrchestrator) isMailingListUID(key string) bool {
	// Constraint keys start with "lookup:" prefix, UIDs are UUIDs with dashes
	return len(key) == 36 && key[8] == '-' && key[13] == '-' && key[18] == '-' && key[23] == '-'
}

// UpdateGrpsIOMailingList updates an existing mailing list (TODO: implement in future PR)
func (ml *GrpsIOMailingListWriterOrchestrator) UpdateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	// TODO: Implement in future PR for PUT endpoint
	return nil, errors.NewServiceUnavailable("update mailing list not implemented yet")
}

// DeleteGrpsIOMailingList deletes a mailing list (TODO: implement in future PR)
func (ml *GrpsIOMailingListWriterOrchestrator) DeleteGrpsIOMailingList(ctx context.Context, uid string) error {
	// TODO: Implement in future PR for DELETE endpoint
	return errors.NewServiceUnavailable("delete mailing list not implemented yet")
}
