// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service implements the mailing list API service, proxying to the ITX GroupsIO API.
package service

import (
	"context"
	"errors"
	"log/slog"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"

	"goa.design/goa/v3/security"
)

// mailingListAPI implements the generated mailinglist.Service interface.
type mailingListAPI struct {
	auth              port.Authenticator
	serviceReader     port.GroupsIOServiceReader
	serviceWriter     port.GroupsIOServiceWriter
	mailingListReader port.GroupsIOMailingListReader
	mailingListWriter port.GroupsIOMailingListWriter
	memberReader      port.GroupsIOMailingListMemberReader
	memberWriter      port.GroupsIOMailingListMemberWriter
}

// NewMailingListAPI returns the mailing list API service implementation.
func NewMailingListAPI(
	auth port.Authenticator,
	serviceReader port.GroupsIOServiceReader,
	serviceWriter port.GroupsIOServiceWriter,
	mailingListReader port.GroupsIOMailingListReader,
	mailingListWriter port.GroupsIOMailingListWriter,
	memberReader port.GroupsIOMailingListMemberReader,
	memberWriter port.GroupsIOMailingListMemberWriter,
) mailinglist.Service {
	return &mailingListAPI{
		auth:              auth,
		serviceReader:     serviceReader,
		serviceWriter:     serviceWriter,
		mailingListReader: mailingListReader,
		mailingListWriter: mailingListWriter,
		memberReader:      memberReader,
		memberWriter:      memberWriter,
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

func (s *mailingListAPI) ListGroupsioMailingLists(ctx context.Context, p *mailinglist.ListGroupsioMailingListsPayload) (*mailinglist.GroupsioSubgroupList, error) {
	items, total, err := s.mailingListReader.ListMailingLists(ctx, converter.StringVal(p.ProjectUID), converter.StringVal(p.CommitteeUID))
	if err != nil {
		return nil, mapDomainError(err)
	}
	result := make([]*mailinglist.GroupsioSubgroup, len(items))
	for i, ml := range items {
		result[i] = convertMailingList(ml)
	}
	return &mailinglist.GroupsioSubgroupList{Items: result, Total: &total}, nil
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

func (s *mailingListAPI) GetGroupsioMailingList(ctx context.Context, p *mailinglist.GetGroupsioMailingListPayload) (*mailinglist.GroupsioSubgroup, error) {
	ml, err := s.mailingListReader.GetMailingList(ctx, p.SubgroupID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMailingList(ml), nil
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

func (s *mailingListAPI) GetGroupsioMailingListCount(ctx context.Context, p *mailinglist.GetGroupsioMailingListCountPayload) (*mailinglist.GroupsioCount, error) {
	count, err := s.mailingListReader.GetMailingListCount(ctx, p.ProjectUID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioCount{Count: count}, nil
}

func (s *mailingListAPI) GetGroupsioMailingListMemberCount(ctx context.Context, p *mailinglist.GetGroupsioMailingListMemberCountPayload) (*mailinglist.GroupsioCount, error) {
	count, err := s.mailingListReader.GetMailingListMemberCount(ctx, p.SubgroupID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioCount{Count: count}, nil
}

// ---- GroupsIO Member endpoints ----

func (s *mailingListAPI) ListGroupsioMembers(ctx context.Context, p *mailinglist.ListGroupsioMembersPayload) (*mailinglist.GroupsioMemberList, error) {
	items, total, err := s.memberReader.ListMembers(ctx, p.SubgroupID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	result := make([]*mailinglist.GroupsioMember, len(items))
	for i, m := range items {
		result[i] = convertMember(m)
	}
	return &mailinglist.GroupsioMemberList{Items: result, Total: &total}, nil
}

func (s *mailingListAPI) AddGroupsioMember(ctx context.Context, p *mailinglist.AddGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	member := &model.GrpsIOMember{
		Email:          converter.StringVal(p.Email),
		GroupsFullName: converter.StringVal(p.Name),
		UserID:         converter.StringVal(p.UserID),
		DeliveryMode:   converter.StringVal(p.DeliveryMode),
		MemberType:     converter.StringVal(p.MemberType),
		ModStatus:      converter.StringVal(p.ModStatus),
		Organization:   converter.StringVal(p.Organization),
		JobTitle:       converter.StringVal(p.JobTitle),
	}
	resp, err := s.memberWriter.AddMember(ctx, p.SubgroupID, member)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMember(resp), nil
}

func (s *mailingListAPI) GetGroupsioMember(ctx context.Context, p *mailinglist.GetGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	m, err := s.memberReader.GetMember(ctx, p.SubgroupID, p.MemberID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMember(m), nil
}

func (s *mailingListAPI) UpdateGroupsioMember(ctx context.Context, p *mailinglist.UpdateGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	member := &model.GrpsIOMember{
		Email:          converter.StringVal(p.Email),
		GroupsFullName: converter.StringVal(p.Name),
		UserID:         converter.StringVal(p.UserID),
		DeliveryMode:   converter.StringVal(p.DeliveryMode),
		MemberType:     converter.StringVal(p.MemberType),
		ModStatus:      converter.StringVal(p.ModStatus),
		Organization:   converter.StringVal(p.Organization),
		JobTitle:       converter.StringVal(p.JobTitle),
	}
	resp, err := s.memberWriter.UpdateMember(ctx, p.SubgroupID, p.MemberID, member)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMember(resp), nil
}

func (s *mailingListAPI) DeleteGroupsioMember(ctx context.Context, p *mailinglist.DeleteGroupsioMemberPayload) error {
	return mapDomainError(s.memberWriter.DeleteMember(ctx, p.SubgroupID, p.MemberID))
}

func (s *mailingListAPI) InviteGroupsioMembers(ctx context.Context, p *mailinglist.InviteGroupsioMembersPayload) error {
	return mapDomainError(s.memberWriter.InviteMembers(ctx, p.SubgroupID, p.Emails))
}

func (s *mailingListAPI) CheckGroupsioSubscriber(ctx context.Context, p *mailinglist.CheckGroupsioSubscriberPayload) (*mailinglist.GroupsioCheckSubscriberResponse, error) {
	subscribed, err := s.memberReader.CheckSubscriber(ctx, p.SubgroupID, p.Email)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioCheckSubscriberResponse{Subscribed: subscribed}, nil
}

// ---- Helpers ----

func mapDomainError(err error) error {
	if err == nil {
		return nil
	}
	var notFound errs.NotFound
	if errors.As(err, &notFound) {
		return &mailinglist.NotFoundError{Message: notFound.Error()}
	}
	var validation errs.Validation
	if errors.As(err, &validation) {
		return &mailinglist.BadRequestError{Message: validation.Error()}
	}
	var conflict errs.Conflict
	if errors.As(err, &conflict) {
		return &mailinglist.ConflictError{Message: conflict.Error()}
	}
	var unavailable errs.ServiceUnavailable
	if errors.As(err, &unavailable) {
		return &mailinglist.ServiceUnavailableError{Message: unavailable.Error()}
	}
	return &mailinglist.InternalServerError{Message: err.Error()}
}
