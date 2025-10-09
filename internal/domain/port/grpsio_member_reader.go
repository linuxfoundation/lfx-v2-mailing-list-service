// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOMemberReader defines the interface for reading member data
type GrpsIOMemberReader interface {
	// GetGrpsIOMember retrieves a member by UID with revision
	GetGrpsIOMember(ctx context.Context, uid string) (*model.GrpsIOMember, uint64, error)

	// GetMemberRevision retrieves only the revision for a given UID
	GetMemberRevision(ctx context.Context, uid string) (uint64, error)

	// GetMemberByGroupsIOMemberID retrieves member by Groups.io member ID using secondary index
	// Returns NotFound if no member has this Groups.io ID
	// Used by webhook handlers to find existing members
	GetMemberByGroupsIOMemberID(ctx context.Context, memberID uint64) (*model.GrpsIOMember, uint64, error)

	// GetMemberByEmail retrieves member by email within mailing list
	// Returns NotFound if no member with this email exists in the mailing list
	// Used by orchestrator for idempotency checks
	GetMemberByEmail(ctx context.Context, mailingListUID, email string) (*model.GrpsIOMember, uint64, error)
}
