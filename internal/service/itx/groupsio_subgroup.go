// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
)

// GroupsioSubgroupService handles ITX GroupsIO subgroup operations with ID mapping.
type GroupsioSubgroupService struct {
	client   domain.GroupsioSubgroupClient
	idMapper domain.IDMapper
}

// NewGroupsioSubgroupService creates a new GroupsIO subgroup handler.
func NewGroupsioSubgroupService(client domain.GroupsioSubgroupClient, idMapper domain.IDMapper) *GroupsioSubgroupService {
	return &GroupsioSubgroupService{
		client:   client,
		idMapper: idMapper,
	}
}

// ListSubgroups lists subgroups, mapping v2 UIDs -> v1 SFIDs before calling ITX.
func (s *GroupsioSubgroupService) ListSubgroups(ctx context.Context, projectUID, committeeUID string) (*models.GroupsioSubgroupListResponse, error) {
	v1ProjectID := ""
	if projectUID != "" {
		var err error
		v1ProjectID, err = s.idMapper.MapProjectV2ToV1(ctx, projectUID)
		if err != nil {
			return nil, err
		}
	}

	v1CommitteeID := ""
	if committeeUID != "" {
		var err error
		v1CommitteeID, err = s.idMapper.MapCommitteeV2ToV1(ctx, committeeUID)
		if err != nil {
			return nil, err
		}
	}

	resp, err := s.client.ListSubgroups(ctx, v1ProjectID, v1CommitteeID)
	if err != nil {
		return nil, err
	}

	// Map v1 SFIDs -> v2 UIDs in responses
	for _, sg := range resp.Items {
		if mapErr := s.mapSubgroupResponseIDs(ctx, sg); mapErr != nil {
			return nil, mapErr
		}
	}

	return resp, nil
}

// CreateSubgroup creates a new subgroup, mapping v2 UIDs -> v1 SFIDs.
func (s *GroupsioSubgroupService) CreateSubgroup(ctx context.Context, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
	if req.ProjectID != "" {
		v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, req.ProjectID)
		if err != nil {
			return nil, err
		}
		req.ProjectID = v1ID
	}

	if req.CommitteeID != "" {
		v1ID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.CommitteeID)
		if err != nil {
			return nil, err
		}
		req.CommitteeID = v1ID
	}

	resp, err := s.client.CreateSubgroup(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.mapSubgroupResponseIDs(ctx, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetSubgroup retrieves a subgroup by ID, mapping v1 SFIDs -> v2 UIDs in response.
func (s *GroupsioSubgroupService) GetSubgroup(ctx context.Context, subgroupID string) (*models.GroupsioSubgroup, error) {
	resp, err := s.client.GetSubgroup(ctx, subgroupID)
	if err != nil {
		return nil, err
	}

	if err := s.mapSubgroupResponseIDs(ctx, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateSubgroup updates a subgroup, mapping v2 UIDs -> v1 SFIDs.
func (s *GroupsioSubgroupService) UpdateSubgroup(ctx context.Context, subgroupID string, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
	if req.ProjectID != "" {
		v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, req.ProjectID)
		if err != nil {
			return nil, err
		}
		req.ProjectID = v1ID
	}

	if req.CommitteeID != "" {
		v1ID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.CommitteeID)
		if err != nil {
			return nil, err
		}
		req.CommitteeID = v1ID
	}

	resp, err := s.client.UpdateSubgroup(ctx, subgroupID, req)
	if err != nil {
		return nil, err
	}

	if err := s.mapSubgroupResponseIDs(ctx, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteSubgroup deletes a subgroup.
func (s *GroupsioSubgroupService) DeleteSubgroup(ctx context.Context, subgroupID string) error {
	return s.client.DeleteSubgroup(ctx, subgroupID)
}

// GetSubgroupCount returns the count of subgroups for a project.
func (s *GroupsioSubgroupService) GetSubgroupCount(ctx context.Context, projectUID string) (*models.GroupsioSubgroupCountResponse, error) {
	v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, projectUID)
	if err != nil {
		return nil, err
	}

	return s.client.GetSubgroupCount(ctx, v1ID)
}

// GetMemberCount returns the count of members in a subgroup.
func (s *GroupsioSubgroupService) GetMemberCount(ctx context.Context, subgroupID string) (*models.GroupsioMemberCountResponse, error) {
	return s.client.GetMemberCount(ctx, subgroupID)
}

// mapSubgroupResponseIDs maps v1 SFIDs -> v2 UIDs in a subgroup response.
func (s *GroupsioSubgroupService) mapSubgroupResponseIDs(ctx context.Context, sg *models.GroupsioSubgroup) error {
	if sg == nil {
		return nil
	}
	if sg.ProjectID != "" {
		v2UID, err := s.idMapper.MapProjectV1ToV2(ctx, sg.ProjectID)
		if err != nil {
			return err
		}
		sg.ProjectID = v2UID
	}
	if sg.CommitteeID != "" {
		v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, sg.CommitteeID)
		if err != nil {
			return err
		}
		sg.CommitteeID = v2UID
	}
	return nil
}
