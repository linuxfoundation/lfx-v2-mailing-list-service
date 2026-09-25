// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GroupsIOServiceReader defines the interface for service read operations
type GroupsIOServiceReader interface {

	// ListServices lists GroupsIO services, optionally filtered by project_uid.
	// Returns the matched services and total count.
	ListServices(ctx context.Context, projectUID string) ([]*model.GroupsIOService, int, error)

	// GetService retrieves a GroupsIO service by ID.
	GetService(ctx context.Context, serviceID string) (*model.GroupsIOService, error)

	// GetProjects returns project UIDs that have GroupsIO services.
	GetProjects(ctx context.Context) ([]string, error)

	// FindParentService finds the parent service for a project by project UID.
	FindParentService(ctx context.Context, projectUID string) (*model.GroupsIOService, error)
}
