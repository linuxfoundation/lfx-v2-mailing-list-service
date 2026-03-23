// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// DataEventHandler handles keyed data change events from an external data source
// (e.g. a NATS JetStream KV bucket sourced from DynamoDB via lfx-v1-sync-helper).
//
// HandleChange is called when a record is created, updated, or soft-deleted (the
// caller is responsible for detecting soft-deletes from the payload fields).
// HandleRemoval is called when a record is hard-deleted or purged.
//
// Both methods return true to signal a transient error (retry with exponential
// backoff) or false to acknowledge the event (success, or a permanent error that
// should not be retried).
type DataEventHandler interface {
	HandleChange(ctx context.Context, key string, data map[string]any) bool
	HandleRemoval(ctx context.Context, key string) bool
}
