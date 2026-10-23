// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
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

// authorizeAddGroupsioMember enforces field-level restrictions on top of
// Heimdall's writer-or-public gate for add-groupsio-member. Heimdall's
// "public" branch admits any authenticated caller for a public mailing list
// (not just writers), so a caller who is not a verified writer on the list
// is only allowed to add themselves, and only with non-privileged fields.
// This requires knowing which branch actually matched, so it asks fga-sync
// directly (via s.accessChecker) whether the caller holds the writer
// relation, rather than inferring it from whether the payload email happens
// to match the caller's own email.
func (s *mailingListAPI) authorizeAddGroupsioMember(ctx context.Context, p *mailinglist.AddGroupsioMemberPayload) error {
	principal, _ := ctx.Value(constants.PrincipalContextID).(string)
	if principal == "" {
		return errs.NewUnauthorized("missing caller identity")
	}

	object := fmt.Sprintf("%s:%s", constants.ObjectTypeGroupsIOMailingList, p.SubgroupID)
	user := fmt.Sprintf("%s:%s", constants.ObjectTypeUser, principal)
	isWriter, err := s.accessChecker.HasRelation(ctx, user, constants.RelationWriter, object)
	if err != nil {
		slog.ErrorContext(ctx, "access check failed for add-groupsio-member",
			"error", err, "object", object)
		return errs.NewServiceUnavailable("unable to verify writer access", err)
	}
	if isWriter {
		// Verified writer: no field restrictions beyond what the payload
		// validation already enforces.
		return nil
	}

	// Not a writer: Heimdall only let this through via the public branch, so
	// restrict to a plain self-service subscription — the caller's own email,
	// with no privileged fields.
	callerEmail, _ := ctx.Value(constants.EmailContextID).(string)
	payloadEmail := converter.StringVal(p.Email)
	if callerEmail == "" || !strings.EqualFold(callerEmail, payloadEmail) {
		return errs.NewUnauthorized("non-writer callers may only add themselves")
	}

	memberType := converter.StringVal(p.MemberType)
	if memberType != "" && memberType != "direct" {
		return errs.NewUnauthorized("self-service subscription cannot set member_type")
	}
	modStatus := converter.StringVal(p.ModStatus)
	if modStatus != "" && modStatus != constants.ModStatusNone {
		return errs.NewUnauthorized("self-service subscription cannot set mod_status")
	}
	return nil
}

func (s *mailingListAPI) AddGroupsioMember(ctx context.Context, p *mailinglist.AddGroupsioMemberPayload) (*mailinglist.GroupsioMember, error) {
	if err := s.authorizeAddGroupsioMember(ctx, p); err != nil {
		return nil, mapDomainError(err)
	}
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
