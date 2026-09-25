// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// GroupsIOMailingListMemberWriterOrchestrator implements port.GroupsIOMailingListMemberWriter
// by wrapping an inner GroupsIOMailingListMemberWriter and forwarding requests.
// Member IDs are numeric strings assigned by Groups.io; no v1/v2 UUID translation is needed.
type GroupsIOMailingListMemberWriterOrchestrator struct {
	writer port.GroupsIOMailingListMemberWriter
}

// MemberWriterOrchestratorOption configures a GroupsIOMailingListMemberWriterOrchestrator.
type MemberWriterOrchestratorOption func(*GroupsIOMailingListMemberWriterOrchestrator)

// WithMemberWriter sets the underlying writer (e.g. the ITX proxy client).
func WithMemberWriter(w port.GroupsIOMailingListMemberWriter) MemberWriterOrchestratorOption {
	return func(o *GroupsIOMailingListMemberWriterOrchestrator) {
		o.writer = w
	}
}

// AddMember adds a new member to a mailing list.
func (o *GroupsIOMailingListMemberWriterOrchestrator) AddMember(ctx context.Context, mailingListID string, member *model.GrpsIOMember) (*model.GrpsIOMember, error) {
	return o.writer.AddMember(ctx, mailingListID, member)
}

// UpdateMember updates an existing member in a mailing list.
func (o *GroupsIOMailingListMemberWriterOrchestrator) UpdateMember(ctx context.Context, mailingListID string, memberID string, member *model.GrpsIOMember) (*model.GrpsIOMember, error) {
	return o.writer.UpdateMember(ctx, mailingListID, memberID, member)
}

// DeleteMember removes a member from a mailing list.
func (o *GroupsIOMailingListMemberWriterOrchestrator) DeleteMember(ctx context.Context, mailingListID string, memberID string) error {
	return o.writer.DeleteMember(ctx, mailingListID, memberID)
}

// InviteMembers sends invitations to the given email addresses to join a mailing list.
func (o *GroupsIOMailingListMemberWriterOrchestrator) InviteMembers(ctx context.Context, mailingListID string, emails []string) error {
	return o.writer.InviteMembers(ctx, mailingListID, emails)
}

// NewGroupsIOMailingListMemberWriterOrchestrator creates a new member writer orchestrator with the given options.
func NewGroupsIOMailingListMemberWriterOrchestrator(opts ...MemberWriterOrchestratorOption) port.GroupsIOMailingListMemberWriter {
	o := &GroupsIOMailingListMemberWriterOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
