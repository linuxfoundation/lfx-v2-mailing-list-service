// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
)

// mockIDMapper is a controllable IDMapper for tests.
type mockIDMapper struct {
	mapProjectV2ToV1  func(ctx context.Context, v2UID string) (string, error)
	mapProjectV1ToV2  func(ctx context.Context, v1SFID string) (string, error)
	mapCommitteeV2ToV1 func(ctx context.Context, v2UID string) (string, error)
	mapCommitteeV1ToV2 func(ctx context.Context, v1SFID string) (string, error)
}

func (m *mockIDMapper) MapProjectV2ToV1(ctx context.Context, v2UID string) (string, error) {
	return m.mapProjectV2ToV1(ctx, v2UID)
}
func (m *mockIDMapper) MapProjectV1ToV2(ctx context.Context, v1SFID string) (string, error) {
	return m.mapProjectV1ToV2(ctx, v1SFID)
}
func (m *mockIDMapper) MapCommitteeV2ToV1(ctx context.Context, v2UID string) (string, error) {
	return m.mapCommitteeV2ToV1(ctx, v2UID)
}
func (m *mockIDMapper) MapCommitteeV1ToV2(ctx context.Context, v1SFID string) (string, error) {
	return m.mapCommitteeV1ToV2(ctx, v1SFID)
}

// passthroughMapper returns input IDs unchanged (like NoOpMapper).
func passthroughMapper() *mockIDMapper {
	identity := func(_ context.Context, id string) (string, error) { return id, nil }
	return &mockIDMapper{
		mapProjectV2ToV1:   identity,
		mapProjectV1ToV2:   identity,
		mapCommitteeV2ToV1: identity,
		mapCommitteeV1ToV2: identity,
	}
}

// swappingMapper maps v2→v1 and v1→v2 using provided lookup tables.
func swappingMapper(v2ToV1, v1ToV2 map[string]string) *mockIDMapper {
	lookup := func(table map[string]string) func(_ context.Context, id string) (string, error) {
		return func(_ context.Context, id string) (string, error) {
			if v, ok := table[id]; ok {
				return v, nil
			}
			return id, nil
		}
	}
	return &mockIDMapper{
		mapProjectV2ToV1:   lookup(v2ToV1),
		mapProjectV1ToV2:   lookup(v1ToV2),
		mapCommitteeV2ToV1: lookup(v2ToV1),
		mapCommitteeV1ToV2: lookup(v1ToV2),
	}
}

// mockServiceClient implements domain.GroupsioServiceClient for tests.
type mockServiceClient struct {
	listServices      func(ctx context.Context, projectID string) (*models.GroupsioServiceListResponse, error)
	createService     func(ctx context.Context, req *models.GroupsioServiceRequest) (*models.GroupsioService, error)
	getService        func(ctx context.Context, serviceID string) (*models.GroupsioService, error)
	updateService     func(ctx context.Context, serviceID string, req *models.GroupsioServiceRequest) (*models.GroupsioService, error)
	deleteService     func(ctx context.Context, serviceID string) error
	getProjects       func(ctx context.Context) (*models.GroupsioServiceProjectsResponse, error)
	findParentService func(ctx context.Context, projectID string) (*models.GroupsioService, error)
}

func (m *mockServiceClient) ListServices(ctx context.Context, projectID string) (*models.GroupsioServiceListResponse, error) {
	return m.listServices(ctx, projectID)
}
func (m *mockServiceClient) CreateService(ctx context.Context, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
	return m.createService(ctx, req)
}
func (m *mockServiceClient) GetService(ctx context.Context, serviceID string) (*models.GroupsioService, error) {
	return m.getService(ctx, serviceID)
}
func (m *mockServiceClient) UpdateService(ctx context.Context, serviceID string, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
	return m.updateService(ctx, serviceID, req)
}
func (m *mockServiceClient) DeleteService(ctx context.Context, serviceID string) error {
	return m.deleteService(ctx, serviceID)
}
func (m *mockServiceClient) GetProjects(ctx context.Context) (*models.GroupsioServiceProjectsResponse, error) {
	return m.getProjects(ctx)
}
func (m *mockServiceClient) FindParentService(ctx context.Context, projectID string) (*models.GroupsioService, error) {
	return m.findParentService(ctx, projectID)
}

// mockSubgroupClient implements domain.GroupsioSubgroupClient for tests.
type mockSubgroupClient struct {
	listSubgroups    func(ctx context.Context, projectID, committeeID string) (*models.GroupsioSubgroupListResponse, error)
	createSubgroup   func(ctx context.Context, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error)
	getSubgroup      func(ctx context.Context, subgroupID string) (*models.GroupsioSubgroup, error)
	updateSubgroup   func(ctx context.Context, subgroupID string, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error)
	deleteSubgroup   func(ctx context.Context, subgroupID string) error
	getSubgroupCount func(ctx context.Context, projectID string) (*models.GroupsioSubgroupCountResponse, error)
	getMemberCount   func(ctx context.Context, subgroupID string) (*models.GroupsioMemberCountResponse, error)
}

func (m *mockSubgroupClient) ListSubgroups(ctx context.Context, projectID, committeeID string) (*models.GroupsioSubgroupListResponse, error) {
	return m.listSubgroups(ctx, projectID, committeeID)
}
func (m *mockSubgroupClient) CreateSubgroup(ctx context.Context, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
	return m.createSubgroup(ctx, req)
}
func (m *mockSubgroupClient) GetSubgroup(ctx context.Context, subgroupID string) (*models.GroupsioSubgroup, error) {
	return m.getSubgroup(ctx, subgroupID)
}
func (m *mockSubgroupClient) UpdateSubgroup(ctx context.Context, subgroupID string, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
	return m.updateSubgroup(ctx, subgroupID, req)
}
func (m *mockSubgroupClient) DeleteSubgroup(ctx context.Context, subgroupID string) error {
	return m.deleteSubgroup(ctx, subgroupID)
}
func (m *mockSubgroupClient) GetSubgroupCount(ctx context.Context, projectID string) (*models.GroupsioSubgroupCountResponse, error) {
	return m.getSubgroupCount(ctx, projectID)
}
func (m *mockSubgroupClient) GetMemberCount(ctx context.Context, subgroupID string) (*models.GroupsioMemberCountResponse, error) {
	return m.getMemberCount(ctx, subgroupID)
}

// mockMemberClient implements domain.GroupsioMemberClient for tests.
type mockMemberClient struct {
	listMembers      func(ctx context.Context, subgroupID string) (*models.GroupsioMemberListResponse, error)
	addMember        func(ctx context.Context, subgroupID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error)
	getMember        func(ctx context.Context, subgroupID, memberID string) (*models.GroupsioMember, error)
	updateMember     func(ctx context.Context, subgroupID, memberID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error)
	deleteMember     func(ctx context.Context, subgroupID, memberID string) error
	inviteMembers    func(ctx context.Context, subgroupID string, req *models.GroupsioInviteMembersRequest) error
	checkSubscriber  func(ctx context.Context, req *models.GroupsioCheckSubscriberRequest) (*models.GroupsioCheckSubscriberResponse, error)
}

func (m *mockMemberClient) ListMembers(ctx context.Context, subgroupID string) (*models.GroupsioMemberListResponse, error) {
	return m.listMembers(ctx, subgroupID)
}
func (m *mockMemberClient) AddMember(ctx context.Context, subgroupID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
	return m.addMember(ctx, subgroupID, req)
}
func (m *mockMemberClient) GetMember(ctx context.Context, subgroupID, memberID string) (*models.GroupsioMember, error) {
	return m.getMember(ctx, subgroupID, memberID)
}
func (m *mockMemberClient) UpdateMember(ctx context.Context, subgroupID, memberID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
	return m.updateMember(ctx, subgroupID, memberID, req)
}
func (m *mockMemberClient) DeleteMember(ctx context.Context, subgroupID, memberID string) error {
	return m.deleteMember(ctx, subgroupID, memberID)
}
func (m *mockMemberClient) InviteMembers(ctx context.Context, subgroupID string, req *models.GroupsioInviteMembersRequest) error {
	return m.inviteMembers(ctx, subgroupID, req)
}
func (m *mockMemberClient) CheckSubscriber(ctx context.Context, req *models.GroupsioCheckSubscriberRequest) (*models.GroupsioCheckSubscriberResponse, error) {
	return m.checkSubscriber(ctx, req)
}
