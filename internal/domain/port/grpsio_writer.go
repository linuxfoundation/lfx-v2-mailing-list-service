// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
)

// BaseGrpsIOWriter defines common operations shared by all GrpsIO writers
type BaseGrpsIOWriter interface {
	// GetKeyRevision retrieves the revision for a given key (used for cleanup operations)
	GetKeyRevision(ctx context.Context, key string) (uint64, error)

	// Delete removes a key with the given revision (used for cleanup and rollback)
	Delete(ctx context.Context, key string, revision uint64) error
}

// GrpsIOWriter combines all writer operations for services and mailing lists
type GrpsIOWriter interface {
	GrpsIOServiceWriter
	GrpsIOMailingListWriter
}
