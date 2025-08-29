// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
)

// EntityAttributeReader defines read operations for entity attributes
type EntityAttributeReader interface {
	ProjectSlug(ctx context.Context, uid string) (string, error)
	ProjectName(ctx context.Context, uid string) (string, error)
	CommitteeName(ctx context.Context, uid string) (string, error)
}
