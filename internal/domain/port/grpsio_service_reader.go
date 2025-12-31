// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOServiceReader defines the interface for service read operations
type GrpsIOServiceReader interface {
	// GetGrpsIOService retrieves a single service by ID and returns ETag revision
	GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error)
	// GetRevision retrieves only the revision for a given UID
	GetRevision(ctx context.Context, uid string) (uint64, error)

	// GetServicesByGroupID retrieves all services for a given GroupsIO parent group ID
	// A single parent group can have multiple services (1 primary + N formation/shared services)
	// Returns empty slice if no services found (not an error)
	// Used by webhook processor to determine which service should adopt a subgroup
	GetServicesByGroupID(ctx context.Context, groupID uint64) ([]*model.GrpsIOService, error)

	// GetServicesByProjectUID retrieves all services for a given project UID
	// A single project can have multiple services (1 primary + N formation/shared services)
	// Returns empty slice if no services found (not an error)
	// Used to find parent primary services and list all services for a project
	GetServicesByProjectUID(ctx context.Context, projectUID string) ([]*model.GrpsIOService, error)
}
