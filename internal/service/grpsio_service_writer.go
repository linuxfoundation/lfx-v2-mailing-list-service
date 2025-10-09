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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"
)

// CreateGrpsIOService creates a new service with transactional operations and rollback
func (sw *grpsIOWriterOrchestrator) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "executing create service use case",
		"service_type", service.Type,
		"service_domain", service.Domain,
		"project_uid", service.ProjectUID,
	)

	// Step 1: Set service identifiers and timestamps (server-side generation for security)
	now := time.Now()
	service.UID = uuid.New().String() // Always generate server-side, never trust client
	service.CreatedAt = now
	service.UpdatedAt = now

	// For rollback purposes
	var (
		keys             []string
		rollbackRequired bool
		serviceID        *int64
	)
	defer func() {
		if err := recover(); err != nil || rollbackRequired {
			sw.deleteKeys(ctx, keys, true)

			// Clean up GroupsIO resource if created
			if serviceID != nil && sw.groupsClient != nil {
				if deleteErr := sw.groupsClient.DeleteGroup(ctx, service.GetDomain(), utils.Int64PtrToUint64(serviceID)); deleteErr != nil {
					slog.WarnContext(ctx, "failed to cleanup GroupsIO group during rollback", "error", deleteErr, "group_id", *serviceID)
				}
			}
		}
	}()

	// Step 2: Validate project exists and populate metadata
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

	// Step 3: Reserve unique constraints based on service type
	constraintKey, err := sw.reserveUniqueConstraints(ctx, service)
	if err != nil {
		rollbackRequired = true
		return nil, 0, err
	}
	if constraintKey != "" {
		keys = append(keys, constraintKey)
	}

	// Step 4: Validate source (Service only supports API and Mock, not webhook)
	if service.Source != constants.SourceAPI && service.Source != constants.SourceMock {
		return nil, 0, errors.NewValidation(
			fmt.Sprintf("service only supports api or mock source, got: %s", service.Source))
	}

	// Step 5: Source-based strategy dispatch (API and Mock only)
	var (
		groupID         *int64
		requiresCleanup bool
	)

	switch service.Source {
	case constants.SourceAPI:
		groupID, requiresCleanup, err = sw.handleAPISourceService(ctx, service)
		if err != nil {
			rollbackRequired = true
			return nil, 0, err
		}
		if requiresCleanup {
			serviceID = groupID
		}
		// Set status based on Groups.io creation (preserves existing logic)
		if groupID != nil {
			service.Status = "active"
		} else {
			service.Status = "pending"
		}

	case constants.SourceMock:
		groupID = sw.handleMockSourceService(ctx, service)
		service.Status = "pending"

	default:
		// Should never reach here due to validation above
		return nil, 0, errors.NewValidation(
			fmt.Sprintf("unsupported source for service: %s", service.Source))
	}

	service.GroupID = groupID

	// Step 6: Create service in storage (with Groups.io ID already set)
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

	slog.DebugContext(ctx, "service created successfully",
		"service_uid", createdService.UID,
		"revision", revision,
	)

	// Step 7: Create secondary indices
	secondaryKeys, err := sw.createServiceSecondaryIndices(ctx, createdService)
	if err != nil {
		slog.WarnContext(ctx, "failed to create service secondary indices", "error", err)
		// Don't fail the operation, service creation succeeded
	} else {
		keys = append(keys, secondaryKeys...)
	}

	// Step 8: Publish messages (indexer and access control)
	if sw.publisher != nil {
		if err := sw.publishServiceMessages(ctx, createdService, model.ActionCreated); err != nil {
			slog.ErrorContext(ctx, "failed to publish messages", "error", err)
			// Don't fail the operation on message failure, service creation succeeded
		}
	}

	return createdService, revision, nil
}

// createServiceInGroupsIO handles Groups.io group creation and returns the ID
func (sw *grpsIOWriterOrchestrator) createServiceInGroupsIO(ctx context.Context, service *model.GrpsIOService) (*int64, error) {
	if sw.groupsClient == nil {
		return nil, nil // Skip Groups.io creation
	}

	// Use domain methods for effective values
	effectiveDomain := service.GetDomain()
	effectiveGroupName := service.GetGroupName()

	slog.InfoContext(ctx, "creating group in Groups.io",
		"domain", effectiveDomain,
		"group_name", effectiveGroupName,
	)

	groupOptions := groupsio.GroupCreateOptions{
		GroupName:      effectiveGroupName,
		Desc:           fmt.Sprintf("Mailing lists for %s", service.ProjectName), // Fixed: was Description
		Privacy:        "group_privacy_unlisted_public",                          // Production value
		SubGroupAccess: "sub_group_owners",                                       // Production value
		EmailDelivery:  "email_delivery_none",                                    // Production value
	}

	groupResult, err := sw.groupsClient.CreateGroup(ctx, effectiveDomain, groupOptions)
	if err != nil {
		slog.ErrorContext(ctx, "Groups.io group creation failed",
			"error", err,
			"domain", effectiveDomain,
			"group_name", service.GroupName,
		)
		return nil, fmt.Errorf("groups.io creation failed: %w", err)
	}

	groupID := int64(groupResult.ID)
	slog.InfoContext(ctx, "Groups.io group created successfully",
		"group_id", groupResult.ID,
		"domain", service.Domain,
	)

	// Step 2: Update group with additional settings that cannot be set at creation time
	announce := true
	updateOptions := groupsio.GroupUpdateOptions{
		Announce:              &announce,
		ReplyTo:               "group_reply_to_sender",
		MembersVisible:        "group_view_members_moderators",
		CalendarAccess:        "group_access_none",
		FilesAccess:           "group_access_none",
		DatabaseAccess:        "group_access_none",
		WikiAccess:            "group_access_none",
		PhotosAccess:          "group_access_none",
		MemberDirectoryAccess: "group_access_moderators_only",
		PollsAccess:           "polls_access_none",
		ChatAccess:            "group_access_none",
	}

	err = sw.groupsClient.UpdateGroup(ctx, effectiveDomain, uint64(groupID), updateOptions)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io group update failed, but group creation succeeded",
			"error", err,
			"group_id", groupID,
			"domain", effectiveDomain,
		)
		// Don't fail the creation if update fails, as the group was created successfully
		// TODO: Will be fixed in next PR to handle the sync status
	} else {
		slog.InfoContext(ctx, "Groups.io group updated with additional settings",
			"group_id", groupID,
			"domain", effectiveDomain,
		)
	}

	return &groupID, nil
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

	// Sync service updates to Groups.io
	sw.syncServiceToGroupsIO(ctx, updatedService)

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
func (sw *grpsIOWriterOrchestrator) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error {
	slog.DebugContext(ctx, "executing delete service use case",
		"service_uid", uid,
		"expected_revision", expectedRevision,
	)

	if service != nil {
		slog.DebugContext(ctx, "service data provided for deletion",
			"service_uid", service.UID,
			"service_type", service.Type,
			"project_uid", service.ProjectUID,
		)
	} else {
		slog.DebugContext(ctx, "no service data provided for deletion - will rely on storage layer for validation")
	}

	// Step 1: Delete the main service record (storage layer handles constraint cleanup)
	err := sw.grpsIOWriter.DeleteGrpsIOService(ctx, uid, expectedRevision, service)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete service",
			"error", err,
			"service_uid", uid,
			"expected_revision", expectedRevision,
		)
		return err
	}

	slog.DebugContext(ctx, "service record deleted successfully",
		"service_uid", uid,
	)

	// Step 2: Publish delete messages
	if sw.publisher != nil {
		if err := sw.publishServiceDeleteMessages(ctx, uid); err != nil {
			slog.ErrorContext(ctx, "failed to publish delete messages", "error", err)
		}
	}

	slog.DebugContext(ctx, "service deletion completed successfully",
		"service_uid", uid,
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
	case constants.ServiceTypePrimary:
		// Primary service: unique by project only
		return sw.grpsIOWriter.UniqueProjectType(ctx, service)
	case constants.ServiceTypeFormation:
		// Formation service: unique by project + prefix
		return sw.grpsIOWriter.UniqueProjectPrefix(ctx, service)
	case constants.ServiceTypeShared:
		// Shared service: unique by project + group_id
		return sw.grpsIOWriter.UniqueProjectGroupID(ctx, service)
	default:
		slog.WarnContext(ctx, "unknown service type - no constraint validation", "type", service.Type)
		return "", nil
	}
}

// createServiceSecondaryIndices creates secondary indices for external GroupsIO IDs
func (sw *grpsIOWriterOrchestrator) createServiceSecondaryIndices(ctx context.Context, service *model.GrpsIOService) ([]string, error) {
	// Delegate to storage layer (port interface)
	// This interface method needs to be added to port.GrpsIOServiceWriter
	type serviceIndexCreator interface {
		CreateServiceSecondaryIndices(ctx context.Context, service *model.GrpsIOService) ([]string, error)
	}

	if indexCreator, ok := sw.grpsIOWriter.(serviceIndexCreator); ok {
		return indexCreator.CreateServiceSecondaryIndices(ctx, service)
	}

	// If storage implementation doesn't support indexing (e.g., mock), skip silently
	slog.DebugContext(ctx, "storage implementation doesn't support secondary indices")
	return nil, nil
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
	relations := map[string][]string{}
	if len(service.GlobalOwners) > 0 {
		relations[constants.RelationOwner] = service.GlobalOwners
	}
	if len(service.Writers) > 0 {
		relations[constants.RelationWriter] = service.Writers
	}
	if len(service.Auditors) > 0 {
		relations[constants.RelationAuditor] = service.Auditors
	}

	accessMessage := &model.AccessMessage{
		UID:        service.UID,
		ObjectType: constants.ObjectTypeGroupsIOService,
		Public:     service.Public,
		Relations:  relations,
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

// syncServiceToGroupsIO handles Groups.io service update with proper error handling
func (sw *grpsIOWriterOrchestrator) syncServiceToGroupsIO(ctx context.Context, service *model.GrpsIOService) {
	// Guard clause: skip if Groups.io client not available or service not synced
	if sw.groupsClient == nil || service.GroupID == nil {
		slog.InfoContext(ctx, "Groups.io integration disabled or service not synced - skipping Groups.io update")
		return
	}

	// Get domain using helper method
	domain, err := sw.getGroupsIODomainForResource(ctx, service.UID, constants.ResourceTypeService)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io service sync skipped due to domain lookup failure, local update will proceed",
			"error", err, "service_uid", service.UID)
		return
	}

	// Build update options from service model
	updates := groupsio.GroupUpdateOptions{
		GlobalOwners: service.GlobalOwners,
	}

	// Perform Groups.io service update
	err = sw.groupsClient.UpdateGroup(ctx, domain, utils.Int64PtrToUint64(service.GroupID), updates)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io service update failed, local update will proceed",
			"error", err, "domain", domain, "group_id", *service.GroupID)
	} else {
		slog.InfoContext(ctx, "Groups.io service updated successfully",
			"group_id", *service.GroupID, "domain", domain)
	}
}

// handleAPISourceService handles API-initiated service creation
// Preserves existing logic: calls createServiceInGroupsIO and returns cleanup flag
// For shared services with pre-provided GroupID, preserves the input value
func (sw *grpsIOWriterOrchestrator) handleAPISourceService(
	ctx context.Context,
	service *model.GrpsIOService,
) (*int64, bool, error) {
	// Check if GroupID is already provided (shared service scenario)
	if service.GroupID != nil {
		slog.InfoContext(ctx, "source=api: using pre-provided group_id for shared service",
			"group_id", *service.GroupID,
			"domain", service.GetDomain())
		return service.GroupID, false, nil
	}

	slog.InfoContext(ctx, "source=api: creating group in Groups.io",
		"domain", service.GetDomain(),
		"project_uid", service.ProjectUID)

	// Call existing createServiceInGroupsIO method (preserves all existing logic)
	groupID, err := sw.createServiceInGroupsIO(ctx, service)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create group in Groups.io",
			"error", err,
			"domain", service.GetDomain())
		return nil, false, err
	}

	// Determine if cleanup is required (preserves existing rollback logic)
	requiresCleanup := groupID != nil

	if groupID != nil {
		slog.InfoContext(ctx, "group created successfully in Groups.io",
			"group_id", *groupID)
	} else {
		slog.InfoContext(ctx, "Groups.io client not available, service will be in pending state")
	}

	return groupID, requiresCleanup, nil
}

// handleMockSourceService handles mock/test mode service creation
// Preserves existing logic: returns nil for groupID
func (sw *grpsIOWriterOrchestrator) handleMockSourceService(
	ctx context.Context,
	service *model.GrpsIOService,
) *int64 {
	slog.InfoContext(ctx, "source=mock: skipping Groups.io coordination",
		"domain", service.GetDomain())
	return nil
}

// Note: No handleWebhookSourceService needed
// Services are created explicitly via API, not discovered from webhooks
// Only mailing lists (subgroups) and members can be webhook-adopted
