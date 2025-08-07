package service

import (
	"context"
	"log/slog"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"

	"goa.design/goa/v3/security"
)

// mailingListService is the implementation of the mailing list service.
// Simplified for base PR - only handles health endpoints
type mailingListService struct {
	auth port.Authenticator
}

// NewMailingList returns the mailing list service implementation.
// Simplified for base PR - only requires auth for JWT method
func NewMailingList(auth port.Authenticator) mailinglistservice.Service {
	return &mailingListService{
		auth: auth,
	}
}

// JWTAuth implements the authorization logic for service "mailing-list"
// for the "jwt" security scheme.
func (s *mailingListService) JWTAuth(ctx context.Context, token string, scheme *security.JWTScheme) (context.Context, error) {
	// Parse the Heimdall-authorized principal from the token
	principal, err := s.auth.ParsePrincipal(ctx, token, slog.Default())
	if err != nil {
		return ctx, err
	}

	// Return a new context containing the principal as a value
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}

// Livez implements the livez endpoint for liveness probes.
func (s *mailingListService) Livez(ctx context.Context) error {
	return nil
}

// Readyz implements the readyz endpoint for readiness probes.
func (s *mailingListService) Readyz(ctx context.Context) error {
	// For health endpoints, we don't need complex connectivity checks in base PR
	// This will be enhanced when we add CRUD operations that need storage verification
	return nil
}
