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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	itxsvc "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"

	"goa.design/goa/v3/security"
)

// mailingListAPI implements the generated mailinglist.Service interface by proxying to ITX.
type mailingListAPI struct {
	auth            port.Authenticator
	serviceService  *itxsvc.GroupsioServiceService
	subgroupService *itxsvc.GroupsioSubgroupService
	memberService   *itxsvc.GroupsioMemberService
}

// NewMailingListAPI returns the mailing list API service implementation.
func NewMailingListAPI(
	auth port.Authenticator,
	serviceService *itxsvc.GroupsioServiceService,
	subgroupService *itxsvc.GroupsioSubgroupService,
	memberService *itxsvc.GroupsioMemberService,
) mailinglist.Service {
	return &mailingListAPI{
		auth:            auth,
		serviceService:  serviceService,
		subgroupService: subgroupService,
		memberService:   memberService,
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
	projectUID := ""
	if p.ProjectUID != nil {
		projectUID = *p.ProjectUID
	}
	resp, err := s.serviceService.ListServices(ctx, projectUID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	items := make([]*mailinglist.GroupsioService, len(resp.Items))
	for i, svc := range resp.Items {
		items[i] = convertService(svc)
	}
	total := resp.Total
	return &mailinglist.GroupsioServiceList{Items: items, Total: &total}, nil
}

func (s *mailingListAPI) CreateGroupsioService(ctx context.Context, p *mailinglist.CreateGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	req := &models.GroupsioServiceRequest{
		ProjectID: strVal(p.ProjectUID),
		Type:      strVal(p.Type),
		GroupID:   int64Val(p.GroupID),
		Domain:    strVal(p.Domain),
		Prefix:    strVal(p.Prefix),
		Status:    strVal(p.Status),
	}
	resp, err := s.serviceService.CreateService(ctx, req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(resp), nil
}

func (s *mailingListAPI) GetGroupsioService(ctx context.Context, p *mailinglist.GetGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	resp, err := s.serviceService.GetService(ctx, p.ServiceID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(resp), nil
}

func (s *mailingListAPI) UpdateGroupsioService(ctx context.Context, p *mailinglist.UpdateGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	req := &models.GroupsioServiceRequest{
		ProjectID: strVal(p.ProjectUID),
		Type:      strVal(p.Type),
		GroupID:   int64Val(p.GroupID),
		Domain:    strVal(p.Domain),
		Prefix:    strVal(p.Prefix),
		Status:    strVal(p.Status),
	}
	resp, err := s.serviceService.UpdateService(ctx, p.ServiceID, req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(resp), nil
}

func (s *mailingListAPI) DeleteGroupsioService(ctx context.Context, p *mailinglist.DeleteGroupsioServicePayload) error {
	return mapDomainError(s.serviceService.DeleteService(ctx, p.ServiceID))
}

func (s *mailingListAPI) GetGroupsioServiceProjects(ctx context.Context, _ *mailinglist.GetGroupsioServiceProjectsPayload) (*mailinglist.GroupsioProjectsResponse, error) {
	resp, err := s.serviceService.GetProjects(ctx)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioProjectsResponse{Projects: resp.Projects}, nil
}

func (s *mailingListAPI) FindParentGroupsioService(ctx context.Context, p *mailinglist.FindParentGroupsioServicePayload) (*mailinglist.GroupsioService, error) {
	resp, err := s.serviceService.FindParentService(ctx, p.ProjectUID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertService(resp), nil
}

// ---- GroupsIO Subgroup endpoints ----

func (s *mailingListAPI) ListGroupsioSubgroups(ctx context.Context, p *mailinglist.ListGroupsioSubgroupsPayload) (*mailinglist.GroupsioSubgroupList, error) {
	projectUID := ""
	if p.ProjectUID != nil {
		projectUID = *p.ProjectUID
	}
	committeeUID := ""
	if p.CommitteeUID != nil {
		committeeUID = *p.CommitteeUID
	}
	resp, err := s.subgroupService.ListSubgroups(ctx, projectUID, committeeUID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	items := make([]*mailinglist.GroupsioSubgroup, len(resp.Items))
	for i, sg := range resp.Items {
		items[i] = convertSubgroup(sg)
	}
	total := resp.Meta.TotalResults
	return &mailinglist.GroupsioSubgroupList{Items: items, Total: &total}, nil
}

func (s *mailingListAPI) CreateGroupsioSubgroup(ctx context.Context, p *mailinglist.CreateGroupsioSubgroupPayload) (*mailinglist.GroupsioSubgroup, error) {
	req := &models.GroupsioSubgroupRequest{
		ProjectID:      strVal(p.ProjectUID),
		CommitteeID:    strVal(p.CommitteeUID),
		GroupID:        int64Val(p.GroupID),
		Name:           strVal(p.Name),
		Description:    strVal(p.Description),
		Type:           strVal(p.Type),
		AudienceAccess: strVal(p.AudienceAccess),
	}
	resp, err := s.subgroupService.CreateSubgroup(ctx, req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertSubgroup(resp), nil
}

func (s *mailingListAPI) GetGroupsioSubgroup(ctx context.Context, p *mailinglist.GetGroupsioSubgroupPayload) (*mailinglist.GroupsioSubgroup, error) {
	resp, err := s.subgroupService.GetSubgroup(ctx, p.SubgroupID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertSubgroup(resp), nil
}

func (s *mailingListAPI) UpdateGroupsioSubgroup(ctx context.Context, p *mailinglist.UpdateGroupsioSubgroupPayload) (*mailinglist.GroupsioSubgroup, error) {
	req := &models.GroupsioSubgroupRequest{
		ProjectID:      strVal(p.ProjectUID),
		CommitteeID:    strVal(p.CommitteeUID),
		GroupID:        int64Val(p.GroupID),
		Name:           strVal(p.Name),
		Description:    strVal(p.Description),
		Type:           strVal(p.Type),
		AudienceAccess: strVal(p.AudienceAccess),
	}
	resp, err := s.subgroupService.UpdateSubgroup(ctx, p.SubgroupID, req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertSubgroup(resp), nil
}

func (s *mailingListAPI) DeleteGroupsioSubgroup(ctx context.Context, p *mailinglist.DeleteGroupsioSubgroupPayload) error {
	return mapDomainError(s.subgroupService.DeleteSubgroup(ctx, p.SubgroupID))
}

func (s *mailingListAPI) GetGroupsioSubgroupCount(ctx context.Context, p *mailinglist.GetGroupsioSubgroupCountPayload) (*mailinglist.GroupsioCount, error) {
	resp, err := s.subgroupService.GetSubgroupCount(ctx, p.ProjectUID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioCount{Count: resp.Count}, nil
}

func (s *mailingListAPI) GetGroupsioSubgroupMemberCount(ctx context.Context, p *mailinglist.GetGroupsioSubgroupMemberCountPayload) (*mailinglist.GroupsioCount, error) {
	resp, err := s.subgroupService.GetMemberCount(ctx, p.SubgroupID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioCount{Count: resp.Count}, nil
}

// ---- GroupsIO Member endpoints ----

func (s *mailingListAPI) ListGroupsioMembers(ctx context.Context, p *mailinglist.ListGroupsioMembersPayload) (*mailinglist.GroupsioMemberList, error) {
	resp, err := s.memberService.ListMembers(ctx, p.SubgroupID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	items := make([]*mailinglist.GroupsioMember, len(resp.Items))
	for i, m := range resp.Items {
		items[i] = convertMember(m)
	}
	total := resp.Total
	return &mailinglist.GroupsioMemberList{Items: items, Total: &total}, nil
}

func (s *mailingListAPI) AddGroupsioMember(ctx context.Context, p *mailinglist.AddGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	req := &models.GroupsioMemberRequest{
		Email:        strVal(p.Email),
		Name:         strVal(p.Name),
		ModStatus:    strVal(p.ModStatus),
		DeliveryMode: strVal(p.DeliveryMode),
	}
	resp, err := s.memberService.AddMember(ctx, p.SubgroupID, req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMember(resp), nil
}

func (s *mailingListAPI) GetGroupsioMember(ctx context.Context, p *mailinglist.GetGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	resp, err := s.memberService.GetMember(ctx, p.SubgroupID, p.MemberID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMember(resp), nil
}

func (s *mailingListAPI) UpdateGroupsioMember(ctx context.Context, p *mailinglist.UpdateGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	req := &models.GroupsioMemberRequest{
		Email:        strVal(p.Email),
		Name:         strVal(p.Name),
		ModStatus:    strVal(p.ModStatus),
		DeliveryMode: strVal(p.DeliveryMode),
	}
	resp, err := s.memberService.UpdateMember(ctx, p.SubgroupID, p.MemberID, req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertMember(resp), nil
}

func (s *mailingListAPI) DeleteGroupsioMember(ctx context.Context, p *mailinglist.DeleteGroupsioMemberPayload) error {
	return mapDomainError(s.memberService.DeleteMember(ctx, p.SubgroupID, p.MemberID))
}

func (s *mailingListAPI) InviteGroupsioMembers(ctx context.Context, p *mailinglist.InviteGroupsioMembersPayload) error {
	req := &models.GroupsioInviteMembersRequest{Emails: p.Emails}
	return mapDomainError(s.memberService.InviteMembers(ctx, p.SubgroupID, req))
}

func (s *mailingListAPI) CheckGroupsioSubscriber(ctx context.Context, p *mailinglist.CheckGroupsioSubscriberPayload) (*mailinglist.GroupsioCheckSubscriberResponse, error) {
	req := &models.GroupsioCheckSubscriberRequest{
		Email:      p.Email,
		SubgroupID: p.SubgroupID,
	}
	resp, err := s.memberService.CheckSubscriber(ctx, req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioCheckSubscriberResponse{Subscribed: resp.Subscribed}, nil
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

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func int64Val(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func convertService(svc *models.GroupsioService) *mailinglist.GroupsioService {
	if svc == nil {
		return nil
	}
	return &mailinglist.GroupsioService{
		ID:         &svc.ID,
		ProjectUID: &svc.ProjectID,
		Type:       &svc.Type,
		GroupID:    &svc.GroupID,
		Domain:     &svc.Domain,
		Prefix:     &svc.Prefix,
		Status:     &svc.Status,
		CreatedAt:  &svc.CreatedAt,
		UpdatedAt:  &svc.UpdatedAt,
	}
}

func convertSubgroup(sg *models.GroupsioSubgroup) *mailinglist.GroupsioSubgroup {
	if sg == nil {
		return nil
	}
	return &mailinglist.GroupsioSubgroup{
		ID:             &sg.ID,
		ProjectUID:     &sg.ProjectID,
		CommitteeUID:   &sg.CommitteeID,
		GroupID:        &sg.GroupID,
		Name:           &sg.Name,
		Description:    &sg.Description,
		Type:           &sg.Type,
		AudienceAccess: &sg.AudienceAccess,
		CreatedAt:      &sg.CreatedAt,
		UpdatedAt:      &sg.UpdatedAt,
	}
}

func convertMember(m *models.GroupsioMember) *mailinglist.GroupsioMember {
	if m == nil {
		return nil
	}
	return &mailinglist.GroupsioMember{
		ID:           &m.ID,
		SubgroupID:   &m.SubgroupID,
		Email:        &m.Email,
		Name:         &m.Name,
		FirstName:    &m.FirstName,
		LastName:     &m.LastName,
		ModStatus:    &m.ModStatus,
		DeliveryMode: &m.DeliveryMode,
		Status:       &m.Status,
		CreatedAt:    &m.CreatedAt,
		UpdatedAt:    &m.UpdatedAt,
	}
}
