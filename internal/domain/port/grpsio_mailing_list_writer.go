// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOMailingListWriter defines the interface for writing mailing list data
type GrpsIOMailingListWriter interface {
	// CreateGrpsIOMailingList creates a new GroupsIO mailing list with secondary indices
	CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error)

	// UpdateGrpsIOMailingList updates an existing GroupsIO mailing list
	UpdateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error)

	// DeleteGrpsIOMailingList deletes a GroupsIO mailing list and its indices
	DeleteGrpsIOMailingList(ctx context.Context, uid string) error

	// UpdateSecondaryIndices updates secondary indices for a mailing list
	UpdateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) error

	// UniqueMailingListGroupName validates that group name is unique within parent service
	UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error)

	// GetKeyRevision retrieves the revision for a given key (used for cleanup operations)
	GetKeyRevision(ctx context.Context, key string) (uint64, error)

	// Delete removes a key with the given revision (used for cleanup and rollback)
	Delete(ctx context.Context, key string, revision uint64) error
}
