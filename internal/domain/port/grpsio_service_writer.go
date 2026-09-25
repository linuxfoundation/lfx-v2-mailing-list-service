// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GroupsIOServiceWriter defines the application-level interface for GroupsIO service operations.
// All IDs are v2 UUIDs. Implementations are responsible for v1/v2 ID translation
// when communicating with the ITX proxy.
type GroupsIOServiceWriter interface {
	// CreateService creates a new GroupsIO service.
	CreateService(ctx context.Context, svc *model.GroupsIOService) (*model.GroupsIOService, error)

	// UpdateService updates a GroupsIO service.
	UpdateService(ctx context.Context, serviceID string, svc *model.GroupsIOService) (*model.GroupsIOService, error)

	// DeleteService deletes a GroupsIO service.
	DeleteService(ctx context.Context, serviceID string) error
}
