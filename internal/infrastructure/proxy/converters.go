// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"strconv"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

// ---- wire ↔ domain translation helpers ----

func fromWireService(w *serviceWire) *model.GroupsIOService {
	if w == nil {
		return nil
	}
	createdAt, _ := converter.ParseRFC3339(w.CreatedAt)
	updatedAt, _ := converter.ParseRFC3339(w.UpdatedAt)
	return &model.GroupsIOService{
		UID:        w.ID,
		ProjectUID: w.ProjectID,
		Type:       w.Type,
		GroupID:    converter.NonZeroInt64(w.GroupID),
		Domain:     w.Domain,
		Prefix:     w.Prefix,
		Status:     w.Status,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

func toWireServiceRequest(svc *model.GroupsIOService) *serviceRequestWire {
	return &serviceRequestWire{
		ProjectID: svc.ProjectUID,
		Type:      svc.Type,
		GroupID:   converter.Int64Val(svc.GroupID),
		Domain:    svc.Domain,
		Prefix:    svc.Prefix,
		Status:    svc.Status,
	}
}

func fromWireSubgroup(w *subgroupWire) *model.GroupsIOMailingList {
	if w == nil {
		return nil
	}
	// ITX identifies mailing lists by their numeric group_id, not a UUID.
	uid := strconv.FormatInt(w.GroupID, 10)
	createdAt, _ := converter.ParseRFC3339(w.CreatedAt)
	updatedAt, _ := converter.ParseRFC3339(w.UpdatedAt)
	ml := &model.GroupsIOMailingList{
		UID:            uid,
		ProjectUID:     w.ProjectID,
		ServiceUID:     w.ParentID,
		GroupID:        converter.NonZeroInt64(w.GroupID),
		GroupName:      w.Name,
		Description:    w.Description,
		Type:           w.Type,
		AudienceAccess: w.AudienceAccess,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
	if w.CommitteeID != "" {
		ml.Committees = []model.Committee{{UID: w.CommitteeID}}
	}
	return ml
}

func toWireSubgroupRequest(ml *model.GroupsIOMailingList) *subgroupRequestWire {
	req := &subgroupRequestWire{
		ProjectID:      ml.ProjectUID,
		ParentID:       ml.ServiceUID,
		Name:           ml.GroupName,
		Description:    ml.Description,
		Type:           ml.Type,
		AudienceAccess: ml.AudienceAccess,
	}
	if len(ml.Committees) > 0 {
		req.CommitteeID = ml.Committees[0].UID
	}
	return req
}

// ---- member wire ↔ domain translation helpers ----

func fromWireMember(w *memberWire) *model.GrpsIOMember {
	if w == nil {
		return nil
	}
	createdAt, _ := converter.ParseRFC3339(w.CreatedAt)
	updatedAt, _ := converter.ParseRFC3339(w.UpdatedAt)
	// Resolve UID: POST responses return member_id (int), GET responses return id (string).
	uid := w.ID
	if uid == "" && w.MemberID != 0 {
		uid = strconv.FormatInt(w.MemberID, 10)
	}
	m := &model.GrpsIOMember{
		UID:            uid,
		Email:          w.Email,
		GroupsFullName: w.Name,
		Username:       w.Username,
		DeliveryMode:   w.DeliveryMode,
		ModStatus:      w.ModStatus,
		Status:         w.Status,
		MemberType:     w.MemberType,
		VotingStatus:   w.VotingStatus,
		Organization:   w.Organization,
		JobTitle:       w.JobTitle,
		Role:           w.Role,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
	if w.MemberID != 0 {
		m.MemberID = &w.MemberID
	} else if w.ID != "" {
		if id, err := strconv.ParseInt(w.ID, 10, 64); err == nil {
			m.MemberID = &id
		}
	}
	return m
}

func fromWireArtifactUser(w *artifactUserWire) *model.ArtifactUser {
	if w == nil {
		return nil
	}
	return &model.ArtifactUser{
		ID:             w.ID,
		Username:       w.Username,
		Name:           w.Name,
		Email:          w.Email,
		ProfilePicture: w.ProfilePicture,
	}
}

func fromWireArtifact(w *artifactWire) *model.GroupsIOArtifact {
	if w == nil {
		return nil
	}
	createdAt, _ := converter.ParseRFC3339(w.CreatedAt)
	updatedAt, _ := converter.ParseRFC3339(w.UpdatedAt)
	a := &model.GroupsIOArtifact{
		ArtifactID:          w.ArtifactID,
		GroupID:             w.GroupID,
		ProjectUID:          w.ProjectID,
		CommitteeUID:        w.CommitteeID,
		Type:                w.Type,
		MediaType:           w.MediaType,
		Filename:            w.Filename,
		LinkURL:             w.LinkURL,
		DownloadURL:         w.DownloadURL,
		S3Key:               w.S3Key,
		FileUploaded:        w.FileUploaded,
		FileUploadStatus:    w.FileUploadStatus,
		MessageIDs:          w.MessageIDs,
		LastPostedMessageID: w.LastPostedMessageID,
		Description:         w.Description,
		CreatedBy:           fromWireArtifactUser(w.CreatedBy),
		LastModifiedBy:      fromWireArtifactUser(w.LastModifiedBy),
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
	if t, err := converter.ParseRFC3339(w.FileUploadedAt); err == nil && !t.IsZero() {
		a.FileUploadedAt = &t
	}
	if t, err := converter.ParseRFC3339(w.LastPostedAt); err == nil && !t.IsZero() {
		a.LastPostedAt = &t
	}
	return a
}

func toWireMemberRequest(m *model.GrpsIOMember) *memberRequestWire {
	return &memberRequestWire{
		Email:        m.Email,
		Name:         m.GroupsFullName,
		DeliveryMode: m.DeliveryMode,
		MemberType:   m.MemberType,
		ModStatus:    m.ModStatus,
		Organization: m.Organization,
		JobTitle:     m.JobTitle,
	}
}
