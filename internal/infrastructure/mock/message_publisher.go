// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// SpyMessagePublisher records every call to Indexer and Access for assertion in tests.
type SpyMessagePublisher struct {
	IndexerCalls []PublishedMsg
	AccessCalls  []PublishedMsg
}

// PublishedMsg holds the subject and message from a single publisher call.
type PublishedMsg struct {
	Subject string
	Message any
}

var _ port.MessagePublisher = (*SpyMessagePublisher)(nil)

func (s *SpyMessagePublisher) Indexer(_ context.Context, subject string, message any) error {
	s.IndexerCalls = append(s.IndexerCalls, PublishedMsg{subject, message})
	return nil
}
func (s *SpyMessagePublisher) Access(_ context.Context, subject string, message any) error {
	s.AccessCalls = append(s.AccessCalls, PublishedMsg{subject, message})
	return nil
}
func (s *SpyMessagePublisher) Internal(_ context.Context, _ string, _ any) error { return nil }

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

// Internal publishes internal service events (mock implementation - logs only)
func (m *mockMessagePublisher) Internal(ctx context.Context, subject string, message any) error {
	slog.InfoContext(ctx, "mock internal event published",
		"subject", subject,
		"message_type", "internal",
	)
	return nil
}
