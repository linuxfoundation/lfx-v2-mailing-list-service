// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"
)

// handleIdempotencyLookupError processes errors from idempotency lookups and determines the appropriate action
// Returns true if the error was handled (caller should continue), false if it should be propagated
//
// This helper handles distributed systems challenges with NATS KV + Database:
// - NotFound: Expected during idempotency checks - safe to continue
// - ServiceUnavailable: NATS/storage unavailable - fail operation to prevent duplicates
// - Other errors: Unexpected errors - propagate for investigation
func handleIdempotencyLookupError(ctx context.Context, err error, lookupType, identifier string) (bool, error) {
	var notFoundErr errs.NotFound
	var unavailableErr errs.ServiceUnavailable

	if stderrors.As(err, &notFoundErr) {
		// NotFound is expected during idempotency checks - safe to continue
		slog.DebugContext(ctx, "not found during idempotency check, will continue",
			"lookup_type", lookupType,
			"identifier", identifier)
		return true, nil
	}

	if stderrors.As(err, &unavailableErr) {
		// Storage unavailable - cannot verify idempotency safely
		// Fail operation to prevent potential duplicates
		slog.ErrorContext(ctx, "storage unavailable during idempotency check, cannot verify if entity exists",
			"error", err,
			"lookup_type", lookupType,
			"identifier", identifier)
		return false, err // Propagate ServiceUnavailable → HTTP 503 → client retry
	}

	// Unexpected error (data corruption, permission denied, etc.) - propagate
	slog.ErrorContext(ctx, "unexpected error during idempotency check",
		"error", err,
		"lookup_type", lookupType,
		"identifier", identifier)
	return false, fmt.Errorf("idempotency check failed: %w", err)
}

// GrpsIOWriter defines the composite interface that combines writers
type GrpsIOWriter interface {
	GrpsIOServiceWriter
	GrpsIOMailingListWriter
	port.GrpsIOMemberWriter
}

// GrpsIOServiceWriter defines the interface for service write operations
type GrpsIOServiceWriter interface {
	// CreateGrpsIOService creates a new service and its settings, and returns the service, settings, and revision
	CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService, settings *model.GrpsIOServiceSettings) (*model.GrpsIOServiceFull, uint64, error)

	// UpdateGrpsIOService updates an existing service with expected revision and returns updated service with new revision
	UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error)

	// DeleteGrpsIOService deletes a service by UID with expected revision
	// Pass the existing service data to DeleteGrpsIOService to allow the storage layer to perform
	// constraint cleanup based on the current state of the service. The 'service' parameter provides
	// necessary context for deleting related constraints or dependent records.
	DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error

	// UpdateGrpsIOServiceSettings updates service settings with expected revision and returns updated settings with new revision
	UpdateGrpsIOServiceSettings(ctx context.Context, settings *model.GrpsIOServiceSettings, expectedRevision uint64) (*model.GrpsIOServiceSettings, uint64, error)
}

// GrpsIOMailingListWriter defines the interface for mailing list write operations
type GrpsIOMailingListWriter interface {
	// CreateGrpsIOMailingList creates a new mailing list and returns the mailing list with revision
	CreateGrpsIOMailingList(ctx context.Context, request *model.GrpsIOMailingList, settings *model.GrpsIOMailingListSettings) (*model.GrpsIOMailingList, uint64, error)

	// UpdateGrpsIOMailingList updates an existing mailing list with expected revision and returns updated mailing list with new revision
	UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error)

	// DeleteGrpsIOMailingList deletes a mailing list by UID with expected revision
	DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error

	// UpdateGrpsIOMailingListSettings updates mailing list settings with expected revision and returns updated settings with new revision
	UpdateGrpsIOMailingListSettings(ctx context.Context, settings *model.GrpsIOMailingListSettings, expectedRevision uint64) (*model.GrpsIOMailingListSettings, uint64, error)
}

// grpsIOWriterOrchestratorOption defines a function type for setting options on the composite orchestrator
type grpsIOWriterOrchestratorOption func(*grpsIOWriterOrchestrator)

// WithGrpsIOWriter sets the writer orchestrator
func WithGrpsIOWriter(writer port.GrpsIOWriter) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.grpsIOWriter = writer
	}
}

// WithGrpsIOWriterReader sets the reader orchestrator for writer
func WithGrpsIOWriterReader(reader port.GrpsIOReader) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.grpsIOReader = reader
	}
}

// WithEntityAttributeReader sets the entity attribute reader
func WithEntityAttributeReader(reader port.EntityAttributeReader) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.entityReader = reader
	}
}

// WithPublisher sets the publisher
func WithPublisher(publisher port.MessagePublisher) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.publisher = publisher
	}
}

// WithGroupsIOClient sets the GroupsIO client (may be nil for mock/disabled mode)
func WithGroupsIOClient(client groupsio.ClientInterface) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.groupsClient = client
	}
}

// WithMemberRepository sets the member repository for direct storage operations
func WithMemberRepository(repo port.GrpsIOMemberRepository) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.memberRepository = repo
	}
}

// grpsIOWriterOrchestrator orchestrates the service writing process
type grpsIOWriterOrchestrator struct {
	grpsIOWriter     port.GrpsIOWriter
	memberRepository port.GrpsIOMemberRepository
	grpsIOReader     port.GrpsIOReader
	entityReader     port.EntityAttributeReader
	publisher        port.MessagePublisher
	groupsClient     groupsio.ClientInterface // May be nil for mock/disabled mode
}

// NewGrpsIOWriterOrchestrator creates a new composite writer orchestrator using the option pattern
func NewGrpsIOWriterOrchestrator(opts ...grpsIOWriterOrchestratorOption) GrpsIOWriter {
	uc := &grpsIOWriterOrchestrator{}
	for _, opt := range opts {
		opt(uc)
	}

	return uc
}

// BaseGrpsIOWriter methods - delegated to underlying writer

// GetKeyRevision retrieves the revision for a given key (used for cleanup operations)
func (o *grpsIOWriterOrchestrator) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	return o.grpsIOWriter.GetKeyRevision(ctx, key)
}

// Delete removes a key with the given revision (used for cleanup and rollback)
func (o *grpsIOWriterOrchestrator) Delete(ctx context.Context, key string, revision uint64) error {
	return o.grpsIOWriter.Delete(ctx, key, revision)
}

// UniqueMember validates member email is unique within mailing list
func (o *grpsIOWriterOrchestrator) UniqueMember(ctx context.Context, member *model.GrpsIOMember) (string, error) {
	return o.memberRepository.UniqueMember(ctx, member)
}

// CreateMemberSecondaryIndices creates lookup indices for Groups.io IDs
func (o *grpsIOWriterOrchestrator) CreateMemberSecondaryIndices(ctx context.Context, member *model.GrpsIOMember) ([]string, error) {
	return o.memberRepository.CreateMemberSecondaryIndices(ctx, member)
}

// Common methods implementation

// deleteKeys removes keys by getting their revision and deleting them
// This is used both for rollback scenarios and cleanup operations
func (o *grpsIOWriterOrchestrator) deleteKeys(ctx context.Context, keys []string, isRollback bool) {
	if len(keys) == 0 {
		return
	}

	slog.DebugContext(ctx, "deleting keys",
		"keys", keys,
		"is_rollback", isRollback,
	)

	for _, key := range keys {
		// Get revision using reader interface first (for entity UIDs), then try direct storage (for constraint keys)
		var rev uint64
		var errGet error

		// Try to get revision using reader interface first (works for entity UIDs)
		if o.grpsIOReader != nil {
			rev, errGet = o.grpsIOReader.GetRevision(ctx, key)
		}

		// If reader method fails, try the direct storage approach (works for constraint keys)
		if errGet != nil {
			rev, errGet = o.grpsIOWriter.GetKeyRevision(ctx, key)
		}

		if errGet != nil {
			slog.ErrorContext(ctx, "failed to get revision for key deletion",
				"key", key,
				"error", errGet,
				"is_rollback", isRollback,
			)
			continue
		}

		// Delete the key using the revision
		err := o.grpsIOWriter.Delete(ctx, key, rev)
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

// getGroupsIODomainForResource resolves the Groups.io domain for a resource
// Handles both direct service lookup and member -> mailing list -> service lookup chains
// getServiceFromMailingList retrieves the parent service for a mailing list
func (o *grpsIOWriterOrchestrator) getServiceFromMailingList(ctx context.Context, mailingListUID string) (*model.GrpsIOService, error) {
	mailingList, _, err := o.grpsIOReader.GetGrpsIOMailingList(ctx, mailingListUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list for Groups.io domain", "error", err, "mailing_list_uid", mailingListUID)
		return nil, err
	}

	parentService, _, err := o.grpsIOReader.GetGrpsIOService(ctx, mailingList.ServiceUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get parent service for Groups.io domain", "error", err, "service_uid", mailingList.ServiceUID)
		return nil, err
	}

	return parentService, nil
}

func (o *grpsIOWriterOrchestrator) getGroupsIODomainForResource(ctx context.Context, resourceUID string, resourceType string) (string, error) {
	switch resourceType {
	case constants.ResourceTypeService:
		// Direct service lookup
		service, _, err := o.grpsIOReader.GetGrpsIOService(ctx, resourceUID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get service for Groups.io domain", "error", err, "service_uid", resourceUID)
			return "", err
		}
		return service.GetDomain(), nil

	case constants.ResourceTypeMember:
		// Member -> Mailing List -> Service lookup chain
		member, _, err := o.grpsIOReader.GetGrpsIOMember(ctx, resourceUID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get member for Groups.io domain", "error", err, "member_uid", resourceUID)
			return "", err
		}

		parentService, err := o.getServiceFromMailingList(ctx, member.MailingListUID)
		if err != nil {
			return "", err
		}

		return parentService.GetDomain(), nil

	case constants.ResourceTypeMailingList:
		// Mailing List -> Service lookup
		parentService, err := o.getServiceFromMailingList(ctx, resourceUID)
		if err != nil {
			return "", err
		}

		return parentService.GetDomain(), nil

	default:
		return "", fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// deleteSubgroupWithCleanup handles Groups.io subgroup deletion with proper error handling
func (o *grpsIOWriterOrchestrator) deleteSubgroupWithCleanup(ctx context.Context, serviceUID string, subgroupID *int64) {
	// Guard clause: skip if Groups.io client not available or subgroup not synced
	if o.groupsClient == nil || subgroupID == nil {
		slog.InfoContext(ctx, "Groups.io integration disabled or mailing list not synced - skipping subgroup deletion")
		return
	}

	// Get domain using helper method
	domain, err := o.getGroupsIODomainForResource(ctx, serviceUID, constants.ResourceTypeService)
	if err != nil {
		slog.WarnContext(ctx, "Groups.io subgroup cleanup skipped due to parent service lookup failure, local deletion will proceed",
			"error", err, "service_uid", serviceUID)
		return
	}

	// Perform Groups.io subgroup deletion
	err = o.groupsClient.DeleteSubgroup(ctx, domain, utils.Int64PtrToUint64(subgroupID))
	if err != nil {
		slog.WarnContext(ctx, "Groups.io subgroup deletion failed, local deletion will proceed - orphaned subgroups can be cleaned up later",
			"error", err, "domain", domain, "subgroup_id", *subgroupID)
	} else {
		slog.InfoContext(ctx, "Groups.io subgroup deleted successfully",
			"subgroup_id", *subgroupID, "domain", domain)
	}
}

// removeMemberFromGroupsIO handles Groups.io member deletion with proper error handling
func (o *grpsIOWriterOrchestrator) removeMemberFromGroupsIO(ctx context.Context, member *model.GrpsIOMember) error {
	// Guard clause: skip if Groups.io client not available or member not synced
	if o.groupsClient == nil || member == nil || member.MemberID == nil {
		slog.InfoContext(ctx, "Groups.io integration disabled or member not synced - skipping Groups.io deletion")
		return nil
	}

	logger := slog.With("member_uid", member.UID)
	if member.GroupID != nil {
		logger = logger.With("group_id", *member.GroupID)
	}
	if member.MemberID != nil {
		logger = logger.With("member_id", *member.MemberID)
	}

	// Get domain using helper method through member lookup chain
	domain, err := o.getGroupsIODomainForResource(ctx, member.UID, constants.ResourceTypeMember)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get Groups.io domain for member", "error", err)
		return fmt.Errorf("failed to get Groups.io domain for member: %w", err)
	}
	logger = logger.With("domain", domain)

	// Perform Groups.io member removal
	err = o.groupsClient.RemoveMember(ctx, domain, utils.Int64PtrToUint64(member.GroupID), utils.Int64PtrToUint64(member.MemberID))
	if err != nil {
		logger.ErrorContext(ctx, "Groups.io member deletion failed", "error", err)
		return fmt.Errorf("Groups.io member deletion failed: %w", err)
	}

	logger.InfoContext(ctx, "Groups.io member deleted successfully")
	return nil
}
