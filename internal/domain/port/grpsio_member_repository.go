// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOMemberRepository defines the interface for member data persistence.
// This interface represents pure storage operations without orchestration logic.
// Implementations should handle data persistence, constraints, and indexing
// but not business logic like Groups.io API coordination or message publishing.
//
// This interface follows the Repository pattern and should be implemented by:
//   - NATS storage layer (production)
//   - Mock storage layer (testing)
//
// For business logic orchestration, see service.GrpsIOMemberWriter.
type GrpsIOMemberRepository interface {
	BaseGrpsIOWriter

	// CreateGrpsIOMember stores a new member and returns it with revision
	CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error)

	// UpdateGrpsIOMember updates an existing member with optimistic concurrency control
	UpdateGrpsIOMember(ctx context.Context, uid string, member *model.GrpsIOMember, expectedRevision uint64) (*model.GrpsIOMember, uint64, error)

	// DeleteGrpsIOMember deletes a member with optimistic concurrency control
	// The member parameter provides context for constraint cleanup and should contain the existing member data
	DeleteGrpsIOMember(ctx context.Context, uid string, expectedRevision uint64, member *model.GrpsIOMember) error

	// UniqueMember validates member email is unique within mailing list
	// Returns constraint key for rollback and error if validation fails
	UniqueMember(ctx context.Context, member *model.GrpsIOMember) (string, error)

	// CreateMemberSecondaryIndices creates lookup indices for Groups.io IDs
	// Returns keys created (for rollback) and error if index creation fails
	// Pattern mirrors createMailingListSecondaryIndices
	CreateMemberSecondaryIndices(ctx context.Context, member *model.GrpsIOMember) ([]string, error)
}
