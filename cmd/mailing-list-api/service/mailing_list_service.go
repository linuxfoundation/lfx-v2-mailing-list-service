// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service implements the mailing list service business logic and endpoints.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"

	"github.com/google/uuid"
	"goa.design/goa/v3/security"
)

// mailingListService is the implementation of the mailing list service.
type mailingListService struct {
	auth                            port.Authenticator
	grpsIOServiceReaderOrchestrator service.GrpsIOServiceReader
	grpsIOServiceWriterOrchestrator service.GrpsIOServiceWriter
	storage                         port.GrpsIOServiceReaderWriter
}

// NewMailingList returns the mailing list service implementation.
func NewMailingList(auth port.Authenticator, grpsIOServiceReaderOrchestrator service.GrpsIOServiceReader, grpsIOServiceWriterOrchestrator service.GrpsIOServiceWriter, storage port.GrpsIOServiceReaderWriter) mailinglistservice.Service {
	return &mailingListService{
		auth:                            auth,
		grpsIOServiceReaderOrchestrator: grpsIOServiceReaderOrchestrator,
		grpsIOServiceWriterOrchestrator: grpsIOServiceWriterOrchestrator,
		storage:                         storage,
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
	service, revision, err := s.grpsIOServiceReaderOrchestrator.GetGrpsIOService(ctx, *payload.UID)
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
	createdService, revision, err := s.grpsIOServiceWriterOrchestrator.CreateGrpsIOService(ctx, domainService)
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
	expectedRevision, err := etagValidator(payload.Etag)
	if err != nil {
		slog.ErrorContext(ctx, "invalid etag", "error", err, "etag", payload.Etag)
		return nil, wrapError(ctx, err)
	}

	// Retrieve existing service for immutability validation
	existingService, _, err := s.grpsIOServiceReaderOrchestrator.GetGrpsIOService(ctx, *payload.UID)
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
	updatedService, revision, err := s.grpsIOServiceWriterOrchestrator.UpdateGrpsIOService(ctx, *payload.UID, domainService, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update service", "error", err, "service_uid", payload.UID)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response using committee service pattern
	result = s.convertDomainToStandardResponse(updatedService)

	slog.InfoContext(ctx, "successfully updated service", "service_uid", payload.UID, "revision", revision)
	return result, nil
}

// DeleteGrpsioService deletes a GroupsIO service
func (s *mailingListService) DeleteGrpsioService(ctx context.Context, payload *mailinglistservice.DeleteGrpsioServicePayload) (err error) {
	slog.DebugContext(ctx, "mailingListService.delete-grpsio-service", "service_uid", payload.UID)

	// Validate ETag using committee service pattern
	expectedRevision, err := etagValidator(payload.Etag)
	if err != nil {
		slog.ErrorContext(ctx, "invalid etag", "error", err, "etag", payload.Etag)
		return wrapError(ctx, err)
	}

	// Retrieve existing service for deletion protection validation
	existingService, _, err := s.grpsIOServiceReaderOrchestrator.GetGrpsIOService(ctx, *payload.UID)
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
	err = s.grpsIOServiceWriterOrchestrator.DeleteGrpsIOService(ctx, *payload.UID, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete service", "error", err, "service_uid", payload.UID)
		return wrapError(ctx, err)
	}

	slog.InfoContext(ctx, "successfully deleted service", "service_uid", payload.UID, "service_type", existingService.Type)
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

// payloadInt64Value safely extracts int64 value from payload pointer
func payloadInt64Value(val *int64) int64 {
	if val == nil {
		return 0
	}
	return *val
}

// validateServiceCreationRules validates type-specific business rules for service creation
func validateServiceCreationRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	serviceType := payload.Type

	switch serviceType {
	case "primary":
		return validatePrimaryRules(payload)
	case "formation":
		return validateFormationRules(payload)
	case "shared":
		return validateSharedRules(payload)
	default:
		return errors.NewValidation(fmt.Sprintf("invalid service type: %s. Must be one of: primary, formation, shared", serviceType))
	}
}

// validatePrimaryRules validates rules for primary service type
func validatePrimaryRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	// primary rules:
	// - prefix must NOT be provided (will return 400 error)
	// - global_owners must be provided and contain at least one valid email
	// - No existing non-formation service for the project (TODO: implement project validation)

	if payload.Prefix != nil && *payload.Prefix != "" {
		return errors.NewValidation("prefix must not be provided for primary service type")
	}

	// global_owners is required for primary services
	if len(payload.GlobalOwners) == 0 {
		return errors.NewValidation("global_owners is required and must contain at least one email address for primary service type")
	}

	// Validate global_owners email addresses
	if err := validateEmailAddresses(payload.GlobalOwners, "global_owners"); err != nil {
		return err
	}

	return nil
}

// validateFormationRules validates rules for formation service type
func validateFormationRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	// formation rules:
	// - prefix must be non-empty string

	if payload.Prefix == nil || strings.TrimSpace(*payload.Prefix) == "" {
		return errors.NewValidation("prefix is required and must be non-empty for formation service type")
	}

	// Validate global_owners email addresses if provided
	if err := validateEmailAddresses(payload.GlobalOwners, "global_owners"); err != nil {
		return err
	}

	return nil
}

// validateSharedRules validates rules for shared service type
func validateSharedRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	// shared rules:
	// - prefix must be non-empty string
	// - group_id must be valid Groups.io group ID
	// - global_owners must NOT be provided (will return 400 error)

	if payload.Prefix == nil || strings.TrimSpace(*payload.Prefix) == "" {
		return errors.NewValidation("prefix is required and must be non-empty for shared service type")
	}

	if payload.GroupID == nil || *payload.GroupID <= 0 {
		return errors.NewValidation("group_id is required and must be a valid Groups.io group ID for shared service type")
	}

	if len(payload.GlobalOwners) > 0 {
		return errors.NewValidation("global_owners must not be provided for shared service type")
	}

	return nil
}

// validateUpdateImmutabilityConstraints validates that only mutable fields are being modified
func validateUpdateImmutabilityConstraints(existing *model.GrpsIOService, payload *mailinglistservice.UpdateGrpsioServicePayload) error {
	// Immutable Fields: type, project_uid, prefix, domain, group_id, url, group_name
	// Mutable Fields: global_owners, status, public only

	if payload.Type != existing.Type {
		return errors.NewValidation(fmt.Sprintf("field 'type' is immutable. Cannot change from '%s' to '%s'", existing.Type, payload.Type))
	}

	if payload.ProjectUID != existing.ProjectUID {
		return errors.NewValidation(fmt.Sprintf("field 'project_uid' is immutable. Cannot change from '%s' to '%s'", existing.ProjectUID, payload.ProjectUID))
	}

	// Check prefix immutability
	currentPrefix := existing.Prefix
	newPrefix := payloadStringValue(payload.Prefix)
	if newPrefix != currentPrefix {
		return errors.NewValidation(fmt.Sprintf("field 'prefix' is immutable. Cannot change from '%s' to '%s'", currentPrefix, newPrefix))
	}

	// Check domain immutability
	currentDomain := existing.Domain
	newDomain := payloadStringValue(payload.Domain)
	if newDomain != currentDomain {
		return errors.NewValidation(fmt.Sprintf("field 'domain' is immutable. Cannot change from '%s' to '%s'", currentDomain, newDomain))
	}

	// Check group_id immutability
	currentGroupID := existing.GroupID
	newGroupID := payloadInt64Value(payload.GroupID)
	if newGroupID != currentGroupID {
		return errors.NewValidation(fmt.Sprintf("field 'group_id' is immutable. Cannot change from '%d' to '%d'", currentGroupID, newGroupID))
	}

	// Check url immutability
	currentURL := existing.URL
	newURL := payloadStringValue(payload.URL)
	if newURL != currentURL {
		return errors.NewValidation(fmt.Sprintf("field 'url' is immutable. Cannot change from '%s' to '%s'", currentURL, newURL))
	}

	// Check group_name immutability
	currentGroupName := existing.GroupName
	newGroupName := payloadStringValue(payload.GroupName)
	if newGroupName != currentGroupName {
		return errors.NewValidation(fmt.Sprintf("field 'group_name' is immutable. Cannot change from '%s' to '%s'", currentGroupName, newGroupName))
	}

	// Validate global_owners email addresses if being updated
	if err := validateEmailAddresses(payload.GlobalOwners, "global_owners"); err != nil {
		return err
	}

	return nil
}

// validateDeleteProtectionRules validates deletion protection rules based on service type
func validateDeleteProtectionRules(service *model.GrpsIOService) error {
	// Delete Protection Rules:
	// - primary services: Cannot be deleted (critical infrastructure protection)
	// - formation/shared services: Can be deleted by owner only (TODO: implement owner check)

	switch service.Type {
	case "primary":
		return errors.NewValidation("Primary services cannot be deleted as they are critical infrastructure components")
	case "formation":
		// TODO: Add owner permission check when OpenFGA integration is complete
		// For now, allow deletion of formation services
		slog.Debug("Allowing deletion of formation service", "service_id", service.UID, "type", service.Type)
		return nil
	case "shared":
		// TODO: Add owner permission check when OpenFGA integration is complete
		// For now, allow deletion of shared services
		slog.Debug("Allowing deletion of shared service", "service_id", service.UID, "type", service.Type)
		return nil
	default:
		return errors.NewValidation(fmt.Sprintf("Unknown service type '%s' - deletion not permitted", service.Type))
	}
}

// Helper functions for code reuse - added for optimization

// payloadToDomainService converts create payload to domain model
func payloadToDomainService(payload *mailinglistservice.CreateGrpsioServicePayload, uid string) *model.GrpsIOService {
	now := time.Now()
	return &model.GrpsIOService{
		Type:         payload.Type,
		UID:          uid,
		Domain:       payloadStringValue(payload.Domain),
		GroupID:      payloadInt64Value(payload.GroupID),
		Status:       payloadStringValue(payload.Status),
		GlobalOwners: payload.GlobalOwners,
		Prefix:       payloadStringValue(payload.Prefix),
		ProjectSlug:  payloadStringValue(payload.ProjectSlug),
		ProjectUID:   payload.ProjectUID,
		URL:          payloadStringValue(payload.URL),
		GroupName:    payloadStringValue(payload.GroupName),
		Public:       payload.Public,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// buildUpdateRequest creates domain model for updates with only mutable fields
func buildUpdateRequest(existing *model.GrpsIOService, payload *mailinglistservice.UpdateGrpsioServicePayload) *model.GrpsIOService {
	return &model.GrpsIOService{
		// Preserve immutable fields from existing service
		Type:        existing.Type,
		UID:         *payload.UID,
		Domain:      existing.Domain,
		GroupID:     existing.GroupID,
		Prefix:      existing.Prefix,
		ProjectSlug: existing.ProjectSlug,
		ProjectUID:  existing.ProjectUID,
		URL:         existing.URL,
		GroupName:   existing.GroupName,
		CreatedAt:   existing.CreatedAt,

		// Update only mutable fields
		Status:       payloadStringValue(payload.Status),
		GlobalOwners: payload.GlobalOwners,
		Public:       payload.Public,
		UpdatedAt:    time.Now(),
	}
}

// validateEmailAddresses validates a slice of email addresses
func validateEmailAddresses(emails []string, fieldName string) error {
	if emails == nil {
		return nil
	}
	for _, email := range emails {
		if _, err := mail.ParseAddress(email); err != nil {
			return errors.NewValidation(fmt.Sprintf("invalid email address in %s: %s", fieldName, email))
		}
	}
	return nil
}
