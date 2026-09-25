// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package mock provides mock implementations for testing purposes.
package mock

import (
	"context"
	"log/slog"
	"os"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// MockAuthService provides a mock implementation of the authentication service
type MockAuthService struct{}

// ParsePrincipal parses and validates a JWT token, returning a mock principal
func (m *MockAuthService) ParsePrincipal(ctx context.Context, _ string, logger *slog.Logger) (string, error) {

	principal := os.Getenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL")

	if principal == "" {
		return "", errors.NewValidation("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL environment variable not set")
	}

	logger.DebugContext(ctx, "parsed principal",
		"user_id", principal,
	)

	return principal, nil
}

// NewMockAuthService creates a new mock authentication service
func NewMockAuthService() port.Authenticator {
	return &MockAuthService{}
}
