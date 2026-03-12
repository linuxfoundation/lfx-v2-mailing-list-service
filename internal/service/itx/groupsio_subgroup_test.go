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

const (
	commV2 = "comm-uuid-bbbbbbbb"
	commV1 = "comm-sfid-00000002"
)

var subgroupV2ToV1 = map[string]string{projV2: projV1, commV2: commV1}
var subgroupV1ToV2 = map[string]string{projV1: projV2, commV1: commV2}

func TestGroupsioSubgroupService_ListSubgroups_BothFilters(t *testing.T) {
	var receivedProjectID, receivedCommitteeID string
	client := &mockSubgroupClient{
		listSubgroups: func(_ context.Context, projectID, committeeID string) (*models.GroupsioSubgroupListResponse, error) {
			receivedProjectID = projectID
			receivedCommitteeID = committeeID
			return &models.GroupsioSubgroupListResponse{
				Items: []*models.GroupsioSubgroup{
					{ID: "sg-1", ProjectID: projV1, CommitteeID: commV1},
				},
				Meta: models.GroupsioSubgroupMeta{TotalResults: 1},
			}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, swappingMapper(subgroupV2ToV1, subgroupV1ToV2))
	resp, err := svc.ListSubgroups(context.Background(), projV2, commV2)

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedProjectID, "v2 project UID should map to v1 SFID")
	assert.Equal(t, commV1, receivedCommitteeID, "v2 committee UID should map to v1 SFID")
	require.Len(t, resp.Items, 1)
	assert.Equal(t, projV2, resp.Items[0].ProjectID, "v1 project_id in response should map back to v2")
	assert.Equal(t, commV2, resp.Items[0].CommitteeID, "v1 committee_id in response should map back to v2")
}

func TestGroupsioSubgroupService_ListSubgroups_NoFilters(t *testing.T) {
	client := &mockSubgroupClient{
		listSubgroups: func(_ context.Context, projectID, committeeID string) (*models.GroupsioSubgroupListResponse, error) {
			assert.Empty(t, projectID)
			assert.Empty(t, committeeID)
			return &models.GroupsioSubgroupListResponse{}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, passthroughMapper())
	_, err := svc.ListSubgroups(context.Background(), "", "")
	require.NoError(t, err)
}

func TestGroupsioSubgroupService_CreateSubgroup(t *testing.T) {
	var receivedReq *models.GroupsioSubgroupRequest
	client := &mockSubgroupClient{
		createSubgroup: func(_ context.Context, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
			receivedReq = req
			return &models.GroupsioSubgroup{
				ID:          "sg-new",
				ProjectID:   req.ProjectID,
				CommitteeID: req.CommitteeID,
				Name:        req.Name,
			}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, swappingMapper(subgroupV2ToV1, subgroupV1ToV2))
	resp, err := svc.CreateSubgroup(context.Background(), &models.GroupsioSubgroupRequest{
		ProjectID:   projV2,
		CommitteeID: commV2,
		Name:        "dev-list",
	})

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedReq.ProjectID, "request should carry v1 project SFID")
	assert.Equal(t, commV1, receivedReq.CommitteeID, "request should carry v1 committee SFID")
	assert.Equal(t, projV2, resp.ProjectID, "response should have v2 project UUID")
	assert.Equal(t, commV2, resp.CommitteeID, "response should have v2 committee UUID")
	assert.Equal(t, "dev-list", resp.Name)
}

func TestGroupsioSubgroupService_CreateSubgroup_EmptyIDs(t *testing.T) {
	client := &mockSubgroupClient{
		createSubgroup: func(_ context.Context, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
			assert.Empty(t, req.ProjectID)
			assert.Empty(t, req.CommitteeID)
			return &models.GroupsioSubgroup{ID: "sg-new"}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, passthroughMapper())
	resp, err := svc.CreateSubgroup(context.Background(), &models.GroupsioSubgroupRequest{})
	require.NoError(t, err)
	assert.Equal(t, "sg-new", resp.ID)
}

func TestGroupsioSubgroupService_GetSubgroup(t *testing.T) {
	client := &mockSubgroupClient{
		getSubgroup: func(_ context.Context, subgroupID string) (*models.GroupsioSubgroup, error) {
			assert.Equal(t, "sg-42", subgroupID)
			return &models.GroupsioSubgroup{
				ID:          "sg-42",
				ProjectID:   projV1,
				CommitteeID: commV1,
			}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, swappingMapper(subgroupV2ToV1, subgroupV1ToV2))
	resp, err := svc.GetSubgroup(context.Background(), "sg-42")

	require.NoError(t, err)
	assert.Equal(t, "sg-42", resp.ID)
	assert.Equal(t, projV2, resp.ProjectID)
	assert.Equal(t, commV2, resp.CommitteeID)
}

func TestGroupsioSubgroupService_UpdateSubgroup(t *testing.T) {
	var receivedReq *models.GroupsioSubgroupRequest
	client := &mockSubgroupClient{
		updateSubgroup: func(_ context.Context, subgroupID string, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
			assert.Equal(t, "sg-42", subgroupID)
			receivedReq = req
			return &models.GroupsioSubgroup{
				ID:          "sg-42",
				ProjectID:   req.ProjectID,
				CommitteeID: req.CommitteeID,
			}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, swappingMapper(subgroupV2ToV1, subgroupV1ToV2))
	resp, err := svc.UpdateSubgroup(context.Background(), "sg-42", &models.GroupsioSubgroupRequest{
		ProjectID:   projV2,
		CommitteeID: commV2,
	})

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedReq.ProjectID)
	assert.Equal(t, commV1, receivedReq.CommitteeID)
	assert.Equal(t, projV2, resp.ProjectID)
	assert.Equal(t, commV2, resp.CommitteeID)
}

func TestGroupsioSubgroupService_DeleteSubgroup(t *testing.T) {
	called := false
	client := &mockSubgroupClient{
		deleteSubgroup: func(_ context.Context, subgroupID string) error {
			assert.Equal(t, "sg-42", subgroupID)
			called = true
			return nil
		},
	}

	svc := NewGroupsioSubgroupService(client, passthroughMapper())
	err := svc.DeleteSubgroup(context.Background(), "sg-42")
	require.NoError(t, err)
	assert.True(t, called)
}

func TestGroupsioSubgroupService_GetSubgroupCount(t *testing.T) {
	var receivedProjectID string
	client := &mockSubgroupClient{
		getSubgroupCount: func(_ context.Context, projectID string) (*models.GroupsioSubgroupCountResponse, error) {
			receivedProjectID = projectID
			return &models.GroupsioSubgroupCountResponse{Count: 7}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, swappingMapper(subgroupV2ToV1, subgroupV1ToV2))
	resp, err := svc.GetSubgroupCount(context.Background(), projV2)

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedProjectID, "v2 project UID should be mapped to v1 before calling client")
	assert.Equal(t, 7, resp.Count)
}

func TestGroupsioSubgroupService_GetMemberCount(t *testing.T) {
	client := &mockSubgroupClient{
		getMemberCount: func(_ context.Context, subgroupID string) (*models.GroupsioMemberCountResponse, error) {
			assert.Equal(t, "sg-42", subgroupID)
			return &models.GroupsioMemberCountResponse{Count: 15}, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, passthroughMapper())
	resp, err := svc.GetMemberCount(context.Background(), "sg-42")

	require.NoError(t, err)
	assert.Equal(t, 15, resp.Count)
}

func TestGroupsioSubgroupService_CommitteeMapperError(t *testing.T) {
	mapErr := errors.New("committee mapping failed")
	mapper := &mockIDMapper{
		mapProjectV2ToV1:   func(_ context.Context, id string) (string, error) { return id, nil },
		mapProjectV1ToV2:   func(_ context.Context, id string) (string, error) { return id, nil },
		mapCommitteeV2ToV1: func(_ context.Context, _ string) (string, error) { return "", mapErr },
		mapCommitteeV1ToV2: func(_ context.Context, id string) (string, error) { return id, nil },
	}
	client := &mockSubgroupClient{
		createSubgroup: func(_ context.Context, _ *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
			t.Fatal("client should not be called when mapper fails")
			return nil, nil
		},
	}

	svc := NewGroupsioSubgroupService(client, mapper)
	_, err := svc.CreateSubgroup(context.Background(), &models.GroupsioSubgroupRequest{
		CommitteeID: commV2,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, mapErr))
}
