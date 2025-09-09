// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service implements the mailing list service business logic and endpoints.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"

	"github.com/google/uuid"
	"goa.design/goa/v3/security"
)

// mailingListService is the implementation of the mailing list service.
type mailingListService struct {
	auth                     port.Authenticator
	grpsIOReaderOrchestrator service.GrpsIOReader
	grpsIOWriterOrchestrator service.GrpsIOWriter
	storage                  port.GrpsIOReaderWriter
}

// NewMailingList returns the mailing list service implementation.
func NewMailingList(auth port.Authenticator, grpsIOReaderOrchestrator service.GrpsIOReader, grpsIOWriterOrchestrator service.GrpsIOWriter, storage port.GrpsIOReaderWriter) mailinglistservice.Service {
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
	createdMailingList, revision, err := s.grpsIOWriterOrchestrator.CreateGrpsIOMailingList(ctx, domainMailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list", "error", err, "group_name", payload.GroupName)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertMailingListDomainToResponse(createdMailingList)

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
	goaMailingList := s.convertMailingListDomainToStandardResponse(mailingList)

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
func (s *mailingListService) UpdateGrpsioMailingList(ctx context.Context, payload *mailinglistservice.UpdateGrpsioMailingListPayload) (result *mailinglistservice.MailingListWithReadonlyAttributes, err error) {
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
	domainMailingList := s.convertUpdateMailingListPayloadToDomain(payload)
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
	result = s.convertMailingListDomainToStandardResponse(updatedMailingList)

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

// CreateGrpsioMailingListMember creates a new member for a GroupsIO mailing list
func (s *mailingListService) CreateGrpsioMailingListMember(ctx context.Context, payload *mailinglistservice.CreateGrpsioMailingListMemberPayload) (result *mailinglistservice.MemberFull, err error) {
	slog.DebugContext(ctx, "mailingListService.create-grpsio-mailing-list-member",
		"mailing_list_uid", payload.UID,
		"email", payload.Email,
	)

	// Generate new UID for the member
	memberUID := uuid.New().String()

	// Convert GOA payload to domain model
	domainMember := s.convertMemberPayloadToDomain(payload)
	domainMember.UID = memberUID

	// Execute use case
	createdMember, revision, err := s.grpsIOWriterOrchestrator.CreateGrpsIOMember(ctx, domainMember)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create member",
			"error", err,
			"mailing_list_uid", payload.UID,
			"email", payload.Email,
		)
		return nil, wrapError(ctx, err)
	}

	// Convert domain model to GOA response
	result = s.convertMemberDomainToResponse(createdMember)

	slog.InfoContext(ctx, "successfully created member",
		"member_uid", createdMember.UID,
		"mailing_list_uid", createdMember.MailingListUID,
		"email", createdMember.Email,
		"revision", revision,
	)
	return result, nil
}

// Helper functions

// convertMemberPayloadToDomain converts GOA member payload to domain model
func (s *mailingListService) convertMemberPayloadToDomain(payload *mailinglistservice.CreateGrpsioMailingListMemberPayload) *model.GrpsIOMember {
	member := &model.GrpsIOMember{
		MailingListUID: payload.UID,
		Email:          payload.Email,
		MemberType:     payload.MemberType,
		DeliveryMode:   payload.DeliveryMode,
		ModStatus:      payload.ModStatus,
	}

	// Handle required fields that might be pointers
	if payload.FirstName != nil {
		member.FirstName = *payload.FirstName
	}
	if payload.LastName != nil {
		member.LastName = *payload.LastName
	}

	// Handle optional fields
	if payload.Username != nil {
		member.Username = *payload.Username
	}
	if payload.Organization != nil {
		member.Organization = *payload.Organization
	}
	if payload.JobTitle != nil {
		member.JobTitle = *payload.JobTitle
	}
	if payload.LastReviewedAt != nil {
		member.LastReviewedAt = payload.LastReviewedAt
	}
	if payload.LastReviewedBy != nil {
		member.LastReviewedBy = payload.LastReviewedBy
	}
	if payload.Writers != nil {
		member.Writers = payload.Writers
	}
	if payload.Auditors != nil {
		member.Auditors = payload.Auditors
	}

	return member
}

// convertMemberDomainToResponse converts domain member to GOA response
func (s *mailingListService) convertMemberDomainToResponse(member *model.GrpsIOMember) *mailinglistservice.MemberFull {
	response := &mailinglistservice.MemberFull{
		UID:            member.UID,
		MailingListUID: member.MailingListUID,
		FirstName:      member.FirstName,
		LastName:       member.LastName,
		Email:          member.Email,
		MemberType:     member.MemberType,
		DeliveryMode:   member.DeliveryMode,
		ModStatus:      member.ModStatus,
		Status:         member.Status,
	}

	// Handle optional fields
	if member.Username != "" {
		response.Username = &member.Username
	}
	if member.Organization != "" {
		response.Organization = &member.Organization
	}
	if member.JobTitle != "" {
		response.JobTitle = &member.JobTitle
	}
	if member.GroupsIOMemberID != 0 {
		response.GroupsioMemberID = &member.GroupsIOMemberID
	}
	if member.GroupsIOGroupID != 0 {
		response.GroupsioGroupID = &member.GroupsIOGroupID
	}
	if member.LastReviewedAt != nil {
		response.LastReviewedAt = member.LastReviewedAt
	}
	if member.LastReviewedBy != nil {
		response.LastReviewedBy = member.LastReviewedBy
	}
	if len(member.Writers) > 0 {
		response.Writers = member.Writers
	}
	if len(member.Auditors) > 0 {
		response.Auditors = member.Auditors
	}

	// Convert timestamps
	if !member.CreatedAt.IsZero() {
		response.CreatedAt = member.CreatedAt.Format(time.RFC3339)
	}
	if !member.UpdatedAt.IsZero() {
		response.UpdatedAt = member.UpdatedAt.Format(time.RFC3339)
	}

	return response
}

// convertMailingListDomainToStandardResponse converts a domain mailing list to GOA standard response type
func (s *mailingListService) convertMailingListDomainToStandardResponse(mailingList *model.GrpsIOMailingList) *mailinglistservice.MailingListWithReadonlyAttributes {
	response := &mailinglistservice.MailingListWithReadonlyAttributes{
		UID:              &mailingList.UID,
		GroupName:        &mailingList.GroupName,
		Public:           mailingList.Public,
		Type:             &mailingList.Type,
		CommitteeUID:     maybeString(mailingList.CommitteeUID),
		CommitteeFilters: maybeStringSlice(mailingList.CommitteeFilters),
		Description:      &mailingList.Description,
		Title:            &mailingList.Title,
		SubjectTag:       maybeString(mailingList.SubjectTag),
		ServiceUID:       &mailingList.ServiceUID,
		ProjectUID:       maybeString(mailingList.ProjectUID),
		ProjectName:      maybeString(mailingList.ProjectName),
		ProjectSlug:      maybeString(mailingList.ProjectSlug),
		Writers:          maybeStringSlice(mailingList.Writers),
		Auditors:         maybeStringSlice(mailingList.Auditors),
	}

	// Convert timestamps
	if !mailingList.CreatedAt.IsZero() {
		createdAt := mailingList.CreatedAt.Format(time.RFC3339)
		response.CreatedAt = &createdAt
	}
	if !mailingList.UpdatedAt.IsZero() {
		updatedAt := mailingList.UpdatedAt.Format(time.RFC3339)
		response.UpdatedAt = &updatedAt
	}

	// Note: LastReviewedAt/By fields are not in MailingListWithReadonlyAttributes
	// They might be in a different response type or future enhancement

	return response
}

// convertUpdateMailingListPayloadToDomain converts an update payload to domain model
func (s *mailingListService) convertUpdateMailingListPayloadToDomain(payload *mailinglistservice.UpdateGrpsioMailingListPayload) *model.GrpsIOMailingList {
	// Create a new mailing list from payload data
	mailingList := &model.GrpsIOMailingList{
		GroupName:   payload.GroupName,
		Public:      payload.Public,
		Type:        payload.Type,
		Description: payload.Description,
		Title:       payload.Title,
		ServiceUID:  payload.ServiceUID,
	}

	// Handle pointer fields
	if payload.CommitteeUID != nil {
		mailingList.CommitteeUID = *payload.CommitteeUID
	}
	if payload.SubjectTag != nil {
		mailingList.SubjectTag = *payload.SubjectTag
	}

	// Handle slice fields
	if payload.CommitteeFilters != nil {
		mailingList.CommitteeFilters = payload.CommitteeFilters
	}
	if payload.Writers != nil {
		mailingList.Writers = payload.Writers
	}
	if payload.Auditors != nil {
		mailingList.Auditors = payload.Auditors
	}

	return mailingList
}

// Helper function to convert string to pointer if non-empty
func maybeString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Helper function to convert string slice to pointer if non-empty
func maybeStringSlice(slice []string) []string {
	if len(slice) == 0 {
		return nil
	}
	return slice
}

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
