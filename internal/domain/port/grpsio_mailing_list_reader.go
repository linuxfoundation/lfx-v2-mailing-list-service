// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOMailingListReader defines the interface for reading mailing list data
type GrpsIOMailingListReader interface {
	// GetGrpsIOMailingList retrieves a mailing list by UID with revision
	GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, uint64, error)

	// GetMailingListRevision retrieves only the revision for a given UID
	GetMailingListRevision(ctx context.Context, uid string) (uint64, error)


	// CheckMailingListExists checks if a mailing list with the given name exists in parent service
	CheckMailingListExists(ctx context.Context, parentID, groupName string) (bool, error)
}
