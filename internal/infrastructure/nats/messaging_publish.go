// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// messagingPublisher implements the MessagePublisher interface using NATS
type messagingPublisher struct {
	client *NATSClient
}

// Indexer publishes indexer messages for search and discovery services
// These messages are consumed by indexing services to maintain search indexes
func (m *messagingPublisher) Indexer(ctx context.Context, subject string, message any) error {
	return m.publish(ctx, subject, message, "indexer")
}

// Access publishes access control messages for OpenFGA permission management
// These messages are consumed by the fga-sync service to update permission tuples
func (m *messagingPublisher) Access(ctx context.Context, subject string, message any) error {
	return m.publish(ctx, subject, message, "access")
}

// publish is the common method for publishing messages to NATS
func (m *messagingPublisher) publish(ctx context.Context, subject string, message any, messageType string) error {
	// Check if client is ready
	if err := m.client.IsReady(ctx); err != nil {
		slog.ErrorContext(ctx, "NATS client is not ready for publishing",
			"error", err,
			"subject", subject,
			"message_type", messageType,
		)
		return errors.NewServiceUnavailable("NATS client is not ready", err)
	}

	// Marshal message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal message to JSON",
			"error", err,
			"subject", subject,
			"message_type", messageType,
		)
		return errors.NewUnexpected("failed to marshal message", err)
	}

	// Publish message
	if err := m.client.conn.Publish(subject, data); err != nil {
		slog.ErrorContext(ctx, "failed to publish message to NATS",
			"error", err,
			"subject", subject,
			"message_type", messageType,
		)
		return errors.NewServiceUnavailable("failed to publish message", err)
	}

	slog.DebugContext(ctx, "message published successfully",
		"subject", subject,
		"message_type", messageType,
		"message_size", len(data),
	)

	return nil
}

// NewMessagePublisher creates a new MessagePublisher using NATS
func NewMessagePublisher(client *NATSClient) port.MessagePublisher {
	return &messagingPublisher{
		client: client,
	}
}
