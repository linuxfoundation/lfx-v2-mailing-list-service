// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// EntityAttributeReader defines read operations for entity attributes
type EntityAttributeReader interface {
	ProjectSlug(ctx context.Context, uid string) (string, error)
	ProjectName(ctx context.Context, uid string) (string, error)
	ProjectParentUID(ctx context.Context, uid string) (string, error)
	CommitteeName(ctx context.Context, uid string) (string, error)
	ListMembers(ctx context.Context, committeeUID string) ([]model.CommitteeMember, error)
}
