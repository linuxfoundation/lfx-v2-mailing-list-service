// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

// convertSyncDiff converts a domain sync diff into its Goa result type.
func convertSyncDiff(d *model.SyncDiff) *mailinglist.GroupsioSyncDiff {
	adds := make([]*mailinglist.GroupsioSyncDiffAdd, len(d.Adds))
	for i, a := range d.Adds {
		adds[i] = &mailinglist.GroupsioSyncDiffAdd{
			Email:              a.Email,
			CommitteeMemberUID: a.CommitteeMemberUID,
			Name:               converter.NonEmptyString(a.Name),
		}
	}
	removes := make([]*mailinglist.GroupsioSyncDiffRemove, len(d.Removes))
	for i, r := range d.Removes {
		removes[i] = &mailinglist.GroupsioSyncDiffRemove{Email: r.Email, MailingListMemberID: r.MailingListMemberID}
	}
	return &mailinglist.GroupsioSyncDiff{
		MailingListID: d.MailingListID,
		CommitteeUID:  d.CommitteeUID,
		SyncMode:      d.SyncMode,
		Adds:          adds,
		Removes:       removes,
	}
}

// convertSyncApplyResult converts a domain sync-apply result into its Goa result type.
func convertSyncApplyResult(r *model.SyncApplyResult) *mailinglist.GroupsioSyncApplyResult {
	added := make([]*mailinglist.GroupsioSyncApplyAddResult, len(r.Added))
	for i, a := range r.Added {
		added[i] = &mailinglist.GroupsioSyncApplyAddResult{
			Email:              a.Email,
			CommitteeMemberUID: a.CommitteeMemberUID,
			Outcome:            string(a.Outcome),
			NameHint:           converter.NonEmptyString(a.NameHint),
			Error:              converter.NonEmptyString(a.Error),
		}
	}
	removed := make([]*mailinglist.GroupsioSyncApplyRemoveResult, len(r.Removed))
	for i, rm := range r.Removed {
		removed[i] = &mailinglist.GroupsioSyncApplyRemoveResult{
			Email:               rm.Email,
			MailingListMemberID: rm.MailingListMemberID,
			Outcome:             string(rm.Outcome),
			Error:               converter.NonEmptyString(rm.Error),
		}
	}
	return &mailinglist.GroupsioSyncApplyResult{
		MailingListID: r.MailingListID,
		CommitteeUID:  r.CommitteeUID,
		Added:         added,
		Removed:       removed,
	}
}

// convertSyncApplyAddPayload converts Goa sync-apply add payload entries into domain adds.
func convertSyncApplyAddPayload(adds []*mailinglist.GroupsioSyncDiffAdd) []model.SyncDiffAdd {
	result := make([]model.SyncDiffAdd, len(adds))
	for i, a := range adds {
		result[i] = model.SyncDiffAdd{
			Email:              a.Email,
			CommitteeMemberUID: a.CommitteeMemberUID,
			Name:               converter.StringVal(a.Name),
		}
	}
	return result
}

// convertSyncApplyRemovePayload converts Goa sync-apply remove payload entries into domain removes.
func convertSyncApplyRemovePayload(removes []*mailinglist.GroupsioSyncDiffRemove) []model.SyncDiffRemove {
	result := make([]model.SyncDiffRemove, len(removes))
	for i, r := range removes {
		result[i] = model.SyncDiffRemove{
			Email:               r.Email,
			MailingListMemberID: r.MailingListMemberID,
		}
	}
	return result
}

// formatTime returns t formatted as RFC3339, or "" if t is zero.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// formatTimePtr returns a pointer to t formatted as RFC3339, or nil if t is nil or zero.
func formatTimePtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func convertMember(m *model.GrpsIOMember) *mailinglist.GroupsioMember {
	if m == nil {
		return nil
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
		CreatedAt:    converter.NonEmptyString(formatTime(m.CreatedAt)),
		UpdatedAt:    converter.NonEmptyString(formatTime(m.UpdatedAt)),
	}
}

func convertMailingList(ml *model.GroupsIOMailingList) *mailinglist.GroupsioSubgroup {
	if ml == nil {
		return nil
	}
	committeeUID := ""
	syncMode := ""
	if len(ml.Committees) > 0 {
		committeeUID = ml.Committees[0].UID
		syncMode = ml.Committees[0].SyncMode
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
		SyncMode:       converter.NonEmptyString(syncMode),
		CreatedAt:      converter.NonEmptyString(formatTime(ml.CreatedAt)),
		UpdatedAt:      converter.NonEmptyString(formatTime(ml.UpdatedAt)),
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
		FileUploadedAt:      formatTimePtr(a.FileUploadedAt),
		MessageIds:          a.MessageIDs,
		LastPostedAt:        formatTimePtr(a.LastPostedAt),
		LastPostedMessageID: a.LastPostedMessageID,
		Description:         converter.NonEmptyString(a.Description),
		CreatedBy:           convertArtifactUser(a.CreatedBy),
		LastModifiedBy:      convertArtifactUser(a.LastModifiedBy),
		CreatedAt:           converter.NonEmptyString(formatTime(a.CreatedAt)),
		UpdatedAt:           converter.NonEmptyString(formatTime(a.UpdatedAt)),
	}
}

func convertService(svc *model.GroupsIOService) *mailinglist.GroupsioService {
	if svc == nil {
		return nil
	}
	return &mailinglist.GroupsioService{
		ID:         &svc.UID,
		ProjectUID: &svc.ProjectUID,
		Type:       &svc.Type,
		GroupID:    svc.GroupID,
		Domain:     &svc.Domain,
		Prefix:     &svc.Prefix,
		Status:     &svc.Status,
		CreatedAt:  converter.NonEmptyString(formatTime(svc.CreatedAt)),
		UpdatedAt:  converter.NonEmptyString(formatTime(svc.UpdatedAt)),
	}
}
