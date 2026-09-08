// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service implements the mailing list API service, proxying to the ITX GroupsIO API.
package service

import (
	"context"
	"errors"
	"log/slog"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"

	"goa.design/goa/v3/security"
)

// mailingListAPI implements the generated mailinglist.Service interface.
type mailingListAPI struct {
	auth              port.Authenticator
	natsClient        *infraNATS.NATSClient
	serviceReader     port.GroupsIOServiceReader
	serviceWriter     port.GroupsIOServiceWriter
	mailingListReader port.GroupsIOMailingListReader
	mailingListWriter port.GroupsIOMailingListWriter
	memberReader      port.GroupsIOMailingListMemberReader
	memberWriter      port.GroupsIOMailingListMemberWriter
	artifactReader    port.GroupsIOArtifactReader
}

// NewMailingListAPI returns the mailing list API service implementation.
func NewMailingListAPI(
	auth port.Authenticator,
	natsClient *infraNATS.NATSClient,
	serviceReader port.GroupsIOServiceReader,
	serviceWriter port.GroupsIOServiceWriter,
	mailingListReader port.GroupsIOMailingListReader,
	mailingListWriter port.GroupsIOMailingListWriter,
	memberReader port.GroupsIOMailingListMemberReader,
	memberWriter port.GroupsIOMailingListMemberWriter,
	artifactReader port.GroupsIOArtifactReader,
) mailinglist.Service {
	return &mailingListAPI{
		auth:              auth,
		natsClient:        natsClient,
		serviceReader:     serviceReader,
		serviceWriter:     serviceWriter,
		mailingListReader: mailingListReader,
		mailingListWriter: mailingListWriter,
		memberReader:      memberReader,
		memberWriter:      memberWriter,
		artifactReader:    artifactReader,
	}
}

// JWTAuth implements the authorization logic for the JWT security scheme.
func (s *mailingListAPI) JWTAuth(ctx context.Context, token string, _ *security.JWTScheme) (context.Context, error) {
	principal, err := s.auth.ParsePrincipal(ctx, token, slog.Default())
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}

// Livez implements the liveness probe endpoint.
func (s *mailingListAPI) Livez(_ context.Context) ([]byte, error) {
	return []byte("OK"), nil
}

// Readyz implements the readiness probe endpoint. It returns ServiceUnavailable
// when the NATS connection is not ready to serve traffic.
func (s *mailingListAPI) Readyz(ctx context.Context) ([]byte, error) {
	if err := s.natsClient.IsReady(ctx); err != nil {
		return nil, &mailinglist.ServiceUnavailableError{Message: err.Error()}
	}
	return []byte("OK"), nil
}

// mapDomainError converts domain errors into the appropriate GOA HTTP error types.
func mapDomainError(err error) error {
	if err == nil {
		return nil
	}
	var notFound errs.NotFound
	if errors.As(err, &notFound) {
		return &mailinglist.NotFoundError{Message: notFound.Error()}
	}
	var validation errs.Validation
	if errors.As(err, &validation) {
		return &mailinglist.BadRequestError{Message: validation.Error()}
	}
	var conflict errs.Conflict
	if errors.As(err, &conflict) {
		return &mailinglist.ConflictError{Message: conflict.Error()}
	}
	var unavailable errs.ServiceUnavailable
	if errors.As(err, &unavailable) {
		return &mailinglist.ServiceUnavailableError{Message: unavailable.Error()}
	}
	return &mailinglist.InternalServerError{Message: err.Error()}
}
