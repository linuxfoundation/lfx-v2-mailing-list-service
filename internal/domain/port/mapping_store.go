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

// MappingState is the lifecycle state of a mapping key.
type MappingState int

const (
	// MappingAbsent means the key was never written (or has been purged).
	MappingAbsent MappingState = iota
	// MappingPresent means the key holds a live value.
	MappingPresent
	// MappingTombstoned means the key holds the deletion marker.
	MappingTombstoned
)

// MappingEntry is the result of a successful Lookup.
type MappingEntry struct {
	State MappingState
	// Value is the stored value; it is only set when State is MappingPresent.
	Value string
}

// Present reports whether the key holds a live value.
func (e MappingEntry) Present() bool { return e.State == MappingPresent }

// Tombstoned reports whether the key holds the deletion marker.
func (e MappingEntry) Tombstoned() bool { return e.State == MappingTombstoned }

// Action returns ActionUpdated when a live mapping exists, and ActionCreated when
// the key is absent or tombstoned (entity never seen, or previously deleted and
// being re-created).
func (e MappingEntry) Action() model.MessageAction {
	if e.Present() {
		return model.ActionUpdated
	}
	return model.ActionCreated
}

// MappingReader abstracts reads from v1-mappings and the subgroup service index bucket.
// Implementations hide storage-level details such as tombstone markers and
// key-not-found semantics behind domain-meaningful operations, routing by key prefix.
type MappingReader interface {
	// Lookup reads a key once and reports whether it is absent, present (with its
	// value), or tombstoned. A storage read failure is returned as an error and is
	// never folded into MappingAbsent, so callers can retry instead of acting on a
	// state they could not verify. Implementations mark errors they know to be
	// permanent with pkg/errors.MarkPermanent.
	Lookup(ctx context.Context, key string) (MappingEntry, error)
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
