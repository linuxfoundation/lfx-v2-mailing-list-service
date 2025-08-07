// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
)

// ProjectReader defines read operations for project information
type ProjectReader interface {
	Slug(ctx context.Context, uid string) (string, error)
	Name(ctx context.Context, uid string) (string, error)
}
