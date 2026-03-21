// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

func convertMember(m *model.GrpsIOMember) *mailinglist.GroupsioMember {
	if m == nil {
		return nil
	}
	createdAt := ""
	if !m.CreatedAt.IsZero() {
		createdAt = m.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if !m.UpdatedAt.IsZero() {
		updatedAt = m.UpdatedAt.Format(time.RFC3339)
	}
	return &mailinglist.GroupsioMember{
		ID:           converter.NonEmptyString(m.UID),
		Email:        converter.NonEmptyString(m.Email),
		Name:         converter.NonEmptyString(m.GroupsFullName),
		MemberType:   converter.NonEmptyString(m.MemberType),
		DeliveryMode: converter.NonEmptyString(m.DeliveryMode),
		ModStatus:    converter.NonEmptyString(m.ModStatus),
		Status:       converter.NonEmptyString(m.Status),
		UserID:       converter.NonEmptyString(m.UserID),
		Organization: converter.NonEmptyString(m.Organization),
		JobTitle:     converter.NonEmptyString(m.JobTitle),
		CreatedAt:    converter.NonEmptyString(createdAt),
		UpdatedAt:    converter.NonEmptyString(updatedAt),
	}
}

func convertMailingList(ml *model.GroupsIOMailingList) *mailinglist.GroupsioSubgroup {
	if ml == nil {
		return nil
	}
	committeeUID := ""
	if len(ml.Committees) > 0 {
		committeeUID = ml.Committees[0].UID
	}
	createdAt := ml.CreatedAt.Format(time.RFC3339)
	updatedAt := ""
	if !ml.UpdatedAt.IsZero() {
		updatedAt = ml.UpdatedAt.Format(time.RFC3339)
	}
	return &mailinglist.GroupsioSubgroup{
		ID:             &ml.UID,
		ProjectUID:     &ml.ProjectUID,
		CommitteeUID:   converter.NonEmptyString(committeeUID),
		ServiceID:      &ml.ServiceUID,
		GroupID:        ml.GroupID,
		Name:           &ml.GroupName,
		Description:    &ml.Description,
		Type:           &ml.Type,
		AudienceAccess: &ml.AudienceAccess,
		CreatedAt:      &createdAt,
		UpdatedAt:      converter.NonEmptyString(updatedAt),
	}
}

func convertService(svc *model.GroupsIOService) *mailinglist.GroupsioService {
	if svc == nil {
		return nil
	}
	createdAt := svc.CreatedAt.Format(time.RFC3339)
	updatedAt := ""
	if !svc.UpdatedAt.IsZero() {
		updatedAt = svc.UpdatedAt.Format(time.RFC3339)
	}
	return &mailinglist.GroupsioService{
		ID:         &svc.UID,
		ProjectUID: &svc.ProjectUID,
		Type:       &svc.Type,
		GroupID:    svc.GroupID,
		Domain:     &svc.Domain,
		Prefix:     &svc.Prefix,
		Status:     &svc.Status,
		CreatedAt:  &createdAt,
		UpdatedAt:  converter.NonEmptyString(updatedAt),
	}
}
