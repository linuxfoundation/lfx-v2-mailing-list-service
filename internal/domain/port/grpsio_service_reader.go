// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOServiceReader defines the interface for service read operations
type GrpsIOServiceReader interface {
	// GetGrpsIOService retrieves a single service by ID and returns ETag revision
	GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error)
}
