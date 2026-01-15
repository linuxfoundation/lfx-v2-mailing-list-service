// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOServiceRepository defines the interface for service data persistence.
// This interface represents pure storage operations without orchestration logic.
// Implementations should handle data persistence, constraints, and indexing
// but not business logic like Groups.io API coordination or message publishing.
//
// This interface follows the Repository pattern and should be implemented by:
//   - NATS storage layer (production)
//   - Mock storage layer (testing)
//
// For business logic orchestration, see service.GrpsIOServiceWriter.
type GrpsIOServiceRepository interface {
	BaseGrpsIOWriter

	// CreateGrpsIOService creates a new service and its settings, and returns the service, settings, and revision
	CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService, settings *model.GrpsIOServiceSettings) (*model.GrpsIOService, *model.GrpsIOServiceSettings, uint64, error)

	// UpdateGrpsIOService updates an existing service with expected revision and returns updated service with new revision
	UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error)

	// DeleteGrpsIOService deletes a service by UID with expected revision
	// service parameter contains the service data for constraint cleanup
	DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error

	// UniqueProjectType validates that only one primary service exists per project
	UniqueProjectType(ctx context.Context, service *model.GrpsIOService) (string, error)

	// UniqueProjectPrefix validates that the prefix is unique within the project for formation services
	UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error)

	// UniqueProjectGroupID validates that the group_id is unique within the project for shared services
	UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error)

	// CreateGrpsIOServiceSettings creates new service settings and returns the settings with revision
	CreateGrpsIOServiceSettings(ctx context.Context, settings *model.GrpsIOServiceSettings) (*model.GrpsIOServiceSettings, uint64, error)

	// UpdateGrpsIOServiceSettings updates service settings with expected revision and returns updated settings with new revision
	UpdateGrpsIOServiceSettings(ctx context.Context, settings *model.GrpsIOServiceSettings, expectedRevision uint64) (*model.GrpsIOServiceSettings, uint64, error)
}
