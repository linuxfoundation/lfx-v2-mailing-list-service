// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// GroupsIOMailingListMemberReaderOrchestrator implements port.GroupsIOMailingListMemberReader
// by wrapping an inner GroupsIOMailingListMemberReader and forwarding requests.
// Member IDs are numeric strings assigned by Groups.io; no v1/v2 UUID translation is needed.
type GroupsIOMailingListMemberReaderOrchestrator struct {
	reader port.GroupsIOMailingListMemberReader
}

// MemberReaderOrchestratorOption configures a GroupsIOMailingListMemberReaderOrchestrator.
type MemberReaderOrchestratorOption func(*GroupsIOMailingListMemberReaderOrchestrator)

// WithMemberReader sets the underlying reader (e.g. the ITX proxy client).
func WithMemberReader(r port.GroupsIOMailingListMemberReader) MemberReaderOrchestratorOption {
	return func(o *GroupsIOMailingListMemberReaderOrchestrator) {
		o.reader = r
	}
}

// ListMembers lists all members of a mailing list.
func (o *GroupsIOMailingListMemberReaderOrchestrator) ListMembers(ctx context.Context, mailingListID string) ([]*model.GrpsIOMember, int, error) {
	return o.reader.ListMembers(ctx, mailingListID)
}

// GetMember retrieves a member by ID from a mailing list.
func (o *GroupsIOMailingListMemberReaderOrchestrator) GetMember(ctx context.Context, mailingListID string, memberID string) (*model.GrpsIOMember, error) {
	return o.reader.GetMember(ctx, mailingListID, memberID)
}

// CheckSubscriber checks whether an email is subscribed to a mailing list.
func (o *GroupsIOMailingListMemberReaderOrchestrator) CheckSubscriber(ctx context.Context, mailingListID string, email string) (bool, error) {
	return o.reader.CheckSubscriber(ctx, mailingListID, email)
}

// NewGroupsIOMailingListMemberReaderOrchestrator creates a new member reader orchestrator with the given options.
func NewGroupsIOMailingListMemberReaderOrchestrator(opts ...MemberReaderOrchestratorOption) port.GroupsIOMailingListMemberReader {
	o := &GroupsIOMailingListMemberReaderOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
