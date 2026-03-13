// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Member operations have no ID mapping — they delegate directly to the client.

func TestGroupsioMemberService_ListMembers(t *testing.T) {
	client := &mockMemberClient{
		listMembers: func(_ context.Context, subgroupID string) (*models.GroupsioMemberListResponse, error) {
			assert.Equal(t, "sg-42", subgroupID)
			return &models.GroupsioMemberListResponse{
				Items: []*models.GroupsioMember{
					{ID: "m-1", Email: "alice@example.com"},
					{ID: "m-2", Email: "bob@example.com"},
				},
				Total: 2,
			}, nil
		},
	}

	svc := NewGroupsioMemberService(client)
	resp, err := svc.ListMembers(context.Background(), "sg-42")

	require.NoError(t, err)
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, "alice@example.com", resp.Items[0].Email)
}

func TestGroupsioMemberService_AddMember(t *testing.T) {
	client := &mockMemberClient{
		addMember: func(_ context.Context, subgroupID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
			assert.Equal(t, "sg-42", subgroupID)
			assert.Equal(t, "new@example.com", req.Email)
			return &models.GroupsioMember{ID: "m-new", Email: req.Email}, nil
		},
	}

	svc := NewGroupsioMemberService(client)
	resp, err := svc.AddMember(context.Background(), "sg-42", &models.GroupsioMemberRequest{Email: "new@example.com"})

	require.NoError(t, err)
	assert.Equal(t, "m-new", resp.ID)
	assert.Equal(t, "new@example.com", resp.Email)
}

func TestGroupsioMemberService_GetMember(t *testing.T) {
	client := &mockMemberClient{
		getMember: func(_ context.Context, subgroupID, memberID string) (*models.GroupsioMember, error) {
			assert.Equal(t, "sg-42", subgroupID)
			assert.Equal(t, "m-7", memberID)
			return &models.GroupsioMember{ID: "m-7", Name: "Alice"}, nil
		},
	}

	svc := NewGroupsioMemberService(client)
	resp, err := svc.GetMember(context.Background(), "sg-42", "m-7")

	require.NoError(t, err)
	assert.Equal(t, "Alice", resp.Name)
}

func TestGroupsioMemberService_UpdateMember(t *testing.T) {
	client := &mockMemberClient{
		updateMember: func(_ context.Context, subgroupID, memberID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
			assert.Equal(t, "sg-42", subgroupID)
			assert.Equal(t, "m-7", memberID)
			return &models.GroupsioMember{ID: "m-7", Name: req.Name}, nil
		},
	}

	svc := NewGroupsioMemberService(client)
	resp, err := svc.UpdateMember(context.Background(), "sg-42", "m-7", &models.GroupsioMemberRequest{Name: "Updated"})

	require.NoError(t, err)
	assert.Equal(t, "Updated", resp.Name)
}

func TestGroupsioMemberService_DeleteMember(t *testing.T) {
	called := false
	client := &mockMemberClient{
		deleteMember: func(_ context.Context, subgroupID, memberID string) error {
			assert.Equal(t, "sg-42", subgroupID)
			assert.Equal(t, "m-7", memberID)
			called = true
			return nil
		},
	}

	svc := NewGroupsioMemberService(client)
	err := svc.DeleteMember(context.Background(), "sg-42", "m-7")

	require.NoError(t, err)
	assert.True(t, called)
}

func TestGroupsioMemberService_InviteMembers(t *testing.T) {
	client := &mockMemberClient{
		inviteMembers: func(_ context.Context, subgroupID string, req *models.GroupsioInviteMembersRequest) error {
			assert.Equal(t, "sg-42", subgroupID)
			assert.Equal(t, []string{"a@example.com", "b@example.com"}, req.Emails)
			return nil
		},
	}

	svc := NewGroupsioMemberService(client)
	err := svc.InviteMembers(context.Background(), "sg-42", &models.GroupsioInviteMembersRequest{
		Emails: []string{"a@example.com", "b@example.com"},
	})
	require.NoError(t, err)
}

func TestGroupsioMemberService_CheckSubscriber_Subscribed(t *testing.T) {
	client := &mockMemberClient{
		checkSubscriber: func(_ context.Context, req *models.GroupsioCheckSubscriberRequest) (*models.GroupsioCheckSubscriberResponse, error) {
			assert.Equal(t, "a@example.com", req.Email)
			assert.Equal(t, "sg-42", req.SubgroupID)
			return &models.GroupsioCheckSubscriberResponse{Subscribed: true}, nil
		},
	}

	svc := NewGroupsioMemberService(client)
	resp, err := svc.CheckSubscriber(context.Background(), &models.GroupsioCheckSubscriberRequest{
		Email:      "a@example.com",
		SubgroupID: "sg-42",
	})

	require.NoError(t, err)
	assert.True(t, resp.Subscribed)
}

func TestGroupsioMemberService_CheckSubscriber_NotSubscribed(t *testing.T) {
	client := &mockMemberClient{
		checkSubscriber: func(_ context.Context, _ *models.GroupsioCheckSubscriberRequest) (*models.GroupsioCheckSubscriberResponse, error) {
			return &models.GroupsioCheckSubscriberResponse{Subscribed: false}, nil
		},
	}

	svc := NewGroupsioMemberService(client)
	resp, err := svc.CheckSubscriber(context.Background(), &models.GroupsioCheckSubscriberRequest{
		Email: "b@example.com", SubgroupID: "sg-42",
	})

	require.NoError(t, err)
	assert.False(t, resp.Subscribed)
}

func TestGroupsioMemberService_ClientError_Propagated(t *testing.T) {
	clientErr := errors.New("member client error")
	client := &mockMemberClient{
		getMember: func(_ context.Context, _, _ string) (*models.GroupsioMember, error) {
			return nil, clientErr
		},
	}

	svc := NewGroupsioMemberService(client)
	_, err := svc.GetMember(context.Background(), "sg-42", "m-7")

	require.Error(t, err)
	assert.True(t, errors.Is(err, clientErr))
}
