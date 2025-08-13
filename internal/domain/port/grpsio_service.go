// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// GrpsIOServiceReaderWriter provides access to service reading and writing operations
type GrpsIOServiceReaderWriter interface {
	GrpsIOServiceReader
	// GrpsIOServiceWriter will be added later when implementing CRUD operations

	// IsReady checks if the storage is ready by verifying the connection
	IsReady(ctx context.Context) error
}
