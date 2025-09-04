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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// CreateGrpsIOService creates a new service with transactional operations and rollback
func (sw *grpsIOWriterOrchestrator) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "executing create service use case",
		"service_type", service.Type,
		"service_domain", service.Domain,
		"project_uid", service.ProjectUID,
	)

	// Set service identifiers and timestamps
	now := time.Now()
	if service.UID == "" {
		service.UID = uuid.New().String()
	}
	service.CreatedAt = now
	service.UpdatedAt = now

	// For rollback purposes
	var (
		keys             []string
		rollbackRequired bool
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			sw.deleteKeys(ctx, keys, true)
		}
	}()

	// Step 1: Validate project exists and populate metadata
	if err := sw.validateAndPopulateProject(ctx, service); err != nil {
		slog.ErrorContext(ctx, "project validation failed",
			"error", err,
			"project_uid", service.ProjectUID,
		)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "project validation successful",
		"project_uid", service.ProjectUID,
		"project_slug", service.ProjectSlug,
		"project_name", service.ProjectName,
	)

	// Step 2: Reserve unique constraints based on service type
	constraintKey, err := sw.reserveUniqueConstraints(ctx, service)
	if err != nil {
		rollbackRequired = true
		return nil, 0, err
	}
	if constraintKey != "" {
		keys = append(keys, constraintKey)
	}

	// Step 3: Create service in storage
	createdService, revision, err := sw.grpsIOWriter.CreateGrpsIOService(ctx, service)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create service",
			"error", err,
			"service_type", service.Type,
			"service_domain", service.Domain,
		)
		rollbackRequired = true
		return nil, 0, err
	}
	keys = append(keys, createdService.UID)

	slog.DebugContext(ctx, "service created successfully",
		"service_uid", createdService.UID,
		"revision", revision,
	)

	// Step 4: Publish messages (indexer and access control)
	if sw.publisher != nil {
		if err := sw.publishServiceMessages(ctx, createdService, model.ActionCreated); err != nil {
			slog.ErrorContext(ctx, "failed to publish messages", "error", err)
			// Don't rollback on message failure, service creation succeeded
		}
	}

	return createdService, revision, nil
}

// UpdateGrpsIOService updates an existing service with transactional patterns
func (sw *grpsIOWriterOrchestrator) UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "executing update service use case",
		"service_uid", uid,
		"expected_revision", expectedRevision,
		"project_uid", service.ProjectUID,
	)

	// For rollback purposes and cleanup
	var (
		staleKeys        []string
		newKeys          []string
		rollbackRequired bool
		updateSucceeded  bool
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			// Rollback new keys
			sw.deleteKeys(ctx, newKeys, true)
		}
		if updateSucceeded && len(staleKeys) > 0 {
			slog.DebugContext(ctx, "cleaning up stale keys",
				"keys_count", len(staleKeys),
			)
			go func() {
				// Cleanup stale keys in a separate goroutine
				// Use WithoutCancel to inherit values (tracing, auth) but not cancellation from parent request
				// This ensures cleanup completes even if original request times out
				ctxCleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second*10)
				defer cancel()

				sw.deleteKeys(ctxCleanup, staleKeys, false)
				slog.DebugContext(ctxCleanup, "stale keys cleanup completed",
					"keys_count", len(staleKeys),
				)
			}()
		}
	}()

	// Validate project exists and populate metadata
	if err := sw.validateAndPopulateProject(ctx, service); err != nil {
		slog.ErrorContext(ctx, "project validation failed during update",
			"error", err,
			"project_uid", service.ProjectUID,
			"service_uid", uid,
		)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "project validation successful for update",
		"project_uid", service.ProjectUID,
		"project_slug", service.ProjectSlug,
		"project_name", service.ProjectName,
		"service_uid", uid,
	)

	// Retrieve existing service to merge data properly
	existing, existingRevision, err := sw.grpsIOReader.GetGrpsIOService(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing service",
			"error", err,
			"service_uid", uid,
		)
		return nil, 0, err
	}

	// Verify revision matches to ensure optimistic locking
	if existingRevision != expectedRevision {
		slog.WarnContext(ctx, "revision mismatch during update",
			"expected_revision", expectedRevision,
			"current_revision", existingRevision,
			"service_uid", uid,
		)
		return nil, 0, errors.NewConflict("service has been modified by another process")
	}

	// Merge existing data with updated fields
	sw.mergeServiceData(ctx, existing, service)

	// Update service in storage
	updatedService, revision, err := sw.grpsIOWriter.UpdateGrpsIOService(ctx, uid, service, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update service",
			"error", err,
			"service_uid", uid,
			"expected_revision", expectedRevision,
		)
		rollbackRequired = true
		return nil, 0, err
	}

	slog.DebugContext(ctx, "service updated successfully",
		"service_uid", uid,
		"revision", revision,
	)

	// Publish update messages
	if sw.publisher != nil {
		if err := sw.publishServiceMessages(ctx, updatedService, model.ActionUpdated); err != nil {
			slog.ErrorContext(ctx, "failed to publish update messages", "error", err)
			// Don't fail the update on message publishing errors
		}
	}

	// Mark update as successful for defer cleanup
	updateSucceeded = true
	return updatedService, revision, nil
}

// DeleteGrpsIOService deletes a service with message publishing
func (sw *grpsIOWriterOrchestrator) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64) error {
	slog.DebugContext(ctx, "executing delete service use case",
		"service_uid", uid,
		"expected_revision", expectedRevision,
	)

	// Step 1: Retrieve existing service data to get all the information needed for cleanup
	existing, existingRevision, err := sw.grpsIOReader.GetGrpsIOService(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing service for deletion",
			"error", err,
			"service_uid", uid,
		)
		return err
	}

	// Verify revision matches to ensure optimistic locking
	if existingRevision != expectedRevision {
		slog.WarnContext(ctx, "revision mismatch during deletion",
			"expected_revision", expectedRevision,
			"current_revision", existingRevision,
			"service_uid", uid,
		)
		return errors.NewConflict("service has been modified by another process")
	}

	slog.DebugContext(ctx, "existing service retrieved for deletion",
		"service_uid", existing.UID,
		"service_type", existing.Type,
		"project_uid", existing.ProjectUID,
	)

	// Step 2: Build list of secondary indices to delete
	var indicesToDelete []string

	// Build constraint index key based on service type
	constraintIndexKey := fmt.Sprintf(constants.KVLookupGrpsIOServicePrefix, existing.BuildIndexKey(ctx))
	indicesToDelete = append(indicesToDelete, constraintIndexKey)

	slog.DebugContext(ctx, "secondary indices identified for deletion",
		"service_uid", uid,
		"indices_count", len(indicesToDelete),
		"indices", indicesToDelete,
	)

	// Step 3: Delete the main service record
	err = sw.grpsIOWriter.DeleteGrpsIOService(ctx, uid, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete service",
			"error", err,
			"service_uid", uid,
			"expected_revision", expectedRevision,
		)
		return err
	}

	slog.DebugContext(ctx, "service main record deleted successfully",
		"service_uid", uid,
	)

	// Step 4: Delete secondary indices
	// We use the deleteKeys method which handles errors gracefully and logs them
	// We don't abort here - secondary indices have a minor impact during deletion compared to the main record
	// and access control, which must be executed successfully to avoid data inconsistency
	sw.deleteKeys(ctx, indicesToDelete, false)

	// Step 5: Publish delete messages
	if sw.publisher != nil {
		if err := sw.publishServiceDeleteMessages(ctx, uid); err != nil {
			slog.ErrorContext(ctx, "failed to publish delete messages", "error", err)
		}
	}

	slog.DebugContext(ctx, "service deletion completed successfully",
		"service_uid", uid,
		"indices_deleted", len(indicesToDelete),
	)

	return nil
}

// validateAndPopulateProject validates project exists and populates project metadata
func (sw *grpsIOWriterOrchestrator) validateAndPopulateProject(ctx context.Context, service *model.GrpsIOService) error {
	if service.ProjectUID == "" {
		return errors.NewValidation("project_uid is required")
	}

	// Fetch project slug
	slug, err := sw.entityReader.ProjectSlug(ctx, service.ProjectUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve project slug",
			"error", err,
			"project_uid", service.ProjectUID,
		)
		return err
	}

	// Fetch project name
	name, err := sw.entityReader.ProjectName(ctx, service.ProjectUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve project name",
			"error", err,
			"project_uid", service.ProjectUID,
		)
		return err
	}

	// Populate service with project metadata
	service.ProjectSlug = slug
	service.ProjectName = name

	return nil
}

// reserveUniqueConstraints reserves unique constraints based on service type
func (sw *grpsIOWriterOrchestrator) reserveUniqueConstraints(ctx context.Context, service *model.GrpsIOService) (string, error) {
	switch service.Type {
	case "primary":
		// Primary service: unique by project only
		return sw.grpsIOWriter.UniqueProjectType(ctx, service)
	case "formation":
		// Formation service: unique by project + prefix
		return sw.grpsIOWriter.UniqueProjectPrefix(ctx, service)
	case "shared":
		// Shared service: unique by project + group_id
		return sw.grpsIOWriter.UniqueProjectGroupID(ctx, service)
	default:
		slog.WarnContext(ctx, "unknown service type - no constraint validation", "type", service.Type)
		return "", nil
	}
}

// publishServiceMessages publishes service-specific messages
func (sw *grpsIOWriterOrchestrator) publishServiceMessages(ctx context.Context, service *model.GrpsIOService, action model.MessageAction) error {
	if sw.publisher == nil {
		slog.WarnContext(ctx, "publisher not available, skipping service message publishing")
		return nil
	}

	// Build indexer message
	indexerMessage := &model.IndexerMessage{
		Action: action,
		Tags:   service.Tags(),
	}
	builtIndexerMessage, err := indexerMessage.Build(ctx, service)
	if err != nil {
		return fmt.Errorf("failed to build service indexer message: %w", err)
	}

	// Build access control message
	accessMessage := &model.AccessMessage{
		UID:        service.UID,
		ObjectType: "groupsio_service",
		Public:     service.Public,
		Relations:  map[string][]string{},
		References: map[string]string{
			constants.RelationProject: service.ProjectUID,
		},
	}

	// Publish messages concurrently
	messages := []func() error{
		func() error {
			return sw.publisher.Indexer(ctx, constants.IndexGroupsIOServiceSubject, builtIndexerMessage)
		},
		func() error {
			return sw.publisher.Access(ctx, constants.UpdateAccessGroupsIOServiceSubject, accessMessage)
		},
	}

	// Execute all messages concurrently
	errPublishingMessage := concurrent.NewWorkerPool(len(messages)).Run(ctx, messages...)
	if errPublishingMessage != nil {
		slog.ErrorContext(ctx, "failed to publish service messages",
			"error", errPublishingMessage,
			"service_id", service.UID,
		)
		return errPublishingMessage
	}

	slog.DebugContext(ctx, "service messages published successfully",
		"service_id", service.UID,
		"action", action,
	)

	return nil
}

// publishServiceDeleteMessages publishes service delete messages concurrently
func (sw *grpsIOWriterOrchestrator) publishServiceDeleteMessages(ctx context.Context, uid string) error {
	if sw.publisher == nil {
		slog.WarnContext(ctx, "publisher not available, skipping service delete message publishing")
		return nil
	}

	// For delete messages, we just need the UID
	indexerMessage := &model.IndexerMessage{
		Action: model.ActionDeleted,
		Tags:   []string{},
	}

	builtMessage, err := indexerMessage.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build service delete indexer message", "error", err, "service_uid", uid)
		return fmt.Errorf("failed to build service delete indexer message: %w", err)
	}

	// Publish delete messages concurrently
	messages := []func() error{
		func() error {
			return sw.publisher.Indexer(ctx, constants.IndexGroupsIOServiceSubject, builtMessage)
		},
		func() error {
			return sw.publisher.Access(ctx, constants.DeleteAllAccessGroupsIOServiceSubject, uid)
		},
	}

	// Execute all messages concurrently
	errPublishingMessage := concurrent.NewWorkerPool(len(messages)).Run(ctx, messages...)
	if errPublishingMessage != nil {
		slog.ErrorContext(ctx, "failed to publish service delete messages",
			"error", errPublishingMessage,
			"service_uid", uid,
		)
		return errPublishingMessage
	}

	slog.DebugContext(ctx, "service delete messages published successfully", "service_uid", uid)
	return nil
}

// mergeServiceData merges existing service data with updated fields
func (sw *grpsIOWriterOrchestrator) mergeServiceData(ctx context.Context, existing *model.GrpsIOService, updated *model.GrpsIOService) {
	// Preserve immutable fields
	updated.UID = existing.UID
	updated.CreatedAt = existing.CreatedAt
	updated.ProjectUID = existing.ProjectUID
	updated.Type = existing.Type
	updated.Prefix = existing.Prefix
	updated.Domain = existing.Domain
	updated.GroupID = existing.GroupID
	updated.URL = existing.URL
	updated.GroupName = existing.GroupName

	// Update timestamp
	updated.UpdatedAt = time.Now()

	slog.DebugContext(ctx, "service data merged",
		"service_id", existing.UID,
		"mutable_fields", []string{"global_owners", "status", "public"},
	)
}
