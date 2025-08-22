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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// GrpsIOServiceWriter defines the interface for service write operations
type GrpsIOServiceWriter interface {
	// CreateGrpsIOService creates a new service and returns the service with revision
	CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error)

	// UpdateGrpsIOService updates an existing service with expected revision and returns updated service with new revision
	UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error)

	// DeleteGrpsIOService deletes a service by UID with expected revision
	DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64) error
}

// grpsIOServiceWriterOrchestratorOption defines a function type for setting options
type grpsIOServiceWriterOrchestratorOption func(*grpsIOServiceWriterOrchestrator)

// WithServiceWriter sets the service writer
func WithServiceWriter(writer port.GrpsIOServiceWriter) grpsIOServiceWriterOrchestratorOption {
	return func(w *grpsIOServiceWriterOrchestrator) {
		w.grpsIOServiceWriter = writer
	}
}

// WithProjectRetriever sets the project reader
func WithProjectRetriever(reader port.ProjectReader) grpsIOServiceWriterOrchestratorOption {
	return func(w *grpsIOServiceWriterOrchestrator) {
		w.projectReader = reader
	}
}

// WithPublisher sets the publisher
func WithPublisher(publisher port.MessagePublisher) grpsIOServiceWriterOrchestratorOption {
	return func(w *grpsIOServiceWriterOrchestrator) {
		w.publisher = publisher
	}
}

// WithGrpsIOServiceReader sets the service reader for the writer orchestrator
func WithGrpsIOServiceReader(reader port.GrpsIOServiceReader) grpsIOServiceWriterOrchestratorOption {
	return func(w *grpsIOServiceWriterOrchestrator) {
		w.grpsIOServiceReader = reader
	}
}

// grpsIOServiceWriterOrchestrator orchestrates the service writing process
type grpsIOServiceWriterOrchestrator struct {
	grpsIOServiceWriter port.GrpsIOServiceWriter
	grpsIOServiceReader port.GrpsIOServiceReader
	projectReader       port.ProjectReader
	publisher           port.MessagePublisher
}

// CreateGrpsIOService creates a new service with transactional operations and rollback
func (sw *grpsIOServiceWriterOrchestrator) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
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
	createdService, revision, err := sw.grpsIOServiceWriter.CreateGrpsIOService(ctx, service)
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
		if err := sw.publishMessages(ctx, createdService, model.ActionCreated); err != nil {
			slog.ErrorContext(ctx, "failed to publish messages", "error", err)
			// Don't rollback on message failure, service creation succeeded
		}
	}

	return createdService, revision, nil
}

// UpdateGrpsIOService updates an existing service with transactional patterns
func (sw *grpsIOServiceWriterOrchestrator) UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error) {
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
	existing, existingRevision, err := sw.grpsIOServiceReader.GetGrpsIOService(ctx, uid)
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
	updatedService, revision, err := sw.grpsIOServiceWriter.UpdateGrpsIOService(ctx, uid, service, expectedRevision)
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
		if err := sw.publishMessages(ctx, updatedService, model.ActionUpdated); err != nil {
			slog.ErrorContext(ctx, "failed to publish update messages", "error", err)
			// Don't fail the update on message publishing errors
		}
	}

	// Mark update as successful for defer cleanup
	updateSucceeded = true
	return updatedService, revision, nil
}

// DeleteGrpsIOService deletes a service with message publishing
func (sw *grpsIOServiceWriterOrchestrator) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64) error {
	slog.DebugContext(ctx, "executing delete service use case",
		"service_uid", uid,
		"expected_revision", expectedRevision,
	)

	// Step 1: Retrieve existing service data to get all the information needed for cleanup
	existing, existingRevision, err := sw.grpsIOServiceReader.GetGrpsIOService(ctx, uid)
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
	err = sw.grpsIOServiceWriter.DeleteGrpsIOService(ctx, uid, expectedRevision)
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
		if err := sw.publishDeleteMessages(ctx, uid); err != nil {
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
func (sw *grpsIOServiceWriterOrchestrator) validateAndPopulateProject(ctx context.Context, service *model.GrpsIOService) error {
	if service.ProjectUID == "" {
		return errors.NewValidation("project_uid is required")
	}

	// Fetch project slug
	slug, err := sw.projectReader.Slug(ctx, service.ProjectUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve project slug",
			"error", err,
			"project_uid", service.ProjectUID,
		)
		return err
	}

	// Fetch project name
	name, err := sw.projectReader.Name(ctx, service.ProjectUID)
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
func (sw *grpsIOServiceWriterOrchestrator) reserveUniqueConstraints(ctx context.Context, service *model.GrpsIOService) (string, error) {
	switch service.Type {
	case "primary":
		// Primary service: unique by project only
		return sw.grpsIOServiceWriter.UniqueProjectType(ctx, service)
	case "formation":
		// Formation service: unique by project + prefix
		return sw.grpsIOServiceWriter.UniqueProjectPrefix(ctx, service)
	case "shared":
		// Shared service: unique by project + group_id
		return sw.grpsIOServiceWriter.UniqueProjectGroupID(ctx, service)
	default:
		slog.WarnContext(ctx, "unknown service type - no constraint validation", "type", service.Type)
		return "", nil
	}
}

// publishMessages publishes indexer and access control messages concurrently
func (sw *grpsIOServiceWriterOrchestrator) publishMessages(ctx context.Context, service *model.GrpsIOService, action model.MessageAction) error {
	if sw.publisher == nil {
		slog.WarnContext(ctx, "publisher not available, skipping message publishing")
		return nil
	}

	// Build indexer message
	indexerMessage, err := sw.buildIndexerMessage(ctx, service, action, service.Tags())
	if err != nil {
		return fmt.Errorf("failed to build indexer message: %w", err)
	}

	// Build access control message
	accessMessage := sw.buildAccessControlMessage(ctx, service)

	// Publish messages concurrently
	messages := []func() error{
		func() error {
			return sw.publisher.Indexer(ctx, constants.IndexGrpsIOServiceSubject, indexerMessage)
		},
		func() error {
			return sw.publisher.Access(ctx, constants.UpdateAccessGrpsIOServiceSubject, accessMessage)
		},
	}

	// Execute all messages concurrently
	errPublishingMessage := concurrent.NewWorkerPool(len(messages)).Run(ctx, messages...)
	if errPublishingMessage != nil {
		slog.ErrorContext(ctx, "failed to publish messages",
			"error", errPublishingMessage,
			"service_id", service.UID,
		)
		return errPublishingMessage
	}

	slog.DebugContext(ctx, "messages published successfully",
		"service_id", service.UID,
		"action", action,
	)

	return nil
}

// publishDeleteMessages publishes delete messages concurrently
func (sw *grpsIOServiceWriterOrchestrator) publishDeleteMessages(ctx context.Context, uid string) error {
	if sw.publisher == nil {
		slog.WarnContext(ctx, "publisher not available, skipping delete message publishing")
		return nil
	}

	// For delete messages, we just need the UID
	indexerMessage := &model.IndexerMessage{
		Action: model.ActionDeleted,
		Tags:   []string{}, // No tags needed for deletion
	}

	builtMessage, err := indexerMessage.Build(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build delete indexer message", "error", err, "service_uid", uid)
		return fmt.Errorf("failed to build delete indexer message: %w", err)
	}

	// Publish delete messages concurrently
	messages := []func() error{
		func() error {
			return sw.publisher.Indexer(ctx, constants.IndexGrpsIOServiceSubject, builtMessage)
		},
		func() error {
			return sw.publisher.Access(ctx, constants.DeleteAllAccessGrpsIOServiceSubject, uid)
		},
	}

	// Execute all messages concurrently
	errPublishingMessage := concurrent.NewWorkerPool(len(messages)).Run(ctx, messages...)
	if errPublishingMessage != nil {
		slog.ErrorContext(ctx, "failed to publish delete messages",
			"error", errPublishingMessage,
			"service_uid", uid,
		)
		return errPublishingMessage
	}

	slog.DebugContext(ctx, "delete messages published successfully", "service_uid", uid)
	return nil
}

// buildIndexerMessage builds an indexer message for the service
func (sw *grpsIOServiceWriterOrchestrator) buildIndexerMessage(ctx context.Context, service *model.GrpsIOService, action model.MessageAction, tags []string) (*model.IndexerMessage, error) {
	indexerMessage := &model.IndexerMessage{
		Action: action,
		Tags:   tags,
	}

	return indexerMessage.Build(ctx, service)
}

// buildAccessControlMessage builds an access control message for the service
func (sw *grpsIOServiceWriterOrchestrator) buildAccessControlMessage(ctx context.Context, service *model.GrpsIOService) *model.AccessMessage {
	message := &model.AccessMessage{
		UID:        service.UID,
		ObjectType: "grpsio_service",
		Public:     service.Public,
		// Relations is reserved for future use and is intentionally left empty
		Relations: map[string][]string{},
		References: map[string]string{
			// project is required in the flow for inheritance
			constants.RelationProject: service.ProjectUID,
		},
	}

	slog.DebugContext(ctx, "building access control message",
		"service_id", service.UID,
		"public", service.Public,
		"project_uid", service.ProjectUID,
	)

	return message
}

// mergeServiceData merges existing service data with updated fields
func (sw *grpsIOServiceWriterOrchestrator) mergeServiceData(ctx context.Context, existing *model.GrpsIOService, updated *model.GrpsIOService) {
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

// deleteKeys removes keys by getting their revision and deleting them
// This is used both for rollback scenarios and cleanup of stale keys
func (sw *grpsIOServiceWriterOrchestrator) deleteKeys(ctx context.Context, keys []string, isRollback bool) {
	if len(keys) == 0 {
		return
	}

	slog.DebugContext(ctx, "deleting keys",
		"keys", keys,
		"is_rollback", isRollback,
	)

	for _, key := range keys {
		// For service UIDs, use the reader interface; for constraint keys, get revision directly from storage
		var rev uint64
		var errGet error

		// Try to get revision using reader interface first (for service UIDs)
		if sw.grpsIOServiceReader != nil {
			rev, errGet = sw.grpsIOServiceReader.GetRevision(ctx, key)
		}

		// If reader method fails, try the direct storage approach (for constraint keys)
		if errGet != nil {
			rev, errGet = sw.grpsIOServiceWriter.GetKeyRevision(ctx, key)
		}

		if errGet != nil {
			slog.ErrorContext(ctx, "failed to get revision for key deletion",
				"key", key,
				"error", errGet,
				"is_rollback", isRollback,
			)
			continue
		}

		err := sw.grpsIOServiceWriter.Delete(ctx, key, rev)
		if err != nil {
			slog.ErrorContext(ctx, "failed to delete key",
				"key", key,
				"error", err,
				"is_rollback", isRollback,
			)
		} else {
			slog.DebugContext(ctx, "successfully deleted key",
				"key", key,
				"is_rollback", isRollback,
			)
		}
	}

	slog.DebugContext(ctx, "key deletion completed",
		"keys_count", len(keys),
		"is_rollback", isRollback,
	)
}

// NewGrpsIOServiceWriterOrchestrator creates a new service writer use case using the option pattern
func NewGrpsIOServiceWriterOrchestrator(opts ...grpsIOServiceWriterOrchestratorOption) GrpsIOServiceWriter {
	sw := &grpsIOServiceWriterOrchestrator{}
	for _, opt := range opts {
		opt(sw)
	}
	if sw.grpsIOServiceWriter == nil {
		panic("grpsIOServiceWriter is required")
	}
	return sw
}
