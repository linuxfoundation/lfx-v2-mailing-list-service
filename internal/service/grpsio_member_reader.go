// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GetGrpsIOMember retrieves a member by UID
func (r *grpsIOReaderOrchestrator) GetGrpsIOMember(ctx context.Context, uid string) (*model.GrpsIOMember, uint64, error) {
	if r.grpsIOReader == nil {
		panic("grpsIOReader dependency is required but was not provided")
	}

	slog.DebugContext(ctx, "executing get member use case",
		"member_uid", uid,
	)

	// Get member from storage
	member, revision, err := r.grpsIOReader.GetGrpsIOMember(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get member",
			"error", err,
			"member_uid", uid,
		)
		return nil, 0, err
	}

	slog.DebugContext(ctx, "member retrieved successfully",
		"member_uid", uid,
		"revision", revision,
	)

	return member, revision, nil
}

// GetMemberRevision retrieves only the revision for a given member UID
func (r *grpsIOReaderOrchestrator) GetMemberRevision(ctx context.Context, uid string) (uint64, error) {
	if r.grpsIOReader == nil {
		panic("grpsIOReader dependency is required but was not provided")
	}

	slog.DebugContext(ctx, "executing get member revision use case", "member_uid", uid)

	revision, err := r.grpsIOReader.GetMemberRevision(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get member revision", "error", err, "member_uid", uid)
		return 0, err
	}

	slog.DebugContext(ctx, "member revision retrieved successfully", "member_uid", uid, "revision", revision)
	return revision, nil
}
