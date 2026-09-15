// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

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
