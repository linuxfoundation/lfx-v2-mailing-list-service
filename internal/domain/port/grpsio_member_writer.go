// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GroupsIOMailingListMemberWriter defines the application-level interface for GroupsIO member write operations.
type GroupsIOMailingListMemberWriter interface {
	// AddMember adds a new member to a mailing list.
	AddMember(ctx context.Context, mailingListID string, member *model.GrpsIOMember) (*model.GrpsIOMember, error)

	// UpdateMember updates an existing member in a mailing list.
	UpdateMember(ctx context.Context, mailingListID string, memberID string, member *model.GrpsIOMember) (*model.GrpsIOMember, error)

	// DeleteMember removes a member from a mailing list.
	DeleteMember(ctx context.Context, mailingListID string, memberID string) error

	// InviteMembers sends invitations to the given email addresses to join a mailing list.
	InviteMembers(ctx context.Context, mailingListID string, emails []string) error
}
