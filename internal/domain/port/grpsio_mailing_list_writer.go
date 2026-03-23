// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GroupsIOMailingListWriter defines the application-level interface for GroupsIO mailing list operations.
// All IDs are v2 UUIDs. Implementations are responsible for v1/v2 ID translation
// when communicating with the ITX proxy.
type GroupsIOMailingListWriter interface {
	// CreateMailingList creates a new mailing list.
	CreateMailingList(ctx context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error)

	// UpdateMailingList updates an existing mailing list.
	UpdateMailingList(ctx context.Context, mailingListID string, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error)

	// DeleteMailingList deletes a mailing list.
	DeleteMailingList(ctx context.Context, mailingListID string) error
}