// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOMailingListRepository defines the interface for mailing list data persistence.
// This interface represents pure storage operations without orchestration logic.
// Implementations should handle data persistence, constraints, and indexing
// but not business logic like Groups.io API coordination or message publishing.
//
// This interface follows the Repository pattern and should be implemented by:
//   - NATS storage layer (production)
//   - Mock storage layer (testing)
//
// For business logic orchestration, see service.GrpsIOMailingListWriter.
type GrpsIOMailingListRepository interface {
	BaseGrpsIOWriter

	// CreateGrpsIOMailingList creates a new GroupsIO mailing list and returns the mailing list with revision
	CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error)

	// UpdateGrpsIOMailingList updates an existing GroupsIO mailing list with optimistic concurrency control
	UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error)

	// DeleteGrpsIOMailingList deletes a GroupsIO mailing list with optimistic concurrency control
	DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error

	// CreateSecondaryIndices creates secondary indices for a mailing list and returns the created keys
	CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error)

	// UniqueMailingListGroupName validates that group name is unique within parent service
	UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error)

	// CreateGrpsIOMailingListSettings creates new mailing list settings and returns the settings with revision
	CreateGrpsIOMailingListSettings(ctx context.Context, settings *model.GrpsIOMailingListSettings) (*model.GrpsIOMailingListSettings, uint64, error)

	// UpdateGrpsIOMailingListSettings updates mailing list settings with expected revision and returns updated settings with new revision
	UpdateGrpsIOMailingListSettings(ctx context.Context, settings *model.GrpsIOMailingListSettings, expectedRevision uint64) (*model.GrpsIOMailingListSettings, uint64, error)
}
