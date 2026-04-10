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
		Organization: converter.NonEmptyString(m.Organization),
		JobTitle:     converter.NonEmptyString(m.JobTitle),
		Username:     converter.NonEmptyString(m.Username),
		Role:         converter.NonEmptyString(m.Role),
		VotingStatus: converter.NonEmptyString(m.VotingStatus),
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
	createdAt := ""
	if !ml.CreatedAt.IsZero() {
		createdAt = ml.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if !ml.UpdatedAt.IsZero() {
		updatedAt = ml.UpdatedAt.Format(time.RFC3339)
	}
	return &mailinglist.GroupsioSubgroup{
		ID:             &ml.UID,
		ProjectUID:     converter.NonEmptyString(ml.ProjectUID),
		CommitteeUID:   converter.NonEmptyString(committeeUID),
		ServiceID:      &ml.ServiceUID,
		GroupID:        ml.GroupID,
		Name:           &ml.GroupName,
		Description:    &ml.Description,
		Type:           &ml.Type,
		AudienceAccess: &ml.AudienceAccess,
		CreatedAt:      converter.NonEmptyString(createdAt),
		UpdatedAt:      converter.NonEmptyString(updatedAt),
	}
}

func convertArtifactUser(u *model.ArtifactUser) *mailinglist.GroupsioArtifactUser {
	if u == nil {
		return nil
	}
	return &mailinglist.GroupsioArtifactUser{
		ID:             converter.NonEmptyString(u.ID),
		Username:       converter.NonEmptyString(u.Username),
		Name:           converter.NonEmptyString(u.Name),
		Email:          converter.NonEmptyString(u.Email),
		ProfilePicture: converter.NonEmptyString(u.ProfilePicture),
	}
}

func convertArtifact(a *model.GroupsIOArtifact) *mailinglist.GroupsioArtifact {
	if a == nil {
		return nil
	}
	createdAt := ""
	if !a.CreatedAt.IsZero() {
		createdAt = a.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if !a.UpdatedAt.IsZero() {
		updatedAt = a.UpdatedAt.Format(time.RFC3339)
	}
	var fileUploadedAt *string
	if a.FileUploadedAt != nil && !a.FileUploadedAt.IsZero() {
		s := a.FileUploadedAt.Format(time.RFC3339)
		fileUploadedAt = &s
	}
	var lastPostedAt *string
	if a.LastPostedAt != nil && !a.LastPostedAt.IsZero() {
		s := a.LastPostedAt.Format(time.RFC3339)
		lastPostedAt = &s
	}
	groupID := a.GroupID
	var fileUploaded *bool
	if a.Type == "file" {
		fileUploaded = a.FileUploaded
	}
	return &mailinglist.GroupsioArtifact{
		ArtifactID:          converter.NonEmptyString(a.ArtifactID),
		GroupID:             &groupID,
		ProjectID:           converter.NonEmptyString(a.ProjectUID),
		CommitteeID:         converter.NonEmptyString(a.CommitteeUID),
		Type:                converter.NonEmptyString(a.Type),
		MediaType:           converter.NonEmptyString(a.MediaType),
		Filename:            converter.NonEmptyString(a.Filename),
		LinkURL:             converter.NonEmptyString(a.LinkURL),
		DownloadURL:         converter.NonEmptyString(a.DownloadURL),
		S3Key:               converter.NonEmptyString(a.S3Key),
		FileUploaded:        fileUploaded,
		FileUploadStatus:    converter.NonEmptyString(a.FileUploadStatus),
		FileUploadedAt:      fileUploadedAt,
		MessageIds:          a.MessageIDs,
		LastPostedAt:        lastPostedAt,
		LastPostedMessageID: a.LastPostedMessageID,
		Description:         converter.NonEmptyString(a.Description),
		CreatedBy:           convertArtifactUser(a.CreatedBy),
		LastModifiedBy:      convertArtifactUser(a.LastModifiedBy),
		CreatedAt:           converter.NonEmptyString(createdAt),
		UpdatedAt:           converter.NonEmptyString(updatedAt),
	}
}

func convertService(svc *model.GroupsIOService) *mailinglist.GroupsioService {
	if svc == nil {
		return nil
	}
	createdAt := ""
	if !svc.CreatedAt.IsZero() {
		createdAt = svc.CreatedAt.Format(time.RFC3339)
	}
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
		CreatedAt:  converter.NonEmptyString(createdAt),
		UpdatedAt:  converter.NonEmptyString(updatedAt),
	}
}
