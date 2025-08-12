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

	"goa.design/goa/v3/security"
)

// mailingListService is the implementation of the mailing list service.
type mailingListService struct {
	auth                            port.Authenticator
	grpsIOServiceReaderOrchestrator service.GrpsIOServiceReader
	storage                         port.GrpsIOServiceReaderWriter
}

// NewMailingList returns the mailing list service implementation.
func NewMailingList(auth port.Authenticator, grpsIOServiceReaderOrchestrator service.GrpsIOServiceReader, storage port.GrpsIOServiceReaderWriter) mailinglistservice.Service {
	return &mailingListService{
		auth:                            auth,
		grpsIOServiceReaderOrchestrator: grpsIOServiceReaderOrchestrator,
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
	goaService := &mailinglistservice.ServiceInfo{
		Type:         service.Type,
		ID:           service.ID,
		Domain:       service.Domain,
		GroupID:      service.GroupID,
		Status:       service.Status,
		GlobalOwners: service.GlobalOwners,
		Prefix:       &service.Prefix,
		ProjectSlug:  service.ProjectSlug,
		ProjectID:    service.ProjectID,
		URL:          service.URL,
		GroupName:    service.GroupName,
	}

	// Create result with ETag (using revision from NATS)
	revisionStr := fmt.Sprintf("%d", revision)
	result = &mailinglistservice.GetGrpsioServiceResult{
		Service: goaService,
		Etag:    &revisionStr,
	}

	slog.InfoContext(ctx, "successfully retrieved service", "service_uid", payload.UID, "etag", revisionStr)
	return result, nil
}
