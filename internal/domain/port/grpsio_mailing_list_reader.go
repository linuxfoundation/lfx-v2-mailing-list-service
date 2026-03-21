// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GroupsIOMailingListReader defines the application-level interface for GroupsIO mailing list read operations.
// All IDs are v2 UUIDs. Implementations are responsible for v1/v2 ID translation
// when communicating with the ITX proxy.
type GroupsIOMailingListReader interface {
	// ListMailingLists lists mailing lists, optionally filtered by project UID and/or committee UID.
	// Returns the matched mailing lists and total count.
	ListMailingLists(ctx context.Context, projectUID string, committeeUID string) ([]*model.GroupsIOMailingList, int, error)

	// GetMailingList retrieves a mailing list by ID.
	GetMailingList(ctx context.Context, mailingListID string) (*model.GroupsIOMailingList, error)

	// GetMailingListCount returns the count of mailing lists for a given project UID.
	GetMailingListCount(ctx context.Context, projectUID string) (int, error)

	// GetMailingListMemberCount returns the count of members in a given mailing list.
	GetMailingListMemberCount(ctx context.Context, mailingListID string) (int, error)
}
