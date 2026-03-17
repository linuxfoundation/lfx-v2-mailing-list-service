// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// MappingReader abstracts read operations on the v1-mappings KV bucket.
// Implementations hide storage-level details such as tombstone markers and
// key-not-found semantics behind domain-meaningful operations.
type MappingReader interface {
	// ResolveAction returns ActionCreated when the key is absent or tombstoned
	// (entity never seen, or previously deleted and being re-created), and
	// ActionUpdated when a live mapping already exists.
	ResolveAction(ctx context.Context, key string) model.MessageAction

	// IsMappingPresent returns true when the key exists and is not tombstoned.
	// Used for parent-dependency checks (service before subgroup, subgroup before member).
	IsMappingPresent(ctx context.Context, key string) bool

	// IsTombstoned returns true when the key holds the deletion marker,
	// so duplicate delete events can be skipped.
	IsTombstoned(ctx context.Context, key string) bool

	// GetMappingValue returns the stored value and true when the key exists and
	// is not tombstoned. Used when the caller needs the actual value (e.g. the
	// reverse group_id → subgroup UID index in the member handler).
	GetMappingValue(ctx context.Context, key string) (string, bool)
}

// MappingWriter abstracts write operations on the v1-mappings KV bucket.
type MappingWriter interface {
	// PutMapping records that an entity has been successfully processed so that
	// subsequent events for the same key are treated as updates rather than creates.
	PutMapping(ctx context.Context, key, value string) error

	// PutTombstone writes the deletion marker to prevent duplicate delete
	// processing on consumer redelivery.
	PutTombstone(ctx context.Context, key string) error
}

// MappingReaderWriter combines read and write access to the v1-mappings KV bucket.
type MappingReaderWriter interface {
	MappingReader
	MappingWriter
}
