// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// GrpsIOWriter defines the composite interface that combines writers
type GrpsIOWriter interface {
	GrpsIOServiceWriter
	GrpsIOMailingListWriter
	port.GrpsIOMemberWriter
}

// GrpsIOServiceWriter defines the interface for service write operations
type GrpsIOServiceWriter interface {
	// CreateGrpsIOService creates a new service and returns the service with revision
	CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error)

	// UpdateGrpsIOService updates an existing service with expected revision and returns updated service with new revision
	UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error)

	// DeleteGrpsIOService deletes a service by UID with expected revision
	// Pass the existing service data to DeleteGrpsIOService to allow the storage layer to perform
	// constraint cleanup based on the current state of the service. The 'service' parameter provides
	// necessary context for deleting related constraints or dependent records.
	DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error
}

// GrpsIOMailingListWriter defines the interface for mailing list write operations
type GrpsIOMailingListWriter interface {
	// CreateGrpsIOMailingList creates a new mailing list and returns the mailing list with revision
	CreateGrpsIOMailingList(ctx context.Context, request *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error)

	// UpdateGrpsIOMailingList updates an existing mailing list with expected revision and returns updated mailing list with new revision
	UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error)

	// DeleteGrpsIOMailingList deletes a mailing list by UID with expected revision
	DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error
}

// grpsIOWriterOrchestratorOption defines a function type for setting options on the composite orchestrator
type grpsIOWriterOrchestratorOption func(*grpsIOWriterOrchestrator)

// WithGrpsIOWriter sets the writer orchestrator
func WithGrpsIOWriter(writer port.GrpsIOWriter) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.grpsIOWriter = writer
	}
}

// WithGrpsIOWriterReader sets the reader orchestrator for writer
func WithGrpsIOWriterReader(reader port.GrpsIOReader) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.grpsIOReader = reader
	}
}

// WithEntityAttributeReader sets the entity attribute reader
func WithEntityAttributeReader(reader port.EntityAttributeReader) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.entityReader = reader
	}
}

// WithPublisher sets the publisher
func WithPublisher(publisher port.MessagePublisher) grpsIOWriterOrchestratorOption {
	return func(w *grpsIOWriterOrchestrator) {
		w.publisher = publisher
	}
}

// grpsIOWriterOrchestrator orchestrates the service writing process
type grpsIOWriterOrchestrator struct {
	grpsIOWriter port.GrpsIOWriter
	grpsIOReader port.GrpsIOReader
	entityReader port.EntityAttributeReader
	publisher    port.MessagePublisher
}

// NewGrpsIOWriterOrchestrator creates a new composite writer orchestrator using the option pattern
func NewGrpsIOWriterOrchestrator(opts ...grpsIOWriterOrchestratorOption) GrpsIOWriter {
	uc := &grpsIOWriterOrchestrator{}
	for _, opt := range opts {
		opt(uc)
	}

	return uc
}

// BaseGrpsIOWriter methods - delegated to underlying writer

// GetKeyRevision retrieves the revision for a given key (used for cleanup operations)
func (o *grpsIOWriterOrchestrator) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	return o.grpsIOWriter.GetKeyRevision(ctx, key)
}

// Delete removes a key with the given revision (used for cleanup and rollback)
func (o *grpsIOWriterOrchestrator) Delete(ctx context.Context, key string, revision uint64) error {
	return o.grpsIOWriter.Delete(ctx, key, revision)
}

// UniqueMember validates member email is unique within mailing list
func (o *grpsIOWriterOrchestrator) UniqueMember(ctx context.Context, member *model.GrpsIOMember) (string, error) {
	return o.grpsIOWriter.UniqueMember(ctx, member)
}

// Common methods implementation

// deleteKeys removes keys by getting their revision and deleting them
// This is used both for rollback scenarios and cleanup operations
func (o *grpsIOWriterOrchestrator) deleteKeys(ctx context.Context, keys []string, isRollback bool) {
	if len(keys) == 0 {
		return
	}

	slog.DebugContext(ctx, "deleting keys",
		"keys", keys,
		"is_rollback", isRollback,
	)

	for _, key := range keys {
		// Get revision using reader interface first (for entity UIDs), then try direct storage (for constraint keys)
		var rev uint64
		var errGet error

		// Try to get revision using reader interface first (works for entity UIDs)
		if o.grpsIOReader != nil {
			rev, errGet = o.grpsIOReader.GetRevision(ctx, key)
		}

		// If reader method fails, try the direct storage approach (works for constraint keys)
		if errGet != nil {
			rev, errGet = o.grpsIOWriter.GetKeyRevision(ctx, key)
		}

		if errGet != nil {
			slog.ErrorContext(ctx, "failed to get revision for key deletion",
				"key", key,
				"error", errGet,
				"is_rollback", isRollback,
			)
			continue
		}

		// Delete the key using the revision
		err := o.grpsIOWriter.Delete(ctx, key, rev)
		if err != nil {
			slog.ErrorContext(ctx, "failed to delete key",
				"key", key,
				"error", err,
				"is_rollback", isRollback,
			)
		} else {
			slog.DebugContext(ctx, "successfully deleted key",
				"key", key,
				"is_rollback", isRollback,
			)
		}
	}

	slog.DebugContext(ctx, "key deletion completed",
		"keys_count", len(keys),
		"is_rollback", isRollback,
	)
}
