// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service implements the mailing list service business logic and endpoints.
package service

import (
	"context"
	"fmt"
	"log/slog"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"

	"github.com/google/uuid"
	"goa.design/goa/v3/security"
)

// mailingListService is the implementation of the mailing list service.
type mailingListService struct {
	auth                     port.Authenticator
	grpsIOReaderOrchestrator service.GrpsIOServiceReader
	grpsIOWriterOrchestrator service.GrpsIOWriter
	storage                  port.GrpsIOReaderWriter
}

// NewMailingList returns the mailing list service implementation.
func NewMailingList(auth port.Authenticator, grpsIOReaderOrchestrator service.GrpsIOServiceReader, grpsIOWriterOrchestrator service.GrpsIOWriter, storage port.GrpsIOReaderWriter) mailinglistservice.Service {
	return &mailingListService{
		auth:                     auth,
		grpsIOReaderOrchestrator: grpsIOReaderOrchestrator,
		grpsIOWriterOrchestrator: grpsIOWriterOrchestrator,
		storage:                  storage,
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
	goaService := s.convertDomainToStandardResponse(service)

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
func (s *mailingListService) CreateGrpsioService(ctx context.Context, payload *mailinglistservice.CreateGrpsioServicePayload) (result *mailinglistservice.ServiceFull, err error) {
	slog.DebugContext(ctx, "mailingListService.create-grpsio-service", "service_type", payload.Type)

	// Validate type-specific requirements
	if err := validateServiceCreationRules(payload); err != nil {
		slog.WarnContext(ctx, "service creation validation failed", "error", err, "service_type", payload.Type)
		return nil, wrapError(ctx, err)
	}

	// Generate new UID for the service
	serviceUID := uuid.New().String()

	// Convert GOA payload to domain model
	domainService := s.convertCreatePayloadToDomain(payload)
	domainService.UID = serviceUID

	// Execute use case
	createdService, revision, err := s.grpsIOWriterOrchestrator.CreateGrpsIOService(ctx, domainService)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create service", "error", err, "service_type", payload.Type)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertDomainToFullResponse(createdService)

	slog.InfoContext(ctx, "successfully created service", "service_uid", createdService.UID, "revision", revision)
	return result, nil
}

// UpdateGrpsioService updates an existing GroupsIO service
func (s *mailingListService) UpdateGrpsioService(ctx context.Context, payload *mailinglistservice.UpdateGrpsioServicePayload) (result *mailinglistservice.ServiceWithReadonlyAttributes, err error) {
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
	domainService := s.convertUpdatePayloadToDomain(existingService, payload)

	// Execute use case
	updatedService, revision, err := s.grpsIOWriterOrchestrator.UpdateGrpsIOService(ctx, *payload.UID, domainService, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update service", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertDomainToStandardResponse(updatedService)

	slog.InfoContext(ctx, "successfully updated service", "service_uid", payload.UID, "revision", revision)
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
	err = s.grpsIOWriterOrchestrator.DeleteGrpsIOService(ctx, *payload.UID, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete service", "error", err, "service_uid", payload.UID)
		return wrapError(ctx, err)
	}

	slog.InfoContext(ctx, "successfully deleted service", "service_uid", payload.UID, "service_type", existingService.Type)
	return nil
}

// CreateGrpsioMailingList creates a new GroupsIO mailing list with comprehensive validation
func (s *mailingListService) CreateGrpsioMailingList(ctx context.Context, payload *mailinglistservice.CreateGrpsioMailingListPayload) (result *mailinglistservice.MailingListFull, err error) {
	slog.DebugContext(ctx, "mailingListService.create-grpsio-mailing-list", "group_name", payload.GroupName, "service_uid", payload.ServiceUID)

	// Validate mailing list creation requirements
	if err := validateMailingListCreation(payload); err != nil {
		slog.WarnContext(ctx, "mailing list creation validation failed", "error", err, "group_name", payload.GroupName)
		return nil, wrapError(ctx, err)
	}

	// Generate new UID for the mailing list
	mailingListUID := uuid.New().String()

	// Convert GOA payload to domain model
	domainMailingList := s.convertMailingListPayloadToDomain(payload)
	domainMailingList.UID = mailingListUID

	// Execute use case
	createdMailingList, err := s.grpsIOWriterOrchestrator.CreateGrpsIOMailingList(ctx, domainMailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list", "error", err, "group_name", payload.GroupName)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertMailingListDomainToResponse(createdMailingList)

	slog.InfoContext(ctx, "successfully created mailing list", "mailing_list_uid", createdMailingList.UID, "group_name", createdMailingList.GroupName, "project_uid", createdMailingList.ProjectUID)
	return result, nil
}

// Helper functions

// payloadStringValue safely extracts string value from payload pointer
func payloadStringValue(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}

// payloadInt64Value safely extracts int64 value from payload pointer
func payloadInt64Value(val *int64) int64 {
	if val == nil {
		return 0
	}
	return *val
}
