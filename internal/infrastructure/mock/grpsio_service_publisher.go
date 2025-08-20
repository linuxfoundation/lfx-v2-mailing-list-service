// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// mockGrpsIOServicePublisher is a mock implementation of the GrpsIOServicePublisher interface
type mockGrpsIOServicePublisher struct{}

// Ensure mockGrpsIOServicePublisher implements the GrpsIOServicePublisher interface
var _ port.GrpsIOServicePublisher = (*mockGrpsIOServicePublisher)(nil)

// NewMockGrpsIOServicePublisher creates a new mock publisher for testing
func NewMockGrpsIOServicePublisher() port.GrpsIOServicePublisher {
	return &mockGrpsIOServicePublisher{}
}

// Indexer publishes indexer messages (mock implementation - logs only)
func (m *mockGrpsIOServicePublisher) Indexer(ctx context.Context, subject string, message any) error {
	slog.InfoContext(ctx, "mock indexer message published",
		"subject", subject,
		"message_type", "indexer",
	)
	return nil
}

// Access publishes access control messages (mock implementation - logs only)
func (m *mockGrpsIOServicePublisher) Access(ctx context.Context, subject string, message any) error {
	slog.InfoContext(ctx, "mock access control message published",
		"subject", subject,
		"message_type", "access",
	)
	return nil
}
