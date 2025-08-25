// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// mockMessagePublisher is a mock implementation of the MessagePublisher interface
type mockMessagePublisher struct{}

// Ensure mockMessagePublisher implements the MessagePublisher interface
var _ port.MessagePublisher = (*mockMessagePublisher)(nil)

// NewMockMessagePublisher creates a new mock publisher for testing
func NewMockMessagePublisher() port.MessagePublisher {
	return &mockMessagePublisher{}
}

// Indexer publishes indexer messages (mock implementation - logs only)
func (m *mockMessagePublisher) Indexer(ctx context.Context, subject string, message any) error {
	slog.InfoContext(ctx, "mock indexer message published",
		"subject", subject,
		"message_type", "indexer",
	)
	return nil
}

// Access publishes access control messages (mock implementation - logs only)
func (m *mockMessagePublisher) Access(ctx context.Context, subject string, message any) error {
	slog.InfoContext(ctx, "mock access control message published",
		"subject", subject,
		"message_type", "access",
	)
	return nil
}
