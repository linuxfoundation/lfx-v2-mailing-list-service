// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service implements the mailing list API service, proxying to the ITX GroupsIO API.
package service

import (
	"context"
	"errors"
	"log/slog"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"

	"goa.design/goa/v3/security"
)

// mailingListAPI implements the generated mailinglist.Service interface.
type mailingListAPI struct {
	auth              port.Authenticator
	serviceReader     port.GroupsIOServiceReader
	serviceWriter     port.GroupsIOServiceWriter
	mailingListWriter port.GroupsIOMailingListWriter
}

// NewMailingListAPI returns the mailing list API service implementation.
func NewMailingListAPI(
	auth port.Authenticator,
	serviceReader port.GroupsIOServiceReader,
	serviceWriter port.GroupsIOServiceWriter,
	mailingListWriter port.GroupsIOMailingListWriter,
) mailinglist.Service {
	return &mailingListAPI{
		auth:              auth,
		serviceReader:     serviceReader,
		serviceWriter:     serviceWriter,
		mailingListWriter: mailingListWriter,
	}
}

// JWTAuth implements the authorization logic for the JWT security scheme.
func (s *mailingListAPI) JWTAuth(ctx context.Context, token string, _ *security.JWTScheme) (context.Context, error) {
	principal, err := s.auth.ParsePrincipal(ctx, token, slog.Default())
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}

// Livez implements the liveness probe endpoint.
func (s *mailingListAPI) Livez(_ context.Context) ([]byte, error) {
	return []byte("OK"), nil
}

// Readyz implements the readiness probe endpoint.
func (s *mailingListAPI) Readyz(_ context.Context) ([]byte, error) {
	return []byte("OK"), nil
}

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

// ---- GroupsIO Mailing List endpoints ----

func (s *mailingListAPI) ListGroupsioMailingLists(_ context.Context, _ *mailinglist.ListGroupsioMailingListsPayload) (*mailinglist.GroupsioSubgroupList, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) CreateGroupsioMailingList(ctx context.Context, p *mailinglist.CreateGroupsioMailingListPayload) (*mailinglist.GroupsioSubgroup, error) {
	ml := &model.GroupsIOMailingList{
		ProjectUID:     converter.StringVal(p.ProjectUID),
		ServiceUID:     converter.StringVal(p.ServiceID),
		GroupName:      converter.StringVal(p.Name),
		Description:    converter.StringVal(p.Description),
		Type:           converter.StringVal(p.Type),
		AudienceAccess: converter.StringVal(p.AudienceAccess),
	}
	if committeeUID := converter.StringVal(p.CommitteeUID); committeeUID != "" {
		ml.Committees = []model.Committee{{UID: committeeUID}}
	}
	resp, err := s.mailingListWriter.CreateMailingList(ctx, ml)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMailingList(resp), nil
}

func (s *mailingListAPI) GetGroupsioMailingList(_ context.Context, _ *mailinglist.GetGroupsioMailingListPayload) (*mailinglist.GroupsioSubgroup, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) UpdateGroupsioMailingList(ctx context.Context, p *mailinglist.UpdateGroupsioMailingListPayload) (*mailinglist.GroupsioSubgroup, error) {
	ml := &model.GroupsIOMailingList{
		ProjectUID:     converter.StringVal(p.ProjectUID),
		ServiceUID:     converter.StringVal(p.ServiceID),
		GroupName:      converter.StringVal(p.Name),
		Description:    converter.StringVal(p.Description),
		Type:           converter.StringVal(p.Type),
		AudienceAccess: converter.StringVal(p.AudienceAccess),
	}
	if committeeUID := converter.StringVal(p.CommitteeUID); committeeUID != "" {
		ml.Committees = []model.Committee{{UID: committeeUID}}
	}
	resp, err := s.mailingListWriter.UpdateMailingList(ctx, p.SubgroupID, ml)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMailingList(resp), nil
}

func (s *mailingListAPI) DeleteGroupsioMailingList(ctx context.Context, p *mailinglist.DeleteGroupsioMailingListPayload) error {
	return mapDomainError(s.mailingListWriter.DeleteMailingList(ctx, p.SubgroupID))
}

func (s *mailingListAPI) GetGroupsioMailingListCount(_ context.Context, _ *mailinglist.GetGroupsioMailingListCountPayload) (*mailinglist.GroupsioCount, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) GetGroupsioMailingListMemberCount(_ context.Context, _ *mailinglist.GetGroupsioMailingListMemberCountPayload) (*mailinglist.GroupsioCount, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

// ---- GroupsIO Member endpoints ----

func (s *mailingListAPI) ListGroupsioMembers(_ context.Context, _ *mailinglist.ListGroupsioMembersPayload) (*mailinglist.GroupsioMemberList, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) AddGroupsioMember(_ context.Context, _ *mailinglist.AddGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) GetGroupsioMember(_ context.Context, _ *mailinglist.GetGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) UpdateGroupsioMember(_ context.Context, _ *mailinglist.UpdateGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) DeleteGroupsioMember(_ context.Context, _ *mailinglist.DeleteGroupsioMemberPayload) error {
	return mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) InviteGroupsioMembers(_ context.Context, _ *mailinglist.InviteGroupsioMembersPayload) error {
	return mapDomainError(domain.NewInternalError("not implemented"))
}

func (s *mailingListAPI) CheckGroupsioSubscriber(_ context.Context, _ *mailinglist.CheckGroupsioSubscriberPayload) (*mailinglist.GroupsioCheckSubscriberResponse, error) {
	return nil, mapDomainError(domain.NewInternalError("not implemented"))
}

// ---- Helpers ----

func mapDomainError(err error) error {
	if err == nil {
		return nil
	}
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) {
		return &mailinglist.InternalServerError{Message: err.Error()}
	}
	switch domErr.Type {
	case domain.ErrorTypeNotFound:
		return &mailinglist.NotFoundError{Message: domErr.Message}
	case domain.ErrorTypeValidation:
		return &mailinglist.BadRequestError{Message: domErr.Message}
	case domain.ErrorTypeConflict:
		return &mailinglist.ConflictError{Message: domErr.Message}
	case domain.ErrorTypeUnavailable:
		return &mailinglist.ServiceUnavailableError{Message: domErr.Message}
	default:
		return &mailinglist.InternalServerError{Message: domErr.Message}
	}
}
