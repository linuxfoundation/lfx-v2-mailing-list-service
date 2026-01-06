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
	GrpsIOMemberReader
}

// GrpsIOServiceReader defines the interface for service read operations
type GrpsIOServiceReader interface {
	// GetGrpsIOService retrieves a single service by ID and returns the revision
	GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error)
	// GetRevision retrieves only the revision for a given UID
	GetRevision(ctx context.Context, uid string) (uint64, error)
	// GetServicesByGroupID retrieves all services for a given GroupsIO parent group ID
	GetServicesByGroupID(ctx context.Context, groupID uint64) ([]*model.GrpsIOService, error)
	// GetServicesByProjectUID retrieves all services for a given project UID
	GetServicesByProjectUID(ctx context.Context, projectUID string) ([]*model.GrpsIOService, error)
	// GetGrpsIOServiceSettings retrieves service settings by service UID
	GetGrpsIOServiceSettings(ctx context.Context, uid string) (*model.GrpsIOServiceSettings, uint64, error)
	// GetSettingsRevision retrieves only the revision for service settings
	GetSettingsRevision(ctx context.Context, uid string) (uint64, error)
}

// GrpsIOMailingListReader defines the interface for mailing list read operations
type GrpsIOMailingListReader interface {
	// GetGrpsIOMailingList retrieves a single mailing list by UID with revision
	GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, uint64, error)
	// GetMailingListRevision retrieves only the revision for a given UID
	GetMailingListRevision(ctx context.Context, uid string) (uint64, error)
	// GetMailingListByGroupID retrieves a mailing list by GroupsIO subgroup ID
	GetMailingListByGroupID(ctx context.Context, groupID uint64) (*model.GrpsIOMailingList, uint64, error)
	// GetGrpsIOMailingListSettings retrieves mailing list settings by UID with revision
	GetGrpsIOMailingListSettings(ctx context.Context, uid string) (*model.GrpsIOMailingListSettings, uint64, error)
	// GetMailingListSettingsRevision retrieves only the revision for mailing list settings
	GetMailingListSettingsRevision(ctx context.Context, uid string) (uint64, error)
}

// GrpsIOMemberReader defines the interface for member read operations
type GrpsIOMemberReader interface {
	GetGrpsIOMember(ctx context.Context, uid string) (*model.GrpsIOMember, uint64, error)
	GetMemberRevision(ctx context.Context, uid string) (uint64, error)
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

	// Fail fast if required dependency is missing
	if rc.grpsIOReader == nil {
		panic("grpsIOReader dependency is required")
	}
	// Note: grpsIOReader provides all operations including member operations

	return rc
}
