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
	// GetGrpsIOMailingList retrieves a mailing list by UID
	GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, error)

	// GetGrpsIOMailingListsByParent retrieves mailing lists by parent service ID
	GetGrpsIOMailingListsByParent(ctx context.Context, parentID string) ([]*model.GrpsIOMailingList, error)

	// GetGrpsIOMailingListsByCommittee retrieves mailing lists by committee ID
	GetGrpsIOMailingListsByCommittee(ctx context.Context, committeeID string) ([]*model.GrpsIOMailingList, error)

	// GetGrpsIOMailingListsByProject retrieves mailing lists by project ID
	GetGrpsIOMailingListsByProject(ctx context.Context, projectID string) ([]*model.GrpsIOMailingList, error)

	// CheckMailingListExists checks if a mailing list with the given name exists in parent service
	CheckMailingListExists(ctx context.Context, parentID, groupName string) (bool, error)
}
