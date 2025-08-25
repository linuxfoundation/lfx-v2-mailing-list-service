// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// MessagePublisher defines the interface for publishing GroupsIO service messages
// This interface is implemented by the NATS messaging infrastructure to support
// indexing and access control message publishing for downstream services
type MessagePublisher interface {
	// Indexer publishes indexer messages for search and discovery services
	// These messages are consumed by indexing services to maintain search indexes
	Indexer(ctx context.Context, subject string, message any) error

	// Access publishes access control messages for OpenFGA permission management
	// These messages are consumed by the fga-sync service to update permission tuples
	Access(ctx context.Context, subject string, message any) error
}
