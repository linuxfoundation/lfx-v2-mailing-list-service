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
	projV2 = "proj-uuid-aaaaaaaa"
	projV1 = "proj-sfid-00000001"
)

var v2ToV1 = map[string]string{projV2: projV1}
var v1ToV2 = map[string]string{projV1: projV2}

func TestGroupsioServiceService_ListServices_NoFilter(t *testing.T) {
	var calledWithProjectID string
	client := &mockServiceClient{
		listServices: func(_ context.Context, projectID string) (*models.GroupsioServiceListResponse, error) {
			calledWithProjectID = projectID
			return &models.GroupsioServiceListResponse{
				Items: []*models.GroupsioService{{ID: "svc-1", ProjectID: projV1}},
				Total: 1,
			}, nil
		},
	}

	svc := NewGroupsioServiceService(client, swappingMapper(v2ToV1, v1ToV2))
	resp, err := svc.ListServices(context.Background(), "")

	require.NoError(t, err)
	assert.Empty(t, calledWithProjectID, "empty filter should not call mapper")
	require.Len(t, resp.Items, 1)
	assert.Equal(t, projV2, resp.Items[0].ProjectID, "v1 project_id in response should be mapped to v2 UUID")
}

func TestGroupsioServiceService_ListServices_WithProjectFilter(t *testing.T) {
	var receivedV1ID string
	client := &mockServiceClient{
		listServices: func(_ context.Context, projectID string) (*models.GroupsioServiceListResponse, error) {
			receivedV1ID = projectID
			return &models.GroupsioServiceListResponse{
				Items: []*models.GroupsioService{{ID: "svc-1", ProjectID: projV1}},
			}, nil
		},
	}

	svc := NewGroupsioServiceService(client, swappingMapper(v2ToV1, v1ToV2))
	resp, err := svc.ListServices(context.Background(), projV2)

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedV1ID, "v2 project UID should be mapped to v1 before calling client")
	assert.Equal(t, projV2, resp.Items[0].ProjectID, "v1 project_id in response should be mapped back to v2")
}

func TestGroupsioServiceService_ListServices_EmptyProjectIDInResponse(t *testing.T) {
	client := &mockServiceClient{
		listServices: func(_ context.Context, _ string) (*models.GroupsioServiceListResponse, error) {
			return &models.GroupsioServiceListResponse{
				Items: []*models.GroupsioService{{ID: "svc-no-project"}},
			}, nil
		},
	}

	svc := NewGroupsioServiceService(client, passthroughMapper())
	resp, err := svc.ListServices(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, resp.Items[0].ProjectID)
}

func TestGroupsioServiceService_CreateService(t *testing.T) {
	var receivedReq *models.GroupsioServiceRequest
	client := &mockServiceClient{
		createService: func(_ context.Context, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
			receivedReq = req
			return &models.GroupsioService{ID: "svc-new", ProjectID: req.ProjectID}, nil
		},
	}

	svc := NewGroupsioServiceService(client, swappingMapper(v2ToV1, v1ToV2))
	resp, err := svc.CreateService(context.Background(), &models.GroupsioServiceRequest{
		ProjectID: projV2,
		Type:      "primary",
	})

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedReq.ProjectID, "request should use v1 SFID")
	assert.Equal(t, projV2, resp.ProjectID, "response should use v2 UUID")
}

func TestGroupsioServiceService_CreateService_EmptyProjectID(t *testing.T) {
	client := &mockServiceClient{
		createService: func(_ context.Context, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
			assert.Empty(t, req.ProjectID)
			return &models.GroupsioService{ID: "svc-new"}, nil
		},
	}

	svc := NewGroupsioServiceService(client, passthroughMapper())
	resp, err := svc.CreateService(context.Background(), &models.GroupsioServiceRequest{})
	require.NoError(t, err)
	assert.Equal(t, "svc-new", resp.ID)
}

func TestGroupsioServiceService_GetService(t *testing.T) {
	client := &mockServiceClient{
		getService: func(_ context.Context, serviceID string) (*models.GroupsioService, error) {
			assert.Equal(t, "svc-42", serviceID)
			return &models.GroupsioService{ID: serviceID, ProjectID: projV1}, nil
		},
	}

	svc := NewGroupsioServiceService(client, swappingMapper(v2ToV1, v1ToV2))
	resp, err := svc.GetService(context.Background(), "svc-42")

	require.NoError(t, err)
	assert.Equal(t, "svc-42", resp.ID)
	assert.Equal(t, projV2, resp.ProjectID)
}

func TestGroupsioServiceService_UpdateService(t *testing.T) {
	var receivedReq *models.GroupsioServiceRequest
	client := &mockServiceClient{
		updateService: func(_ context.Context, serviceID string, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
			assert.Equal(t, "svc-42", serviceID)
			receivedReq = req
			return &models.GroupsioService{ID: serviceID, ProjectID: req.ProjectID}, nil
		},
	}

	svc := NewGroupsioServiceService(client, swappingMapper(v2ToV1, v1ToV2))
	resp, err := svc.UpdateService(context.Background(), "svc-42", &models.GroupsioServiceRequest{
		ProjectID: projV2,
		Status:    "active",
	})

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedReq.ProjectID)
	assert.Equal(t, "active", receivedReq.Status)
	assert.Equal(t, projV2, resp.ProjectID)
}

func TestGroupsioServiceService_DeleteService(t *testing.T) {
	called := false
	client := &mockServiceClient{
		deleteService: func(_ context.Context, serviceID string) error {
			assert.Equal(t, "svc-42", serviceID)
			called = true
			return nil
		},
	}

	svc := NewGroupsioServiceService(client, passthroughMapper())
	err := svc.DeleteService(context.Background(), "svc-42")

	require.NoError(t, err)
	assert.True(t, called)
}

func TestGroupsioServiceService_GetProjects(t *testing.T) {
	client := &mockServiceClient{
		getProjects: func(_ context.Context) (*models.GroupsioServiceProjectsResponse, error) {
			return &models.GroupsioServiceProjectsResponse{Projects: []string{"proj-a", "proj-b"}}, nil
		},
	}

	svc := NewGroupsioServiceService(client, passthroughMapper())
	resp, err := svc.GetProjects(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"proj-a", "proj-b"}, resp.Projects)
}

func TestGroupsioServiceService_FindParentService(t *testing.T) {
	var receivedProjectID string
	client := &mockServiceClient{
		findParentService: func(_ context.Context, projectID string) (*models.GroupsioService, error) {
			receivedProjectID = projectID
			return &models.GroupsioService{ID: "parent-svc", ProjectID: projV1}, nil
		},
	}

	svc := NewGroupsioServiceService(client, swappingMapper(v2ToV1, v1ToV2))
	resp, err := svc.FindParentService(context.Background(), projV2)

	require.NoError(t, err)
	assert.Equal(t, projV1, receivedProjectID, "should call ITX with v1 SFID")
	assert.Equal(t, projV2, resp.ProjectID, "response should have v2 UUID")
}

func TestGroupsioServiceService_MapperError_PropagatedOnRequest(t *testing.T) {
	mapErr := errors.New("mapping failed")
	mapper := &mockIDMapper{
		mapProjectV2ToV1: func(_ context.Context, _ string) (string, error) { return "", mapErr },
		mapProjectV1ToV2: func(_ context.Context, id string) (string, error) { return id, nil },
	}
	client := &mockServiceClient{
		createService: func(_ context.Context, _ *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
			t.Fatal("client should not be called when mapper fails")
			return nil, nil
		},
	}

	svc := NewGroupsioServiceService(client, mapper)
	_, err := svc.CreateService(context.Background(), &models.GroupsioServiceRequest{ProjectID: projV2})

	require.Error(t, err)
	assert.True(t, errors.Is(err, mapErr))
}

func TestGroupsioServiceService_MapperError_PropagatedOnResponse(t *testing.T) {
	mapErr := errors.New("reverse mapping failed")
	mapper := &mockIDMapper{
		mapProjectV2ToV1: func(_ context.Context, id string) (string, error) { return id, nil },
		mapProjectV1ToV2: func(_ context.Context, _ string) (string, error) { return "", mapErr },
	}
	client := &mockServiceClient{
		getService: func(_ context.Context, _ string) (*models.GroupsioService, error) {
			return &models.GroupsioService{ID: "svc-1", ProjectID: projV1}, nil
		},
	}

	svc := NewGroupsioServiceService(client, mapper)
	_, err := svc.GetService(context.Background(), "svc-1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, mapErr))
}

func TestGroupsioServiceService_ClientError_Propagated(t *testing.T) {
	clientErr := errors.New("client error")
	client := &mockServiceClient{
		getService: func(_ context.Context, _ string) (*models.GroupsioService, error) {
			return nil, clientErr
		},
	}

	svc := NewGroupsioServiceService(client, passthroughMapper())
	_, err := svc.GetService(context.Background(), "svc-1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, clientErr))
}
