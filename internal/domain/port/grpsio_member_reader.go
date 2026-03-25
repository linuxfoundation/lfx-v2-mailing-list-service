// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GroupsIOMailingListMemberReader defines the application-level interface for GroupsIO member read operations.
type GroupsIOMailingListMemberReader interface {
	// ListMembers lists all members of a mailing list.
	ListMembers(ctx context.Context, mailingListID string) ([]*model.GrpsIOMember, int, error)

	// GetMember retrieves a member by ID from a mailing list.
	GetMember(ctx context.Context, mailingListID string, memberID string) (*model.GrpsIOMember, error)

	// CheckSubscriber checks whether an email is subscribed to a mailing list.
	CheckSubscriber(ctx context.Context, mailingListID string, email string) (bool, error)
}
