// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GetGrpsIOMailingList retrieves a single mailing list by UID
func (mlr *grpsIOReaderOrchestrator) GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "executing get mailing list use case",
		"mailing_list_uid", uid,
	)

	// Get mailing list from storage
	mailingList, err := mlr.grpsIOReader.GetGrpsIOMailingList(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list",
			"error", err,
			"mailing_list_uid", uid,
		)
		return nil, err
	}

	slog.DebugContext(ctx, "mailing list retrieved successfully",
		"mailing_list_uid", uid,
		"group_name", mailingList.GroupName,
	)

	return mailingList, nil
}

// GetGrpsIOMailingListsByParent retrieves mailing lists by parent service ID
func (mlr *grpsIOReaderOrchestrator) GetGrpsIOMailingListsByParent(ctx context.Context, parentID string) ([]*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "executing get mailing lists by parent use case",
		"parent_id", parentID,
	)

	// Get mailing lists from storage
	mailingLists, err := mlr.grpsIOReader.GetGrpsIOMailingListsByParent(ctx, parentID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing lists by parent",
			"error", err,
			"parent_id", parentID,
		)
		return nil, err
	}

	slog.DebugContext(ctx, "mailing lists retrieved successfully by parent",
		"parent_id", parentID,
		"count", len(mailingLists),
	)

	return mailingLists, nil
}

// GetGrpsIOMailingListsByCommittee retrieves mailing lists by committee ID
func (mlr *grpsIOReaderOrchestrator) GetGrpsIOMailingListsByCommittee(ctx context.Context, committeeID string) ([]*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "executing get mailing lists by committee use case",
		"committee_id", committeeID,
	)

	// Get mailing lists from storage
	mailingLists, err := mlr.grpsIOReader.GetGrpsIOMailingListsByCommittee(ctx, committeeID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing lists by committee",
			"error", err,
			"committee_id", committeeID,
		)
		return nil, err
	}

	slog.DebugContext(ctx, "mailing lists retrieved successfully by committee",
		"committee_id", committeeID,
		"count", len(mailingLists),
	)

	return mailingLists, nil
}

// GetGrpsIOMailingListsByProject retrieves mailing lists by project ID
func (mlr *grpsIOReaderOrchestrator) GetGrpsIOMailingListsByProject(ctx context.Context, projectID string) ([]*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "executing get mailing lists by project use case",
		"project_id", projectID,
	)

	// Get mailing lists from storage
	mailingLists, err := mlr.grpsIOReader.GetGrpsIOMailingListsByProject(ctx, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing lists by project",
			"error", err,
			"project_id", projectID,
		)
		return nil, err
	}

	slog.DebugContext(ctx, "mailing lists retrieved successfully by project",
		"project_id", projectID,
		"count", len(mailingLists),
	)

	return mailingLists, nil
}
