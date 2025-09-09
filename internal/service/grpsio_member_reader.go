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

// CheckMemberExists checks if a member with given email exists in mailing list
func (r *grpsIOReaderOrchestrator) CheckMemberExists(ctx context.Context, mailingListUID, email string) (bool, error) {
	if r.grpsIOReader == nil {
		panic("grpsIOReader dependency is required but was not provided")
	}

	slog.DebugContext(ctx, "executing check member exists use case",
		"mailing_list_uid", mailingListUID,
		"email", email,
	)

	// Check if member exists
	exists, err := r.grpsIOReader.CheckMemberExists(ctx, mailingListUID, email)
	if err != nil {
		slog.ErrorContext(ctx, "failed to check member existence",
			"error", err,
			"mailing_list_uid", mailingListUID,
			"email", email,
		)
		return false, err
	}

	slog.DebugContext(ctx, "member existence check completed",
		"mailing_list_uid", mailingListUID,
		"email", email,
		"exists", exists,
	)

	return exists, nil
}
