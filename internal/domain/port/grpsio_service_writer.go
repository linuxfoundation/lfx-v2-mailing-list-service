// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOServiceWriter defines the interface for service write operations
type GrpsIOServiceWriter interface {
	BaseGrpsIOWriter

	// CreateGrpsIOService creates a new service and returns the service with revision
	CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error)

	// UpdateGrpsIOService updates an existing service with expected revision and returns updated service with new revision
	UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error)

	// DeleteGrpsIOService deletes a service by UID with expected revision
	// service parameter contains the service data for constraint cleanup
	DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error

	// Unique constraint validation methods
	// UniqueProjectType validates that only one primary service exists per project
	UniqueProjectType(ctx context.Context, service *model.GrpsIOService) (string, error)

	// UniqueProjectPrefix validates that the prefix is unique within the project for formation services
	UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error)

	// UniqueProjectGroupID validates that the group_id is unique within the project for shared services
	UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error)

	// UpdateGrpsIOServiceSettings updates service settings with expected revision and returns updated settings with new revision
	UpdateGrpsIOServiceSettings(ctx context.Context, settings *model.GrpsIOServiceSettings, expectedRevision uint64) (*model.GrpsIOServiceSettings, uint64, error)
}
