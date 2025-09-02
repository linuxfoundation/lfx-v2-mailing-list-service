// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GetGrpsIOMailingList retrieves a single mailing list by UID with revision
func (mlr *grpsIOReaderOrchestrator) GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "executing get mailing list use case",
		"mailing_list_uid", uid,
	)

	// Get mailing list from storage
	mailingList, revision, err := mlr.grpsIOReader.GetGrpsIOMailingList(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list",
			"error", err,
			"mailing_list_uid", uid,
		)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "mailing list retrieved successfully",
		"mailing_list_uid", uid,
		"group_name", mailingList.GroupName,
		"revision", revision,
	)

	return mailingList, revision, nil
}

// GetMailingListRevision retrieves only the revision for a given UID
func (mlr *grpsIOReaderOrchestrator) GetMailingListRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "executing get mailing list revision use case",
		"mailing_list_uid", uid,
	)

	// Get revision from storage
	revision, err := mlr.grpsIOReader.GetMailingListRevision(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing list revision",
			"error", err,
			"mailing_list_uid", uid,
		)
		return 0, err
	}

	slog.DebugContext(ctx, "mailing list revision retrieved successfully",
		"mailing_list_uid", uid,
		"revision", revision,
	)

	return revision, nil
}

