// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// MockGrpsIOWebhookProcessorWithError implements GrpsIOWebhookProcessor for testing error scenarios
type MockGrpsIOWebhookProcessorWithError struct {
	err error
}

// NewMockGrpsIOWebhookProcessorWithError creates a webhook processor that always returns the given error
func NewMockGrpsIOWebhookProcessorWithError(err error) port.GrpsIOWebhookProcessor {
	return &MockGrpsIOWebhookProcessorWithError{
		err: err,
	}
}

// ProcessEvent always returns the configured error
func (m *MockGrpsIOWebhookProcessorWithError) ProcessEvent(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	return m.err
}
