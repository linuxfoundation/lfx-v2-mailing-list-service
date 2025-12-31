// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GetGrpsIOService retrieves a single service by ID
func (sr *grpsIOReaderOrchestrator) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "executing get service use case",
		"service_uid", uid,
	)

	// Get service from storage
	service, revision, err := sr.grpsIOReader.GetGrpsIOService(ctx, uid)
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
func (sr *grpsIOReaderOrchestrator) GetRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "executing get revision use case",
		"service_uid", uid,
	)

	// Get revision from storage
	revision, err := sr.grpsIOReader.GetRevision(ctx, uid)
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

// GetServicesByGroupID retrieves all services for a given GroupsIO parent group ID
func (sr *grpsIOReaderOrchestrator) GetServicesByGroupID(ctx context.Context, groupID uint64) ([]*model.GrpsIOService, error) {
	slog.DebugContext(ctx, "executing get services by group_id use case",
		"group_id", groupID,
	)

	// Get services from storage
	services, err := sr.grpsIOReader.GetServicesByGroupID(ctx, groupID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get services by group_id",
			"error", err,
			"group_id", groupID,
		)
		return nil, err
	}

	slog.DebugContext(ctx, "services retrieved successfully by group_id",
		"group_id", groupID,
		"count", len(services),
	)

	return services, nil
}

// GetServicesByProjectUID retrieves all services for a given project UID
func (sr *grpsIOReaderOrchestrator) GetServicesByProjectUID(ctx context.Context, projectUID string) ([]*model.GrpsIOService, error) {
	slog.DebugContext(ctx, "executing get services by project_uid use case",
		"project_uid", projectUID,
	)

	// Get services from storage
	services, err := sr.grpsIOReader.GetServicesByProjectUID(ctx, projectUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get services by project_uid",
			"error", err,
			"project_uid", projectUID,
		)
		return nil, err
	}

	slog.DebugContext(ctx, "services retrieved successfully by project_uid",
		"project_uid", projectUID,
		"count", len(services),
	)

	return services, nil
}
