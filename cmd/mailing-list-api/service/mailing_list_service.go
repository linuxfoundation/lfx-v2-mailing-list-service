// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service implements the mailing list service business logic and endpoints.
package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"

	"github.com/google/uuid"
	"goa.design/goa/v3/security"
)

// mailingListService is the implementation of the mailing list service.
type mailingListService struct {
	auth                     port.Authenticator
	grpsIOReaderOrchestrator service.GrpsIOReader
	grpsIOWriterOrchestrator service.GrpsIOWriter
	storage                  port.GrpsIOReaderWriter

	// GroupsIO Webhook dependencies
	grpsioWebhookValidator port.GrpsIOWebhookValidator
	grpsioWebhookProcessor port.GrpsIOWebhookProcessor
}

// NewMailingList returns the mailing list service implementation.
func NewMailingList(
	auth port.Authenticator,
	grpsIOReaderOrchestrator service.GrpsIOReader,
	grpsIOWriterOrchestrator service.GrpsIOWriter,
	storage port.GrpsIOReaderWriter,
	grpsioWebhookValidator port.GrpsIOWebhookValidator,
	grpsioWebhookProcessor port.GrpsIOWebhookProcessor,
) mailinglistservice.Service {
	return &mailingListService{
		auth:                     auth,
		grpsIOReaderOrchestrator: grpsIOReaderOrchestrator,
		grpsIOWriterOrchestrator: grpsIOWriterOrchestrator,
		storage:                  storage,
		grpsioWebhookValidator:   grpsioWebhookValidator,
		grpsioWebhookProcessor:   grpsioWebhookProcessor,
	}
}

// JWTAuth implements the authorization logic for service "mailing-list"
// for the "jwt" security scheme.
func (s *mailingListService) JWTAuth(ctx context.Context, token string, _ *security.JWTScheme) (context.Context, error) {
	// Parse the Heimdall-authorized principal from the token
	principal, err := s.auth.ParsePrincipal(ctx, token, slog.Default())
	if err != nil {
		return ctx, err
	}

	// Return a new context containing the principal as a value
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}

// Livez implements the livez endpoint for liveness probes.
func (s *mailingListService) Livez(ctx context.Context) ([]byte, error) {
	slog.DebugContext(ctx, "liveness check completed successfully")
	return []byte("OK"), nil
}

// Readyz implements the readyz endpoint for readiness probes.
func (s *mailingListService) Readyz(ctx context.Context) ([]byte, error) {
	// Check NATS readiness
	if err := s.storage.IsReady(ctx); err != nil {
		slog.ErrorContext(ctx, "service not ready", "error", err)
		return nil, err // This will automatically return ServiceUnavailable
	}

	return []byte("OK\n"), nil
}

// GetGrpsioService retrieves a single service by ID
func (s *mailingListService) GetGrpsioService(ctx context.Context, payload *mailinglistservice.GetGrpsioServicePayload) (result *mailinglistservice.GetGrpsioServiceResult, err error) {
	slog.DebugContext(ctx, "mailingListService.get-grpsio-service", "service_uid", payload.UID)

	// Execute use case
	service, revision, err := s.grpsIOReaderOrchestrator.GetGrpsIOService(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get service", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	goaService := s.convertGrpsIOServiceDomainToStandardResponse(service)

	// Create result with ETag (using revision from NATS)
	revisionStr := fmt.Sprintf("%d", revision)
	result = &mailinglistservice.GetGrpsioServiceResult{
		Service: goaService,
		Etag:    &revisionStr,
	}

	slog.InfoContext(ctx, "successfully retrieved service", "service_uid", payload.UID, "etag", revisionStr)
	return result, nil
}

// CreateGrpsioService creates a new GroupsIO service with type-specific validation
func (s *mailingListService) CreateGrpsioService(ctx context.Context, payload *mailinglistservice.CreateGrpsioServicePayload) (result *mailinglistservice.GrpsIoServiceFull, err error) {
	slog.DebugContext(ctx, "mailingListService.create-grpsio-service", "service_type", payload.Type)

	// Validate type-specific requirements
	if err := validateServiceCreationRules(payload); err != nil {
		slog.WarnContext(ctx, "service creation validation failed", "error", err, "service_type", payload.Type)
		return nil, wrapError(ctx, err)
	}

	// Generate new UID for the service
	serviceUID := uuid.New().String()

	// Convert GOA payload to domain model
	domainService := s.convertGrpsIOServiceCreatePayloadToDomain(payload)
	domainService.UID = serviceUID

	// Extract settings from payload (writers/auditors)
	domainSettings := s.convertGrpsIOServiceCreatePayloadToSettings(payload, serviceUID)

	// Execute use case
	createdService, createdSettings, revision, err := s.grpsIOWriterOrchestrator.CreateGrpsIOService(ctx, domainService, domainSettings)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create service", "error", err, "service_type", payload.Type)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response, including settings
	result = s.convertGrpsIOServiceDomainToFullResponse(createdService, createdSettings)

	slog.InfoContext(ctx, "successfully created service", "service_uid", createdService.UID, "revision", revision)
	return result, nil
}

// UpdateGrpsioService updates an existing GroupsIO service
func (s *mailingListService) UpdateGrpsioService(ctx context.Context, payload *mailinglistservice.UpdateGrpsioServicePayload) (result *mailinglistservice.GrpsIoServiceWithReadonlyAttributes, err error) {
	slog.DebugContext(ctx, "mailingListService.update-grpsio-service", "service_uid", payload.UID)

	// Parse expected revision from ETag
	expectedRevision, err := etagValidator(payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid if-match", "error", err, "if_match", payload.IfMatch)
		return nil, wrapError(ctx, err)
	}

	// Retrieve existing service for immutability validation
	existingService, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOService(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing service for update validation", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Validate immutability constraints
	if err := validateUpdateImmutabilityConstraints(existingService, payload); err != nil {
		slog.WarnContext(ctx, "update validation failed due to immutability constraints", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert GOA payload to domain model
	domainService := s.convertGrpsIOServiceUpdatePayloadToDomain(existingService, payload)

	// Enhanced business rule validation (POST-PUT conversion)
	// This prevents PUT semantics from violating mandatory business constraints
	if err := validateServiceBusinessRules(domainService); err != nil {
		slog.WarnContext(ctx, "business rule validation failed after payload conversion",
			"error", err,
			"service_uid", payload.UID,
			"service_type", domainService.Type)
		return nil, wrapError(ctx, err)
	}

	// Execute use case
	updatedService, revision, err := s.grpsIOWriterOrchestrator.UpdateGrpsIOService(ctx, *payload.UID, domainService, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update service", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertGrpsIOServiceDomainToStandardResponse(updatedService)

	slog.InfoContext(ctx, "successfully updated service", "service_uid", payload.UID, "revision", revision)
	return result, nil
}

// GetGrpsioServiceSettings retrieves service settings (writers and auditors)
func (s *mailingListService) GetGrpsioServiceSettings(ctx context.Context, payload *mailinglistservice.GetGrpsioServiceSettingsPayload) (result *mailinglistservice.GetGrpsioServiceSettingsResult, err error) {
	slog.DebugContext(ctx, "mailingListService.get-grpsio-service-settings", "service_uid", payload.UID)

	// Execute use case
	settings, revision, err := s.grpsIOReaderOrchestrator.GetGrpsIOServiceSettings(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get service settings", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	goaSettings := s.convertGrpsIOServiceSettingsDomainToResponse(settings)

	// Create result with ETag (using revision from NATS)
	revisionStr := fmt.Sprintf("%d", revision)
	result = &mailinglistservice.GetGrpsioServiceSettingsResult{
		ServiceSettings: goaSettings,
		Etag:            &revisionStr,
	}

	slog.InfoContext(ctx, "successfully retrieved service settings", "service_uid", payload.UID, "etag", revisionStr)
	return result, nil
}

// UpdateGrpsioServiceSettings updates service settings (writers and auditors)
func (s *mailingListService) UpdateGrpsioServiceSettings(ctx context.Context, payload *mailinglistservice.UpdateGrpsioServiceSettingsPayload) (result *mailinglistservice.GrpsIoServiceSettings, err error) {
	slog.DebugContext(ctx, "mailingListService.update-grpsio-service-settings", "service_uid", payload.UID)

	// Parse expected revision from ETag
	expectedRevision, err := etagValidator(payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid if-match", "error", err, "if_match", payload.IfMatch)
		return nil, wrapError(ctx, err)
	}

	// Convert GOA payload to domain model
	domainSettings := s.convertGrpsIOServiceSettingsPayloadToDomain(payload)

	// Execute use case
	updatedSettings, revision, err := s.grpsIOWriterOrchestrator.UpdateGrpsIOServiceSettings(ctx, domainSettings, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update service settings", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertGrpsIOServiceSettingsDomainToResponse(updatedSettings)

	slog.InfoContext(ctx, "successfully updated service settings", "service_uid", payload.UID, "revision", revision)
	return result, nil
}

// DeleteGrpsioService deletes a GroupsIO service
func (s *mailingListService) DeleteGrpsioService(ctx context.Context, payload *mailinglistservice.DeleteGrpsioServicePayload) (err error) {
	slog.DebugContext(ctx, "mailingListService.delete-grpsio-service", "service_uid", payload.UID)

	// Validate ETag
	expectedRevision, err := etagValidator(payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid if-match", "error", err, "if_match", payload.IfMatch)
		return wrapError(ctx, err)
	}

	// Retrieve existing service for deletion protection validation
	existingService, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOService(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing service for delete validation", "error", err, "service_uid", payload.UID)
		return wrapError(ctx, err)
	}

	// Validate deletion protection rules
	if err := validateDeleteProtectionRules(existingService); err != nil {
		slog.WarnContext(ctx, "delete validation failed due to protection rules", "error", err, "service_uid", payload.UID, "service_type", existingService.Type)
		return wrapError(ctx, err)
	}

	// Execute use case
	err = s.grpsIOWriterOrchestrator.DeleteGrpsIOService(ctx, *payload.UID, expectedRevision, existingService)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete service", "error", err, "service_uid", payload.UID)
		return wrapError(ctx, err)
	}

	slog.InfoContext(ctx, "successfully deleted service", "service_uid", payload.UID, "service_type", existingService.Type)
	return nil
}

// CreateGrpsioMailingList creates a new GroupsIO mailing list with comprehensive validation
func (s *mailingListService) CreateGrpsioMailingList(ctx context.Context, payload *mailinglistservice.CreateGrpsioMailingListPayload) (result *mailinglistservice.GrpsIoMailingListFull, err error) {
	slog.DebugContext(ctx, "mailingListService.create-grpsio-mailing-list", "group_name", payload.GroupName, "service_uid", payload.ServiceUID)

	// Validate mailing list creation requirements
	if err := validateMailingListCreation(payload); err != nil {
		slog.WarnContext(ctx, "mailing list creation validation failed", "error", err, "group_name", payload.GroupName)
		return nil, wrapError(ctx, err)
	}

	// Generate new UID for the mailing list
	mailingListUID := uuid.New().String()

	// Convert GOA payload to domain model
	domainMailingList := s.convertGrpsIOMailingListPayloadToDomain(payload)
	domainMailingList.UID = mailingListUID

	// Extract writers and auditors from payload and create settings
	domainSettings := &model.GrpsIOMailingListSettings{
		Writers:  convertUserInfoPayloadToDomain(payload.Writers),
		Auditors: convertUserInfoPayloadToDomain(payload.Auditors),
	}

	// Execute use case
	createdMailingList, revision, err := s.grpsIOWriterOrchestrator.CreateGrpsIOMailingList(ctx, domainMailingList, domainSettings)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list", "error", err, "group_name", payload.GroupName)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response with settings
	result = s.convertGrpsIOMailingListDomainToResponse(createdMailingList, domainSettings)

	slog.InfoContext(ctx, "successfully created mailing list", "mailing_list_uid", createdMailingList.UID, "group_name", createdMailingList.GroupName, "project_uid", createdMailingList.ProjectUID, "revision", revision)
	return result, nil
}

// GetGrpsioMailingList retrieves a single mailing list by UID
func (s *mailingListService) GetGrpsioMailingList(ctx context.Context, payload *mailinglistservice.GetGrpsioMailingListPayload) (result *mailinglistservice.GetGrpsioMailingListResult, err error) {
	slog.DebugContext(ctx, "mailingListService.get-grpsio-mailing-list", "mailing_list_uid", payload.UID)

	// Execute use case
	mailingList, revision, err := s.grpsIOReaderOrchestrator.GetGrpsIOMailingList(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list", "error", err, "mailing_list_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	goaMailingList := s.convertGrpsIOMailingListDomainToStandardResponse(mailingList)

	// Create result with ETag (using revision from NATS)
	revisionStr := fmt.Sprintf("%d", revision)
	result = &mailinglistservice.GetGrpsioMailingListResult{
		MailingList: goaMailingList,
		Etag:        &revisionStr,
	}

	slog.InfoContext(ctx, "successfully retrieved mailing list", "mailing_list_uid", payload.UID, "etag", revisionStr)
	return result, nil
}

// UpdateGrpsioMailingList updates an existing GroupsIO mailing list
func (s *mailingListService) UpdateGrpsioMailingList(ctx context.Context, payload *mailinglistservice.UpdateGrpsioMailingListPayload) (result *mailinglistservice.GrpsIoMailingListWithReadonlyAttributes, err error) {
	slog.DebugContext(ctx, "mailingListService.update-grpsio-mailing-list", "mailing_list_uid", payload.UID)

	// Parse expected revision from ETag
	expectedRevision, err := etagValidator(payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid if-match", "error", err, "if_match", payload.IfMatch)
		return nil, wrapError(ctx, err)
	}

	// Retrieve existing mailing list for validation
	existingMailingList, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOMailingList(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing mailing list for update validation", "error", err, "mailing_list_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Retrieve parent service for validation (needed for main group checks)
	parentService, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOService(ctx, existingMailingList.ServiceUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve parent service for update validation", "error", err, "service_uid", existingMailingList.ServiceUID)
		return nil, wrapError(ctx, err)
	}

	// Validate update constraints
	if err := validateMailingListUpdate(ctx, existingMailingList, parentService, payload, s.grpsIOReaderOrchestrator); err != nil {
		slog.WarnContext(ctx, "update validation failed",
			"error", err,
			"mailing_list_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert GOA payload to domain model
	domainMailingList := s.convertGrpsIOMailingListUpdatePayloadToDomain(existingMailingList, payload)
	// Ensure persisted JSON UID matches the key
	if payload.UID != nil {
		domainMailingList.UID = *payload.UID
	}

	// Execute use case
	updatedMailingList, revision, err := s.grpsIOWriterOrchestrator.UpdateGrpsIOMailingList(ctx, *payload.UID, domainMailingList, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update mailing list", "error", err, "mailing_list_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertGrpsIOMailingListDomainToStandardResponse(updatedMailingList)

	slog.InfoContext(ctx, "successfully updated mailing list", "mailing_list_uid", payload.UID, "revision", revision)
	return result, nil
}

// DeleteGrpsioMailingList deletes a GroupsIO mailing list
func (s *mailingListService) DeleteGrpsioMailingList(ctx context.Context, payload *mailinglistservice.DeleteGrpsioMailingListPayload) (err error) {
	slog.DebugContext(ctx, "mailingListService.delete-grpsio-mailing-list", "mailing_list_uid", payload.UID)

	// Validate ETag
	expectedRevision, err := etagValidator(payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid if-match", "error", err, "if_match", payload.IfMatch)
		return wrapError(ctx, err)
	}

	// Retrieve existing mailing list for deletion protection validation
	existingMailingList, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOMailingList(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve existing mailing list for delete validation", "error", err, "mailing_list_uid", payload.UID)
		return wrapError(ctx, err)
	}

	// Retrieve parent service for deletion protection validation
	parentService, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOService(ctx, existingMailingList.ServiceUID)
	if err != nil {
		slog.WarnContext(ctx, "failed to retrieve parent service for delete validation", "error", err, "service_uid", existingMailingList.ServiceUID)
		// Continue with deletion if service not found (service might be deleted)
		parentService = nil
	}

	// Validate deletion protection rules
	if err := validateMailingListDeleteProtection(existingMailingList, parentService); err != nil {
		slog.WarnContext(ctx, "delete validation failed due to protection rules",
			"error", err,
			"mailing_list_uid", payload.UID,
			"group_name", existingMailingList.GroupName)
		return wrapError(ctx, err)
	}

	// Execute use case
	err = s.grpsIOWriterOrchestrator.DeleteGrpsIOMailingList(ctx, *payload.UID, expectedRevision, existingMailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete mailing list", "error", err, "mailing_list_uid", payload.UID)
		return wrapError(ctx, err)
	}

	slog.InfoContext(ctx, "successfully deleted mailing list", "mailing_list_uid", payload.UID, "group_name", existingMailingList.GroupName)
	return nil
}

// GetGrpsioMailingListSettings retrieves mailing list settings (writers and auditors)
func (s *mailingListService) GetGrpsioMailingListSettings(ctx context.Context, payload *mailinglistservice.GetGrpsioMailingListSettingsPayload) (result *mailinglistservice.GetGrpsioMailingListSettingsResult, err error) {
	slog.DebugContext(ctx, "mailingListService.get-grpsio-mailing-list-settings", "mailing_list_uid", payload.UID)

	// Execute use case
	settings, revision, err := s.grpsIOReaderOrchestrator.GetGrpsIOMailingListSettings(ctx, *payload.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list settings", "error", err, "mailing_list_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	goaSettings := s.convertGrpsIOMailingListSettingsDomainToResponse(settings)

	// Create result with ETag (using revision from NATS)
	revisionStr := fmt.Sprintf("%d", revision)
	result = &mailinglistservice.GetGrpsioMailingListSettingsResult{
		MailingListSettings: goaSettings,
		Etag:                &revisionStr,
	}

	slog.InfoContext(ctx, "successfully retrieved mailing list settings", "mailing_list_uid", payload.UID, "etag", revisionStr)
	return result, nil
}

// UpdateGrpsioMailingListSettings updates mailing list settings (writers and auditors)
func (s *mailingListService) UpdateGrpsioMailingListSettings(ctx context.Context, payload *mailinglistservice.UpdateGrpsioMailingListSettingsPayload) (result *mailinglistservice.GrpsIoMailingListSettings, err error) {
	slog.DebugContext(ctx, "mailingListService.update-grpsio-mailing-list-settings", "mailing_list_uid", payload.UID)

	// Parse expected revision from ETag
	expectedRevision, err := etagValidator(payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid if-match", "error", err, "if_match", payload.IfMatch)
		return nil, wrapError(ctx, err)
	}

	// Convert GOA payload to domain model
	domainSettings := s.convertGrpsIOMailingListSettingsPayloadToDomain(payload)

	// Execute use case
	updatedSettings, revision, err := s.grpsIOWriterOrchestrator.UpdateGrpsIOMailingListSettings(ctx, domainSettings, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update mailing list settings", "error", err, "mailing_list_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertGrpsIOMailingListSettingsDomainToResponse(updatedSettings)

	slog.InfoContext(ctx, "successfully updated mailing list settings", "mailing_list_uid", payload.UID, "revision", revision)
	return result, nil
}

// CreateGrpsioMailingListMember creates a new member for a GroupsIO mailing list
func (s *mailingListService) CreateGrpsioMailingListMember(ctx context.Context, payload *mailinglistservice.CreateGrpsioMailingListMemberPayload) (result *mailinglistservice.GrpsIoMemberFull, err error) {
	slog.DebugContext(ctx, "mailingListService.create-grpsio-mailing-list-member",
		"mailing_list_uid", payload.UID,
		"email", redaction.RedactEmail(payload.Email),
	)

	// Validate member creation requirements
	if err := validateMemberCreation(ctx, payload, s.grpsIOReaderOrchestrator); err != nil {
		slog.WarnContext(ctx, "member creation validation failed", "error", err, "email", redaction.RedactEmail(payload.Email))
		return nil, wrapError(ctx, err)
	}

	// Generate new UID for the member
	memberUID := uuid.New().String()

	// Convert GOA payload to domain model
	domainMember := s.convertGrpsIOMemberPayloadToDomain(payload)
	domainMember.UID = memberUID

	// Execute use case
	createdMember, revision, err := s.grpsIOWriterOrchestrator.CreateGrpsIOMember(ctx, domainMember)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create member",
			"error", err,
			"mailing_list_uid", payload.UID,
			"email", redaction.RedactEmail(payload.Email),
		)
		return nil, wrapError(ctx, err)
	}

	// TODO: Future PR - Add Groups.io API integration
	// When Groups.io API integration is complete, add member to Groups.io
	// Handle Groups.io error "user already exists" to detect member adoption scenarios

	// TODO: LFXV2-478 - Add committee sync functionality
	// - Sync committee members when committee_uid is provided
	// - Handle committee member role changes (owner/moderator/member)
	// - Auto-update member status based on committee membership changes

	// Convert domain model to GOA response
	result = s.convertGrpsIOMemberDomainToResponse(createdMember)

	slog.InfoContext(ctx, "successfully created member",
		"member_uid", createdMember.UID,
		"mailing_list_uid", createdMember.MailingListUID,
		"email", redaction.RedactEmail(createdMember.Email),
		"revision", revision,
	)
	return result, nil
}

// GetGrpsioMailingListMember retrieves a member from a GroupsIO mailing list
func (s *mailingListService) GetGrpsioMailingListMember(ctx context.Context, payload *mailinglistservice.GetGrpsioMailingListMemberPayload) (*mailinglistservice.GetGrpsioMailingListMemberResult, error) {
	slog.DebugContext(ctx, "getting GroupsIO mailing list member",
		"mailing_list_uid", payload.UID,
		"member_uid", payload.MemberUID)

	// Get member using reader orchestrator
	member, revision, err := s.grpsIOReaderOrchestrator.GetGrpsIOMember(ctx, payload.MemberUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get member",
			"error", err,
			"member_uid", payload.MemberUID)
		return nil, wrapError(ctx, err)
	}

	// Verify member belongs to the requested mailing list
	if member.MailingListUID != payload.UID {
		slog.WarnContext(ctx, "member does not belong to requested mailing list",
			"member_uid", payload.MemberUID,
			"requested_mailing_list_uid", payload.UID,
			"actual_mailing_list_uid", member.MailingListUID)
		return nil, wrapError(ctx, errors.NewNotFound("member not found in mailing list"))
	}

	// Convert to response format
	memberResponse := s.convertGrpsIOMemberToResponse(member)
	etag := fmt.Sprintf("%d", revision)

	slog.InfoContext(ctx, "successfully retrieved member",
		"member_uid", payload.MemberUID,
		"mailing_list_uid", payload.UID,
		"etag", etag)

	return &mailinglistservice.GetGrpsioMailingListMemberResult{
		Member: memberResponse,
		Etag:   &etag,
	}, nil
}

// UpdateGrpsioMailingListMember updates a member in a GroupsIO mailing list
func (s *mailingListService) UpdateGrpsioMailingListMember(ctx context.Context, payload *mailinglistservice.UpdateGrpsioMailingListMemberPayload) (*mailinglistservice.GrpsIoMemberWithReadonlyAttributes, error) {
	slog.DebugContext(ctx, "updating GroupsIO mailing list member",
		"mailing_list_uid", payload.UID,
		"member_uid", payload.MemberUID)

	// Parse ETag for revision checking
	expectedRevision, err := etagValidator(&payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid ETag format", "error", err, "etag", payload.IfMatch)
		return nil, wrapError(ctx, errors.NewValidation("invalid ETag format", err))
	}

	// Get existing member for validation
	existingMember, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOMember(ctx, payload.MemberUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get existing member",
			"error", err,
			"member_uid", payload.MemberUID)
		return nil, wrapError(ctx, err)
	}

	// Verify member belongs to the requested mailing list
	if existingMember.MailingListUID != payload.UID {
		slog.WarnContext(ctx, "member does not belong to requested mailing list",
			"member_uid", payload.MemberUID,
			"requested_mailing_list_uid", payload.UID,
			"actual_mailing_list_uid", existingMember.MailingListUID)
		return nil, wrapError(ctx, errors.NewNotFound("member not found in mailing list"))
	}

	// Build updated member model from payload
	updatedMember := s.convertGrpsIOMemberUpdatePayloadToDomain(payload, existingMember)

	// Validate immutable fields
	if err := validateMemberUpdate(existingMember, updatedMember); err != nil {
		slog.ErrorContext(ctx, "member update validation failed",
			"error", err,
			"member_uid", payload.MemberUID)
		return nil, wrapError(ctx, err)
	}

	// Update member via writer orchestrator with revision check
	updated, revision, err := s.grpsIOWriterOrchestrator.UpdateGrpsIOMember(ctx, payload.MemberUID, updatedMember, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update member",
			"error", err,
			"member_uid", payload.MemberUID)
		return nil, wrapError(ctx, err)
	}

	// TODO: LFXV2-353 - Add Groups.io API sync for member updates
	// When Groups.io API integration is complete, sync member changes to Groups.io
	// Handle modStatus changes (owner/moderator/member role updates)

	// TODO: LFXV2-481 - Add member notification for profile changes
	// Notify member when profile information is updated by moderators
	// Send email notification for role changes (promotion to owner/moderator)

	// Convert to response format
	memberResponse := s.convertGrpsIOMemberToResponse(updated)

	slog.InfoContext(ctx, "successfully updated member",
		"member_uid", payload.MemberUID,
		"mailing_list_uid", payload.UID,
		"revision", revision)

	return memberResponse, nil
}

// DeleteGrpsioMailingListMember deletes a member from a GroupsIO mailing list
func (s *mailingListService) DeleteGrpsioMailingListMember(ctx context.Context, payload *mailinglistservice.DeleteGrpsioMailingListMemberPayload) error {
	slog.DebugContext(ctx, "deleting GroupsIO mailing list member",
		"mailing_list_uid", payload.UID,
		"member_uid", payload.MemberUID)

	// Parse ETag for revision checking
	expectedRevision, err := etagValidator(&payload.IfMatch)
	if err != nil {
		slog.ErrorContext(ctx, "invalid ETag format", "error", err, "etag", payload.IfMatch)
		return wrapError(ctx, errors.NewValidation("invalid ETag format"))
	}

	// Get existing member for validation
	existingMember, _, err := s.grpsIOReaderOrchestrator.GetGrpsIOMember(ctx, payload.MemberUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get existing member",
			"error", err,
			"member_uid", payload.MemberUID)
		return wrapError(ctx, err)
	}

	// Verify member belongs to the requested mailing list
	if existingMember.MailingListUID != payload.UID {
		slog.WarnContext(ctx, "member does not belong to requested mailing list",
			"member_uid", payload.MemberUID,
			"requested_mailing_list_uid", payload.UID,
			"actual_mailing_list_uid", existingMember.MailingListUID)
		return wrapError(ctx, errors.NewNotFound("member not found in mailing list"))
	}

	// Validate member deletion protection rules
	if err := validateMemberDeleteProtection(existingMember); err != nil {
		slog.WarnContext(ctx, "member deletion protection failed", "error", err, "member_uid", payload.MemberUID)
		return wrapError(ctx, err)
	}

	// TODO: Future PR - Check sole owner protection via Groups.io API
	// Prevent deletion if this is the only owner/moderator of the mailing list

	// Delete member via writer orchestrator with revision check
	err = s.grpsIOWriterOrchestrator.DeleteGrpsIOMember(ctx, payload.MemberUID, expectedRevision, existingMember)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete member",
			"error", err,
			"member_uid", payload.MemberUID)
		return wrapError(ctx, err)
	}

	slog.InfoContext(ctx, "successfully deleted member",
		"member_uid", payload.MemberUID,
		"mailing_list_uid", payload.UID)

	return nil
}

// GroupsioWebhook handles GroupsIO webhook events
func (s *mailingListService) GroupsioWebhook(ctx context.Context, p *mailinglistservice.GroupsioWebhookPayload) error {
	// Get raw body from context
	bodyBytes, ok := ctx.Value(constants.GrpsIOWebhookBodyContextKey).([]byte)
	if !ok {
		slog.ErrorContext(ctx, "failed to get raw body from context")
		return &mailinglistservice.BadRequestError{Message: "missing webhook body"}
	}

	// Validate signature
	if err := s.grpsioWebhookValidator.ValidateSignature(bodyBytes, p.Signature); err != nil {
		slog.ErrorContext(ctx, "webhook signature validation failed", "error", err)
		return &mailinglistservice.UnauthorizedError{Message: "invalid webhook signature"}
	}

	// GOA already parsed the action field - use it directly
	slog.InfoContext(ctx, "processing groupsio webhook event", "event_type", p.Action)

	// Validate event type
	if !s.grpsioWebhookValidator.IsValidEvent(p.Action) {
		slog.WarnContext(ctx, "unsupported event type", "event_type", p.Action)
		return &mailinglistservice.BadRequestError{Message: fmt.Sprintf("unsupported event type: %s", p.Action)}
	}

	// Convert GOA payload to domain model
	event := &model.GrpsIOWebhookEvent{
		Action:     p.Action,
		ReceivedAt: time.Now(),
	}

	// Convert Group field if present (for subgroup events)
	if p.Group != nil {
		if groupMap, ok := p.Group.(map[string]any); ok {
			convertedGroup, err := s.convertWebhookGroupInfo(groupMap)
			if err != nil {
				slog.ErrorContext(ctx, "invalid group data in webhook", "error", err)
				return &mailinglistservice.BadRequestError{Message: fmt.Sprintf("invalid group data: %v", err)}
			}
			event.Group = convertedGroup
		}
	}

	// Convert MemberInfo field if present (for member events)
	if p.MemberInfo != nil {
		if memberMap, ok := p.MemberInfo.(map[string]any); ok {
			convertedMember, err := s.convertWebhookMemberInfo(memberMap)
			if err != nil {
				slog.ErrorContext(ctx, "invalid member data in webhook", "error", err)
				return &mailinglistservice.BadRequestError{Message: fmt.Sprintf("invalid member data: %v", err)}
			}
			event.MemberInfo = convertedMember
		}
	}

	// Map extra fields
	if p.Extra != nil {
		event.Extra = *p.Extra
	}
	if p.ExtraID != nil {
		event.ExtraID = *p.ExtraID
	}

	// Process event synchronously with exponential backoff retries
	retryConfig := utils.NewRetryConfig(
		constants.WebhookMaxRetries,
		constants.WebhookRetryBaseDelay*time.Millisecond,
		constants.WebhookRetryMaxDelay*time.Millisecond,
	)
	err := utils.RetryWithExponentialBackoff(ctx, retryConfig, func() error {
		return s.grpsioWebhookProcessor.ProcessEvent(ctx, event)
	})

	if err != nil {
		// Check if this is a validation error (malformed data)
		var validationErr errors.Validation
		if stderrors.As(err, &validationErr) {
			// Validation errors should not trigger retries - log and return success
			slog.ErrorContext(ctx, "webhook validation failed - returning success to prevent retries",
				"error", err,
				"action", p.Action)
			return nil // Return 204 to prevent GroupsIO from retrying
		}

		// For other errors (transient failures), return error to trigger GroupsIO retry
		slog.ErrorContext(ctx, "webhook processing failed after retries",
			"error", err,
			"action", p.Action,
			"retries", constants.WebhookMaxRetries)
		return &mailinglistservice.InternalServerError{Message: "webhook processing failed"}
	}

	// Success - return 204 No Content
	return nil
}

// Helper functions

// payloadStringValue safely extracts string value from payload pointer
func payloadStringValue(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}

// payloadInt64Ptr safely converts int64 pointer from payload to nullable pointer for domain model
func payloadInt64Ptr(val *int64) *int64 {
	if val == nil {
		return nil
	}
	return val
}
