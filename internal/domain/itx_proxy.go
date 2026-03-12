// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
)

// GroupsioServiceClient defines the interface for ITX GroupsIO service operations.
type GroupsioServiceClient interface {
	// ListServices lists GroupsIO services, optionally filtered by project_id (v1 SFID)
	ListServices(ctx context.Context, projectID string) (*models.GroupsioServiceListResponse, error)

	// CreateService creates a new GroupsIO service
	CreateService(ctx context.Context, req *models.GroupsioServiceRequest) (*models.GroupsioService, error)

	// GetService retrieves a GroupsIO service by ID
	GetService(ctx context.Context, serviceID string) (*models.GroupsioService, error)

	// UpdateService updates a GroupsIO service
	UpdateService(ctx context.Context, serviceID string, req *models.GroupsioServiceRequest) (*models.GroupsioService, error)

	// DeleteService deletes a GroupsIO service
	DeleteService(ctx context.Context, serviceID string) error

	// GetProjects returns projects that have GroupsIO services
	GetProjects(ctx context.Context) (*models.GroupsioServiceProjectsResponse, error)

	// FindParentService finds the parent service for a project (v1 SFID)
	FindParentService(ctx context.Context, projectID string) (*models.GroupsioService, error)
}

// GroupsioSubgroupClient defines the interface for ITX GroupsIO subgroup operations.
type GroupsioSubgroupClient interface {
	// ListSubgroups lists subgroups, optionally filtered by project_id and/or committee_id (v1 SFIDs)
	ListSubgroups(ctx context.Context, projectID, committeeID string) (*models.GroupsioSubgroupListResponse, error)

	// CreateSubgroup creates a new subgroup
	CreateSubgroup(ctx context.Context, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error)

	// GetSubgroup retrieves a subgroup by ID
	GetSubgroup(ctx context.Context, subgroupID string) (*models.GroupsioSubgroup, error)

	// UpdateSubgroup updates a subgroup
	UpdateSubgroup(ctx context.Context, subgroupID string, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error)

	// DeleteSubgroup deletes a subgroup
	DeleteSubgroup(ctx context.Context, subgroupID string) error

	// GetSubgroupCount returns the count of subgroups for a project (v1 SFID)
	GetSubgroupCount(ctx context.Context, projectID string) (*models.GroupsioSubgroupCountResponse, error)

	// GetMemberCount returns the count of members in a subgroup
	GetMemberCount(ctx context.Context, subgroupID string) (*models.GroupsioMemberCountResponse, error)
}

// GroupsioMemberClient defines the interface for ITX GroupsIO member operations.
type GroupsioMemberClient interface {
	// ListMembers lists members of a subgroup
	ListMembers(ctx context.Context, subgroupID string) (*models.GroupsioMemberListResponse, error)

	// AddMember adds a member to a subgroup
	AddMember(ctx context.Context, subgroupID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error)

	// GetMember retrieves a member by ID
	GetMember(ctx context.Context, subgroupID, memberID string) (*models.GroupsioMember, error)

	// UpdateMember updates a member
	UpdateMember(ctx context.Context, subgroupID, memberID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error)

	// DeleteMember removes a member from a subgroup
	DeleteMember(ctx context.Context, subgroupID, memberID string) error

	// InviteMembers sends invitations to multiple email addresses
	InviteMembers(ctx context.Context, subgroupID string, req *models.GroupsioInviteMembersRequest) error

	// CheckSubscriber checks if an email is subscribed to a subgroup
	CheckSubscriber(ctx context.Context, req *models.GroupsioCheckSubscriberRequest) (*models.GroupsioCheckSubscriberResponse, error)
}

// ITXGroupsioClient combines all GroupsIO client interfaces.
type ITXGroupsioClient interface {
	GroupsioServiceClient
	GroupsioSubgroupClient
	GroupsioMemberClient
}
