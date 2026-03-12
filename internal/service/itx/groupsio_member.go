// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
)

// GroupsioMemberService handles ITX GroupsIO member operations (pass-through, no ID mapping needed).
type GroupsioMemberService struct {
	client domain.GroupsioMemberClient
}

// NewGroupsioMemberService creates a new GroupsIO member handler.
func NewGroupsioMemberService(client domain.GroupsioMemberClient) *GroupsioMemberService {
	return &GroupsioMemberService{
		client: client,
	}
}

// ListMembers lists members of a subgroup.
func (s *GroupsioMemberService) ListMembers(ctx context.Context, subgroupID string) (*models.GroupsioMemberListResponse, error) {
	return s.client.ListMembers(ctx, subgroupID)
}

// AddMember adds a member to a subgroup.
func (s *GroupsioMemberService) AddMember(ctx context.Context, subgroupID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
	return s.client.AddMember(ctx, subgroupID, req)
}

// GetMember retrieves a member by ID.
func (s *GroupsioMemberService) GetMember(ctx context.Context, subgroupID, memberID string) (*models.GroupsioMember, error) {
	return s.client.GetMember(ctx, subgroupID, memberID)
}

// UpdateMember updates a member.
func (s *GroupsioMemberService) UpdateMember(ctx context.Context, subgroupID, memberID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
	return s.client.UpdateMember(ctx, subgroupID, memberID, req)
}

// DeleteMember removes a member from a subgroup.
func (s *GroupsioMemberService) DeleteMember(ctx context.Context, subgroupID, memberID string) error {
	return s.client.DeleteMember(ctx, subgroupID, memberID)
}

// InviteMembers sends invitations to multiple email addresses.
func (s *GroupsioMemberService) InviteMembers(ctx context.Context, subgroupID string, req *models.GroupsioInviteMembersRequest) error {
	return s.client.InviteMembers(ctx, subgroupID, req)
}

// CheckSubscriber checks if an email is subscribed to a subgroup.
func (s *GroupsioMemberService) CheckSubscriber(ctx context.Context, req *models.GroupsioCheckSubscriberRequest) (*models.GroupsioCheckSubscriberResponse, error) {
	return s.client.CheckSubscriber(ctx, req)
}
