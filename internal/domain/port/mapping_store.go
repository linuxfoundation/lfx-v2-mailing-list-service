// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
	"errors"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// ErrMappingAlreadyExists is returned by CreateMapping when the key already exists.
var ErrMappingAlreadyExists = errors.New("mapping key already exists")

// MappingReader abstracts reads from v1-mappings and the subgroup service index bucket.
// Implementations hide storage-level details such as tombstone markers and
// key-not-found semantics behind domain-meaningful operations, routing by key prefix.
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

	// GetMappingValueWithError returns the stored value, whether it exists and is
	// not tombstoned, and any storage read error. Callers can distinguish an
	// absent optional mapping from a failed read that should be retried.
	GetMappingValueWithError(ctx context.Context, key string) (string, bool, error)
}

// MappingWriter abstracts writes to v1-mappings and the subgroup service index bucket.
// Implementations route keys by prefix.
type MappingWriter interface {
	// PutMapping records that an entity has been successfully processed so that
	// subsequent events for the same key are treated as updates rather than creates.
	PutMapping(ctx context.Context, key, value string) error

	// CreateMapping atomically writes key=value only when the key does not yet
	// exist. Returns ErrMappingAlreadyExists when the key is already present,
	// allowing callers to use it as a compare-and-set dedup guard.
	CreateMapping(ctx context.Context, key, value string) error

	// PurgeMapping removes the key and all its history so a subsequent
	// CreateMapping call can succeed. Used to release an in-flight claim
	// (e.g. "pending" invite dedup slot) when the operation it guards fails,
	// allowing JetStream redelivery to retry cleanly.
	PurgeMapping(ctx context.Context, key string) error

	// PutTombstone writes the deletion marker to prevent duplicate delete
	// processing on consumer redelivery.
	PutTombstone(ctx context.Context, key string) error
}

// MappingReaderWriter combines access to v1-mappings and the subgroup service index KV bucket.
type MappingReaderWriter interface {
	MappingReader
	MappingWriter
	// ListSubgroupsByService returns live subgroup UIDs indexed under a service.
	// Order is unspecified; an incomplete KV enumeration returns an error.
	ListSubgroupsByService(ctx context.Context, serviceUID string) ([]string, error)
}
