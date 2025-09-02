// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOMailingListWriter defines the interface for writing mailing list data
type GrpsIOMailingListWriter interface {
	BaseGrpsIOWriter

	// CreateGrpsIOMailingList creates a new GroupsIO mailing list and returns the mailing list with revision
	CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error)

	// UpdateGrpsIOMailingList updates an existing GroupsIO mailing list with optimistic concurrency control
	UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error)

	// DeleteGrpsIOMailingList deletes a GroupsIO mailing list with optimistic concurrency control
	DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64) error

	// CreateSecondaryIndices creates secondary indices for a mailing list and returns the created keys
	CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error)

	// UniqueMailingListGroupName validates that group name is unique within parent service
	UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error)
}
