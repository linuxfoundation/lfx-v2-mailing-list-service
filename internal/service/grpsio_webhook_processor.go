// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
)

// grpsIOWebhookProcessor orchestrates webhook event processing with required dependencies
type grpsIOWebhookProcessor struct {
	serviceReader     port.GrpsIOServiceReader
	mailingListReader port.GrpsIOMailingListReader
	mailingListWriter port.GrpsIOMailingListWriter
	memberReader      port.GrpsIOMemberReader
	memberWriter      port.GrpsIOMemberWriter
}

// WebhookProcessorOption configures the webhook processor
type WebhookProcessorOption func(*grpsIOWebhookProcessor)

// WithServiceReader sets the service reader dependency
func WithServiceReader(reader port.GrpsIOServiceReader) WebhookProcessorOption {
	return func(p *grpsIOWebhookProcessor) {
		p.serviceReader = reader
	}
}

// WithMailingListReader sets the mailing list reader dependency
func WithMailingListReader(reader port.GrpsIOMailingListReader) WebhookProcessorOption {
	return func(p *grpsIOWebhookProcessor) {
		p.mailingListReader = reader
	}
}

// WithMailingListWriter sets the mailing list writer dependency
func WithMailingListWriter(writer port.GrpsIOMailingListWriter) WebhookProcessorOption {
	return func(p *grpsIOWebhookProcessor) {
		p.mailingListWriter = writer
	}
}

// WithMemberReader sets the member reader dependency
func WithMemberReader(reader port.GrpsIOMemberReader) WebhookProcessorOption {
	return func(p *grpsIOWebhookProcessor) {
		p.memberReader = reader
	}
}

// WithMemberWriter sets the member writer dependency
func WithMemberWriter(writer port.GrpsIOMemberWriter) WebhookProcessorOption {
	return func(p *grpsIOWebhookProcessor) {
		p.memberWriter = writer
	}
}

// NewGrpsIOWebhookProcessor creates a new GroupsIO webhook processor with dependencies
func NewGrpsIOWebhookProcessor(opts ...WebhookProcessorOption) port.GrpsIOWebhookProcessor {
	processor := &grpsIOWebhookProcessor{}

	for _, opt := range opts {
		opt(processor)
	}

	return processor
}

// ProcessEvent routes webhook events to appropriate handlers
func (p *grpsIOWebhookProcessor) ProcessEvent(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	slog.InfoContext(ctx, "processing groupsio webhook event", "event_type", event.Action)

	switch event.Action {
	case constants.SubGroupCreatedEvent:
		return p.handleSubGroupCreated(ctx, event)
	case constants.SubGroupDeletedEvent:
		return p.handleSubGroupDeleted(ctx, event)
	case constants.SubGroupMemberAddedEvent:
		return p.handleMemberAdded(ctx, event)
	case constants.SubGroupMemberRemovedEvent:
		return p.handleMemberRemoved(ctx, event)
	case constants.SubGroupMemberBannedEvent:
		return p.handleMemberBanned(ctx, event)
	default:
		slog.WarnContext(ctx, "unknown groupsio webhook event type", "event_type", event.Action)
		return nil // Ignore unknown events
	}
}

// MINIMAL HANDLERS - Log and validate only

func (p *grpsIOWebhookProcessor) handleSubGroupCreated(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.Group == nil {
		return errors.NewValidation("missing group information in created_subgroup event")
	}

	parentGroupID := uint64(event.Group.ParentGroupID)
	subgroupID := uint64(event.Group.ID)
	subgroupSuffix := event.Extra // e.g., "developers" from "myproject+developers"
	fullSubgroupName := fmt.Sprintf("%s+%s", event.Group.Name, subgroupSuffix)

	slog.InfoContext(ctx, "received created_subgroup event",
		"subgroup_name", fullSubgroupName,
		"parent_group_id", parentGroupID,
		"subgroup_id", subgroupID,
		"subgroup_suffix", subgroupSuffix)

	// Step 1: Find all services for the parent group_id
	services, err := p.serviceReader.GetServicesByGroupID(ctx, parentGroupID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get services by group_id",
			"parent_group_id", parentGroupID,
			"error", err)
		return fmt.Errorf("failed to get services by group_id: %w", err)
	}

	if len(services) == 0 {
		slog.WarnContext(ctx, "no services found for parent group_id - subgroup will not be adopted",
			"parent_group_id", parentGroupID,
			"subgroup_name", fullSubgroupName)
		return nil // Not an error - subgroup just won't be adopted
	}

	// Step 2: Determine which service should adopt this subgroup
	// Priority: prefix-matching service > primary service
	adoptingService := p.findAdoptingService(ctx, services, subgroupSuffix)
	if adoptingService == nil {
		slog.WarnContext(ctx, "no suitable service found to adopt subgroup",
			"parent_group_id", parentGroupID,
			"subgroup_name", fullSubgroupName)
		return nil // Not an error - just log and skip
	}

	// Step 3: Prepare mailing list for creation (idempotency handled by orchestrator)
	subgroupIDInt := int64(subgroupID)
	mailingList := &model.GrpsIOMailingList{
		ServiceUID:  adoptingService.UID,
		ProjectUID:  adoptingService.ProjectUID,
		GroupName:   fullSubgroupName,
		GroupID:     &subgroupIDInt,
		Source:      constants.SourceWebhook, // Orchestrator uses this for dispatch
		Type:        model.TypeDiscussionOpen,
		Description: "Auto-created from Groups.io webhook",
		Title:       fullSubgroupName,
	}

	// Note: UID, CreatedAt, UpdatedAt, and validation are handled by orchestrator
	// Note: Idempotency check moved to orchestrator (ensureMailingListIdempotent)

	// Step 4: Create mailing list (orchestrator handles all validation and idempotency)
	createdList, _, err := p.mailingListWriter.CreateGrpsIOMailingList(ctx, mailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list",
			"error", err,
			"mailing_list_uid", mailingList.UID,
			"service_uid", adoptingService.UID)
		return fmt.Errorf("failed to create mailing list: %w", err)
	}

	slog.InfoContext(ctx, "successfully adopted subgroup",
		"mailing_list_uid", createdList.UID,
		"service_uid", adoptingService.UID,
		"service_type", adoptingService.Type,
		"subgroup_name", fullSubgroupName,
		"subgroup_id", subgroupID)

	return nil
}

func (p *grpsIOWebhookProcessor) handleSubGroupDeleted(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	subgroupID := uint64(event.ExtraID)
	if subgroupID == 0 {
		slog.WarnContext(ctx, "deleted_subgroup event missing subgroup_id; ignoring")
		return nil
	}

	slog.InfoContext(ctx, "received deleted_subgroup event",
		"subgroup_id", subgroupID)

	// Step 1: Find mailing list by subgroup_id
	mailingList, revision, err := p.mailingListReader.GetMailingListByGroupID(ctx, subgroupID)
	if err != nil {
		var notFoundErr errors.NotFound
		if stderrors.As(err, &notFoundErr) {
			slog.WarnContext(ctx, "mailing list not found for deletion - may have been deleted already",
				"subgroup_id", subgroupID)
			return nil // Idempotent - not an error if already deleted
		}
		slog.ErrorContext(ctx, "failed to get mailing list by group_id",
			"subgroup_id", subgroupID,
			"error", err)
		return fmt.Errorf("failed to get mailing list by group_id: %w", err)
	}

	slog.InfoContext(ctx, "found mailing list for deletion",
		"mailing_list_uid", mailingList.UID,
		"service_uid", mailingList.ServiceUID,
		"group_name", mailingList.GroupName,
		"revision", revision)

	// Step 2: Delete mailing list from NATS KV with optimistic concurrency control
	err = p.mailingListWriter.DeleteGrpsIOMailingList(ctx, mailingList.UID, revision, mailingList)
	if err != nil {
		return handleDeleteError(ctx, err, "mailing list", mailingList.UID, revision)
	}

	slog.InfoContext(ctx, "successfully deleted mailing list",
		"mailing_list_uid", mailingList.UID,
		"service_uid", mailingList.ServiceUID,
		"subgroup_id", subgroupID)

	// TODO: NATS Publishing PR #3
	// 1. Check if this was the last subgroup for EnabledServices event
	//    Call: p.mailingListReader.ListMailingListsByServiceUID(ctx, mailingList.ServiceUID)
	// 2. Publish member events to NATS using full MemberInfo (see model/grpsio_webhook_event.go TODOs)
	// 3. Ensure downstream consumers (Zoom, Query Service) receive all required fields

	return nil
}

func (p *grpsIOWebhookProcessor) handleMemberAdded(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.MemberInfo == nil {
		return errors.NewValidation("missing member info in added_member event")
	}

	memberID := int64(event.MemberInfo.ID)
	groupID := int64(event.MemberInfo.GroupID)
	email := event.MemberInfo.Email
	status := event.MemberInfo.Status

	slog.InfoContext(ctx, "received added_member event",
		"member_id", memberID,
		"group_id", groupID,
		"email", redaction.RedactEmail(email),
		"status", status)

	// Step 1: Find mailing list by group_id
	// Pattern: Same as handleSubGroupCreated finds service by parent_group_id
	mailingList, _, err := p.mailingListReader.GetMailingListByGroupID(ctx, uint64(groupID))
	if err != nil {
		var notFoundErr errors.NotFound
		if stderrors.As(err, &notFoundErr) {
			slog.WarnContext(ctx, "mailing list not found for parent group_id - member will not be adopted",
				"group_id", groupID,
				"email", redaction.RedactEmail(email))
			return nil // Not an error - member just won't be adopted
		}
		slog.ErrorContext(ctx, "failed to get mailing list by group_id",
			"group_id", groupID,
			"error", err)
		return fmt.Errorf("failed to get mailing list by group_id: %w", err)
	}

	slog.InfoContext(ctx, "found mailing list for member",
		"mailing_list_uid", mailingList.UID,
		"group_name", mailingList.GroupName)

	// Step 2: Prepare member for creation (idempotency handled by orchestrator)
	// Pattern: Same as handleSubGroupCreated builds MailingList model
	firstName, lastName := parseNameFromEmail(email)

	member := &model.GrpsIOMember{
		MailingListUID: mailingList.UID,
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		Status:         status,
		MemberID:       &memberID,
		GroupID:        &groupID,
		Source:         constants.SourceWebhook, // Critical for source dispatch
	}

	// Note: UID, CreatedAt, UpdatedAt, and validation are handled by orchestrator
	// Note: Idempotency check moved to orchestrator (ensureMemberIdempotent)

	// Step 3: Create member (orchestrator handles all validation and idempotency)
	// Pattern: Same as calling CreateGrpsIOMailingList
	createdMember, _, err := p.memberWriter.CreateGrpsIOMember(ctx, member)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create member",
			"error", err,
			"email", redaction.RedactEmail(email),
			"mailing_list_uid", mailingList.UID)
		return fmt.Errorf("failed to create member: %w", err)
	}

	slog.InfoContext(ctx, "successfully adopted member",
		"member_uid", createdMember.UID,
		"mailing_list_uid", mailingList.UID,
		"email", redaction.RedactEmail(email),
		"member_id", memberID)

	return nil
}

func (p *grpsIOWebhookProcessor) handleMemberRemoved(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.MemberInfo == nil {
		return errors.NewValidation("missing member info in removed_member event")
	}

	memberID := uint64(event.MemberInfo.ID)
	email := event.MemberInfo.Email

	slog.InfoContext(ctx, "received removed_member event",
		"member_id", memberID,
		"email", redaction.RedactEmail(email))

	// Step 1: Find member by Groups.io member ID
	// Pattern: Same as handleSubGroupDeleted finds mailing list by subgroup_id
	member, revision, err := p.memberReader.GetMemberByGroupsIOMemberID(ctx, memberID)
	if err != nil {
		var notFoundErr errors.NotFound
		if stderrors.As(err, &notFoundErr) {
			slog.WarnContext(ctx, "member not found for deletion - may have been deleted already",
				"member_id", memberID)
			return nil // Idempotent - not an error if already deleted
		}
		slog.ErrorContext(ctx, "failed to get member by Groups.io member ID",
			"member_id", memberID,
			"error", err)
		return fmt.Errorf("failed to get member by Groups.io member ID: %w", err)
	}

	slog.InfoContext(ctx, "found member for deletion",
		"member_uid", member.UID,
		"email", redaction.RedactEmail(email),
		"revision", revision)

	// Step 2: Delete member with optimistic concurrency control
	// Pattern: Same as DeleteGrpsIOMailingList
	err = p.memberWriter.DeleteGrpsIOMember(ctx, member.UID, revision, member)
	if err != nil {
		return handleDeleteError(ctx, err, "member", member.UID, revision)
	}

	slog.InfoContext(ctx, "successfully deleted member",
		"member_uid", member.UID,
		"email", redaction.RedactEmail(email),
		"member_id", memberID)

	return nil
}

func (p *grpsIOWebhookProcessor) handleMemberBanned(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.MemberInfo == nil {
		return errors.NewValidation("missing member info in ban_members event")
	}

	slog.InfoContext(ctx, "received ban_members event",
		"member_id", event.MemberInfo.ID,
		"email", redaction.RedactEmail(event.MemberInfo.Email))

	// Banning is equivalent to removal - reuse removal logic
	slog.InfoContext(ctx, "treating banned member as removed")

	return p.handleMemberRemoved(ctx, event)
}

// parseNameFromEmail extracts a reasonable name from email address
// Example: "john.doe@example.com" -> ("John", "Doe")
func parseNameFromEmail(email string) (firstName, lastName string) {
	// Split on @ to get local part
	parts := strings.Split(email, "@")
	if len(parts) < 2 || parts[0] == "" {
		return "Unknown", ""
	}

	localPart := parts[0]

	// Try splitting on dots, underscores, hyphens, or plus signs
	nameParts := strings.FieldsFunc(localPart, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == '+'
	})

	// Use cases.Title from golang.org/x/text instead of deprecated strings.Title
	caser := cases.Title(language.Und)
	if len(nameParts) >= 2 {
		return caser.String(nameParts[0]), caser.String(nameParts[1])
	}
	if len(nameParts) == 1 {
		return caser.String(nameParts[0]), ""
	}

	// Fallback: use local part but don't leak full email
	return caser.String(localPart), ""
}

// Helper methods

// findAdoptingService determines which service should adopt a subgroup
// Priority: prefix-matching service > primary service
func (p *grpsIOWebhookProcessor) findAdoptingService(ctx context.Context, services []*model.GrpsIOService, subgroupSuffix string) *model.GrpsIOService {
	var primaryService *model.GrpsIOService

	// First pass: look for prefix match
	for _, service := range services {
		// Track primary service as fallback
		if service.Type == constants.ServiceTypePrimary {
			primaryService = service
		}

		// Check if service prefix matches subgroup suffix
		if service.Prefix != "" && strings.HasPrefix(subgroupSuffix, service.Prefix) {
			slog.InfoContext(ctx, "found prefix-matching service",
				"service_uid", service.UID,
				"service_type", service.Type,
				"service_prefix", service.Prefix,
				"subgroup_suffix", subgroupSuffix)
			return service // Prefix match takes precedence
		}
	}

	// If no prefix match, use primary service as fallback
	if primaryService != nil {
		slog.InfoContext(ctx, "using primary service (no prefix match)",
			"service_uid", primaryService.UID,
			"service_type", primaryService.Type)
		return primaryService
	}

	return nil
}

// handleDeleteError handles common error patterns for delete operations
// Returns nil for NotFound (idempotent), specific error for Conflict, and wraps other errors
func handleDeleteError(ctx context.Context, err error, entityType, entityID string, revision uint64) error {
	var notFoundErr errors.NotFound
	var conflictErr errors.Conflict

	if stderrors.As(err, &notFoundErr) {
		slog.WarnContext(ctx, "entity not found during deletion - may have been deleted already",
			"entity_type", entityType,
			"entity_id", entityID)
		return nil // Idempotent
	}

	if stderrors.As(err, &conflictErr) {
		slog.ErrorContext(ctx, "revision mismatch during deletion - concurrent modification detected",
			"entity_type", entityType,
			"entity_id", entityID,
			"expected_revision", revision,
			"error", err)
		return fmt.Errorf("revision mismatch during %s deletion: %w", entityType, err)
	}

	slog.ErrorContext(ctx, "failed to delete entity",
		"entity_type", entityType,
		"entity_id", entityID,
		"error", err)
	return fmt.Errorf("failed to delete %s: %w", entityType, err)
}
