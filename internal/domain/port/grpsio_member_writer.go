// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOMemberWriter defines the interface for writing member data
type GrpsIOMemberWriter interface {
	BaseGrpsIOWriter

	// CreateGrpsIOMember creates a new member and returns it with revision
	CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error)

	// UpdateGrpsIOMember updates an existing member with optimistic concurrency control
	UpdateGrpsIOMember(ctx context.Context, uid string, member *model.GrpsIOMember, expectedRevision uint64) (*model.GrpsIOMember, uint64, error)

	// DeleteGrpsIOMember deletes a member with optimistic concurrency control
	DeleteGrpsIOMember(ctx context.Context, uid string, expectedRevision uint64) error

	// UniqueMember validates member email is unique within mailing list
	UniqueMember(ctx context.Context, member *model.GrpsIOMember) (string, error)
}
