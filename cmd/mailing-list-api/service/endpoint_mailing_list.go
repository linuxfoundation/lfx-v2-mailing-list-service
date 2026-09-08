// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

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
