// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
)

// GrpsIOReaderWriter combines all reader and writer operations for services and mailing lists
type GrpsIOReaderWriter interface {
	GrpsIOReader
	GrpsIOWriter

	// IsReady checks if the storage is ready by verifying the connection
	IsReady(ctx context.Context) error
}
