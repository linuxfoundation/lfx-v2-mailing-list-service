// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// GrpsIOServiceReader defines the interface for service read operations
type GrpsIOServiceReader interface {
	// GetGrpsIOService retrieves a single service by ID and returns the revision
	GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error)
	// GetRevision retrieves only the revision for a given UID
	GetRevision(ctx context.Context, uid string) (uint64, error)
}

// grpsIOServiceReaderOrchestratorOption defines a function type for setting options
type grpsIOServiceReaderOrchestratorOption func(*grpsIOServiceReaderOrchestrator)

// WithServiceReader sets the service reader
func WithServiceReader(reader port.GrpsIOServiceReader) grpsIOServiceReaderOrchestratorOption {
	return func(r *grpsIOServiceReaderOrchestrator) {
		r.grpsIOServiceReader = reader
	}
}

// grpsIOServiceReaderOrchestrator orchestrates the service reading process
type grpsIOServiceReaderOrchestrator struct {
	grpsIOServiceReader port.GrpsIOServiceReader
}

// GetGrpsIOService retrieves a single service by ID
func (sr *grpsIOServiceReaderOrchestrator) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "executing get service use case",
		"service_uid", uid,
	)

	// Get service from storage
	service, revision, err := sr.grpsIOServiceReader.GetGrpsIOService(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get service",
			"error", err,
			"service_uid", uid,
		)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "service retrieved successfully",
		"service_uid", uid,
		"revision", revision,
	)

	return service, revision, nil
}

// GetRevision retrieves only the revision for a given UID
func (sr *grpsIOServiceReaderOrchestrator) GetRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "executing get revision use case",
		"service_uid", uid,
	)

	// Get revision from storage
	revision, err := sr.grpsIOServiceReader.GetRevision(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get service revision",
			"error", err,
			"service_uid", uid,
		)
		return 0, err
	}

	slog.DebugContext(ctx, "service revision retrieved successfully",
		"service_uid", uid,
		"revision", revision,
	)

	return revision, nil
}

// NewGrpsIOServiceReaderOrchestrator creates a new service reader use case using the option pattern
func NewGrpsIOServiceReaderOrchestrator(opts ...grpsIOServiceReaderOrchestratorOption) GrpsIOServiceReader {
	sr := &grpsIOServiceReaderOrchestrator{}
	for _, opt := range opts {
		opt(sr)
	}
	if sr.grpsIOServiceReader == nil {
		panic("grpsIOServiceReader is required")
	}
	return sr
}
