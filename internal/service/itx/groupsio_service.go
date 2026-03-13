// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package itx provides ITX proxy service implementations for GroupsIO operations.
package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
)

// GroupsioServiceService handles ITX GroupsIO service operations with ID mapping.
type GroupsioServiceService struct {
	client   domain.GroupsioServiceClient
	idMapper domain.IDMapper
}

// NewGroupsioServiceService creates a new GroupsIO service handler.
func NewGroupsioServiceService(client domain.GroupsioServiceClient, idMapper domain.IDMapper) *GroupsioServiceService {
	return &GroupsioServiceService{
		client:   client,
		idMapper: idMapper,
	}
}

// ListServices lists GroupsIO services, mapping project_uid (v2) -> project_id (v1) before calling ITX.
func (s *GroupsioServiceService) ListServices(ctx context.Context, projectUID string) (*models.GroupsioServiceListResponse, error) {
	v1ProjectID := ""
	if projectUID != "" {
		var err error
		v1ProjectID, err = s.idMapper.MapProjectV2ToV1(ctx, projectUID)
		if err != nil {
			return nil, err
		}
	}

	resp, err := s.client.ListServices(ctx, v1ProjectID)
	if err != nil {
		return nil, err
	}

	// Map project_id (v1) -> project_uid (v2) in responses
	for _, svc := range resp.Items {
		if svc.ProjectID != "" {
			v2UID, mapErr := s.idMapper.MapProjectV1ToV2(ctx, svc.ProjectID)
			if mapErr != nil {
				return nil, mapErr
			}
			svc.ProjectID = v2UID
		}
	}

	return resp, nil
}

// CreateService creates a new GroupsIO service, mapping project_uid (v2) -> project_id (v1).
func (s *GroupsioServiceService) CreateService(ctx context.Context, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
	if req.ProjectID != "" {
		v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, req.ProjectID)
		if err != nil {
			return nil, err
		}
		req.ProjectID = v1ID
	}

	resp, err := s.client.CreateService(ctx, req)
	if err != nil {
		return nil, err
	}

	return s.mapServiceResponse(ctx, resp)
}

// GetService retrieves a GroupsIO service by ID, mapping project_id (v1) -> project_uid (v2) in response.
func (s *GroupsioServiceService) GetService(ctx context.Context, serviceID string) (*models.GroupsioService, error) {
	resp, err := s.client.GetService(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	return s.mapServiceResponse(ctx, resp)
}

// UpdateService updates a GroupsIO service, mapping project_uid (v2) -> project_id (v1).
func (s *GroupsioServiceService) UpdateService(ctx context.Context, serviceID string, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
	if req.ProjectID != "" {
		v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, req.ProjectID)
		if err != nil {
			return nil, err
		}
		req.ProjectID = v1ID
	}

	resp, err := s.client.UpdateService(ctx, serviceID, req)
	if err != nil {
		return nil, err
	}

	return s.mapServiceResponse(ctx, resp)
}

// DeleteService deletes a GroupsIO service.
func (s *GroupsioServiceService) DeleteService(ctx context.Context, serviceID string) error {
	return s.client.DeleteService(ctx, serviceID)
}

// GetProjects returns projects that have GroupsIO services.
func (s *GroupsioServiceService) GetProjects(ctx context.Context) (*models.GroupsioServiceProjectsResponse, error) {
	return s.client.GetProjects(ctx)
}

// FindParentService finds the parent service for a project, mapping project_uid (v2) -> project_id (v1).
func (s *GroupsioServiceService) FindParentService(ctx context.Context, projectUID string) (*models.GroupsioService, error) {
	v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, projectUID)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.FindParentService(ctx, v1ID)
	if err != nil {
		return nil, err
	}

	return s.mapServiceResponse(ctx, resp)
}

// mapServiceResponse maps project_id (v1) -> project_uid (v2) in a service response.
func (s *GroupsioServiceService) mapServiceResponse(ctx context.Context, svc *models.GroupsioService) (*models.GroupsioService, error) {
	if svc == nil {
		return nil, nil
	}
	if svc.ProjectID != "" {
		v2UID, err := s.idMapper.MapProjectV1ToV2(ctx, svc.ProjectID)
		if err != nil {
			return nil, err
		}
		svc.ProjectID = v2UID
	}
	return svc, nil
}
