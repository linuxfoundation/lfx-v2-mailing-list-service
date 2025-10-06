// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"

// MockGrpsIOWebhookValidator implements GrpsIOWebhookValidator for testing
type MockGrpsIOWebhookValidator struct{}

// NewMockGrpsIOWebhookValidator creates a new mock GroupsIO webhook validator
func NewMockGrpsIOWebhookValidator() port.GrpsIOWebhookValidator {
	return &MockGrpsIOWebhookValidator{}
}

// ValidateSignature always returns nil in mock mode
func (m *MockGrpsIOWebhookValidator) ValidateSignature(body []byte, signature string) error {
	return nil // Always valid in mock mode
}

// IsValidEvent always returns true in mock mode
func (m *MockGrpsIOWebhookValidator) IsValidEvent(eventType string) bool {
	return true // All events valid in mock mode
}
