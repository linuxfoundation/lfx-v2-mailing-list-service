// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

// ---- GroupsIO Service endpoints ----

func (s *mailingListAPI) ListGroupsioServices(ctx context.Context, p *mailinglist.ListGroupsioServicesPayload) (*mailinglist.GroupsioServiceList, error) {
	svcs, total, err := s.serviceReader.ListServices(ctx, converter.StringVal(p.ProjectUID))
	if err != nil {
		return nil, mapDomainError(err)
	}
	items := make([]*mailinglist.GroupsioService, len(svcs))
	for i, svc := range svcs {
		items[i] = convertService(svc)
	}
	return &mailinglist.GroupsioServiceList{Items: items, Total: &total}, nil
}

func (s *mailingListAPI) CreateGroupsioService(ctx context.Context, p *mailinglist.CreateGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	svc := &model.GroupsIOService{
		ProjectUID: converter.StringVal(p.ProjectUID),
		Type:       converter.StringVal(p.Type),
		GroupID:    p.GroupID,
		Domain:     converter.StringVal(p.Domain),
		Prefix:     converter.StringVal(p.Prefix),
		Status:     converter.StringVal(p.Status),
	}
	resp, err := s.serviceWriter.CreateService(ctx, svc)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(resp), nil
}

func (s *mailingListAPI) GetGroupsioService(ctx context.Context, p *mailinglist.GetGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	svc, err := s.serviceReader.GetService(ctx, p.ServiceID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(svc), nil
}

func (s *mailingListAPI) UpdateGroupsioService(ctx context.Context, p *mailinglist.UpdateGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	svc := &model.GroupsIOService{
		ProjectUID: converter.StringVal(p.ProjectUID),
		Type:       converter.StringVal(p.Type),
		GroupID:    p.GroupID,
		Domain:     converter.StringVal(p.Domain),
		Prefix:     converter.StringVal(p.Prefix),
		Status:     converter.StringVal(p.Status),
	}
	resp, err := s.serviceWriter.UpdateService(ctx, p.ServiceID, svc)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(resp), nil
}

func (s *mailingListAPI) DeleteGroupsioService(ctx context.Context, p *mailinglist.DeleteGroupsioServicePayload) error {
	return mapDomainError(s.serviceWriter.DeleteService(ctx, p.ServiceID))
}

func (s *mailingListAPI) GetGroupsioServiceProjects(ctx context.Context, _ *mailinglist.GetGroupsioServiceProjectsPayload) (*mailinglist.GroupsioProjectsResponse, error) {
	projects, err := s.serviceReader.GetProjects(ctx)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioProjectsResponse{Projects: projects}, nil
}

func (s *mailingListAPI) FindParentGroupsioService(ctx context.Context, p *mailinglist.FindParentGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	svc, err := s.serviceReader.FindParentService(ctx, p.ProjectUID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(svc), nil
}
