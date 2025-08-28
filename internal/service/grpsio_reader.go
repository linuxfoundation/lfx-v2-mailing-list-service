// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// GrpsIOReader defines the composite interface that combines readers
type GrpsIOReader interface {
	GrpsIOServiceReader
	GrpsIOMailingListReader
}

// GrpsIOServiceReader defines the interface for service read operations
type GrpsIOServiceReader interface {
	// GetGrpsIOService retrieves a single service by ID and returns the revision
	GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error)
	// GetRevision retrieves only the revision for a given UID
	GetRevision(ctx context.Context, uid string) (uint64, error)
}

// GrpsIOMailingListReader defines the interface for mailing list read operations
type GrpsIOMailingListReader interface {
	// GetGrpsIOMailingList retrieves a single mailing list by UID
	GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, error)
	// GetGrpsIOMailingListsByParent retrieves mailing lists by parent service ID
	GetGrpsIOMailingListsByParent(ctx context.Context, parentID string) ([]*model.GrpsIOMailingList, error)
	// GetGrpsIOMailingListsByCommittee retrieves mailing lists by committee ID
	GetGrpsIOMailingListsByCommittee(ctx context.Context, committeeID string) ([]*model.GrpsIOMailingList, error)
	// GetGrpsIOMailingListsByProject retrieves mailing lists by project ID
	GetGrpsIOMailingListsByProject(ctx context.Context, projectID string) ([]*model.GrpsIOMailingList, error)
}

// grpsIOReaderOrchestratorOption defines a function type for setting options on the composite orchestrator
type grpsIOReaderOrchestratorOption func(*grpsIOReaderOrchestrator)

// WithGrpsIOReader sets the service reader orchestrator
func WithGrpsIOReader(reader port.GrpsIOReader) grpsIOReaderOrchestratorOption {
	return func(r *grpsIOReaderOrchestrator) {
		r.grpsIOReader = reader
	}
}

// grpsIOReaderOrchestrator is the composite orchestrator that delegates to individual orchestrators
type grpsIOReaderOrchestrator struct {
	grpsIOReader port.GrpsIOReader
}

// NewGrpsIOReaderOrchestrator creates a new composite reader orchestrator using the option pattern
func NewGrpsIOReaderOrchestrator(opts ...grpsIOReaderOrchestratorOption) GrpsIOReader {
	rc := &grpsIOReaderOrchestrator{}
	for _, opt := range opts {
		opt(rc)
	}

	return rc
}
