// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// convertGrpsIOServiceCreatePayloadToDomain converts GOA payload to domain model
// convertPayloadToDomain
func (s *mailingListService) convertGrpsIOServiceCreatePayloadToDomain(p *mailinglistservice.CreateGrpsioServicePayload) *model.GrpsIOService {
	// Check for nil payload to avoid panic
	if p == nil {
		return &model.GrpsIOService{}
	}

	now := time.Now()
	service := &model.GrpsIOService{
		Type:         p.Type,
		Domain:       payloadStringValue(p.Domain),
		GroupID:      payloadInt64Value(p.GroupID),
		Status:       payloadStringValue(p.Status),
		GlobalOwners: p.GlobalOwners,
		Prefix:       payloadStringValue(p.Prefix),
		ProjectSlug:  payloadStringValue(p.ProjectSlug),
		ProjectUID:   p.ProjectUID,
		URL:          payloadStringValue(p.URL),
		GroupName:    payloadStringValue(p.GroupName),
		Public:       p.Public,
		Writers:      p.Writers,
		Auditors:     p.Auditors,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return service
}

// convertGrpsIOMailingListPayloadToDomain converts GOA mailing list payload to domain model
func (s *mailingListService) convertGrpsIOMailingListPayloadToDomain(p *mailinglistservice.CreateGrpsioMailingListPayload) *model.GrpsIOMailingList {
	// Check for nil payload to avoid panic
	if p == nil {
		return &model.GrpsIOMailingList{}
	}

	now := time.Now()
	mailingList := &model.GrpsIOMailingList{
		GroupName:        p.GroupName,
		Public:           p.Public,
		Type:             p.Type,
		CommitteeUID:     payloadStringValue(p.CommitteeUID),
		CommitteeFilters: p.CommitteeFilters,
		Description:      p.Description,
		Title:            p.Title,
		SubjectTag:       payloadStringValue(p.SubjectTag),
		ServiceUID:       p.ServiceUID,
		// project_uid is intentionally NOT set here - it will be inherited from parent in orchestrator
		Writers:   p.Writers,
		Auditors:  p.Auditors,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return mailingList
}

// convertGrpsIOServiceUpdatePayloadToDomain converts GOA update payload to domain model
// convertPayloadToUpdateBase
func (s *mailingListService) convertGrpsIOServiceUpdatePayloadToDomain(existing *model.GrpsIOService, p *mailinglistservice.UpdateGrpsioServicePayload) *model.GrpsIOService {
	// Check for nil payload or existing to avoid panic
	if p == nil || p.UID == nil || existing == nil {
		return &model.GrpsIOService{}
	}

	now := time.Now()
	updated := &model.GrpsIOService{
		// Preserve immutable fields from existing service
		UID:            *p.UID,
		Domain:         existing.Domain,
		GroupID:        existing.GroupID,
		Prefix:         existing.Prefix,
		ProjectSlug:    existing.ProjectSlug,
		ProjectName:    existing.ProjectName,
		ProjectUID:     existing.ProjectUID,
		URL:            existing.URL,
		GroupName:      existing.GroupName,
		CreatedAt:      existing.CreatedAt,
		LastReviewedAt: existing.LastReviewedAt,
		LastReviewedBy: existing.LastReviewedBy,
		UpdatedAt:      now,
	}

	// Handle conditionally updateable fields - preserve existing if not provided
	if p.Type != nil {
		updated.Type = *p.Type
	} else {
		updated.Type = existing.Type
	}

	if p.Status != nil {
		updated.Status = *p.Status
	} else {
		updated.Status = existing.Status
	}

	if p.Public != nil {
		updated.Public = *p.Public
	} else {
		updated.Public = existing.Public
	}

	if p.ProjectUID != nil {
		updated.ProjectUID = *p.ProjectUID
	} else {
		updated.ProjectUID = existing.ProjectUID
	}

	// Handle slice fields
	if p.GlobalOwners != nil {
		updated.GlobalOwners = p.GlobalOwners
	} else {
		updated.GlobalOwners = existing.GlobalOwners
	}

	if p.Writers != nil {
		updated.Writers = p.Writers
	} else {
		updated.Writers = existing.Writers
	}

	if p.Auditors != nil {
		updated.Auditors = p.Auditors
	} else {
		updated.Auditors = existing.Auditors
	}

	return updated
}

// convertGrpsIOMailingListUpdatePayloadToDomain converts an update payload to domain model
func (s *mailingListService) convertGrpsIOMailingListUpdatePayloadToDomain(existing *model.GrpsIOMailingList, payload *mailinglistservice.UpdateGrpsioMailingListPayload) *model.GrpsIOMailingList {
	// Start from existing to preserve immutable/readonly fields
	updated := &model.GrpsIOMailingList{
		UID:            existing.UID,
		ProjectUID:     existing.ProjectUID,
		ProjectName:    existing.ProjectName,
		ProjectSlug:    existing.ProjectSlug,
		CreatedAt:      existing.CreatedAt,
		UpdatedAt:      time.Now().UTC(),
		LastReviewedAt: existing.LastReviewedAt,
		LastReviewedBy: existing.LastReviewedBy,
	}

	// Handle conditionally updateable fields - preserve existing if not provided
	if payload.GroupName != nil {
		updated.GroupName = *payload.GroupName
	} else {
		updated.GroupName = existing.GroupName
	}

	if payload.Public != nil {
		updated.Public = *payload.Public
	} else {
		updated.Public = existing.Public
	}

	if payload.Type != nil {
		updated.Type = *payload.Type
	} else {
		updated.Type = existing.Type
	}

	if payload.Description != nil {
		updated.Description = *payload.Description
	} else {
		updated.Description = existing.Description
	}

	if payload.Title != nil {
		updated.Title = *payload.Title
	} else {
		updated.Title = existing.Title
	}

	if payload.ServiceUID != nil {
		updated.ServiceUID = *payload.ServiceUID
	} else {
		updated.ServiceUID = existing.ServiceUID
	}

	if payload.CommitteeUID != nil {
		updated.CommitteeUID = *payload.CommitteeUID
	} else {
		updated.CommitteeUID = existing.CommitteeUID
	}

	if payload.SubjectTag != nil {
		updated.SubjectTag = *payload.SubjectTag
	} else {
		updated.SubjectTag = existing.SubjectTag
	}

	// Handle slice fields
	if payload.CommitteeFilters != nil {
		updated.CommitteeFilters = payload.CommitteeFilters
	} else {
		updated.CommitteeFilters = existing.CommitteeFilters
	}

	if payload.Writers != nil {
		updated.Writers = payload.Writers
	} else {
		updated.Writers = existing.Writers
	}

	if payload.Auditors != nil {
		updated.Auditors = payload.Auditors
	} else {
		updated.Auditors = existing.Auditors
	}

	return updated
}

// convertGrpsIOMemberPayloadToDomain converts GOA member payload to domain model
func (s *mailingListService) convertGrpsIOMemberPayloadToDomain(payload *mailinglistservice.CreateGrpsioMailingListMemberPayload) *model.GrpsIOMember {
	now := time.Now().UTC()
	member := &model.GrpsIOMember{
		MailingListUID: payload.UID,
		Email:          payload.Email,
		MemberType:     payload.MemberType,
		DeliveryMode:   payload.DeliveryMode,
		ModStatus:      payload.ModStatus,
		Status:         "normal",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Handle required fields that might be pointers
	if payload.FirstName != nil {
		member.FirstName = *payload.FirstName
	}
	if payload.LastName != nil {
		member.LastName = *payload.LastName
	}

	// Handle optional fields
	if payload.Username != nil {
		member.Username = *payload.Username
	}
	if payload.Organization != nil {
		member.Organization = *payload.Organization
	}
	if payload.JobTitle != nil {
		member.JobTitle = *payload.JobTitle
	}
	if payload.LastReviewedAt != nil {
		member.LastReviewedAt = payload.LastReviewedAt
	}
	if payload.LastReviewedBy != nil {
		member.LastReviewedBy = payload.LastReviewedBy
	}

	return member
}

// convertGrpsIOMemberUpdatePayloadToDomain converts update payload to domain member model
func (s *mailingListService) convertGrpsIOMemberUpdatePayloadToDomain(payload *mailinglistservice.UpdateGrpsioMailingListMemberPayload, existing *model.GrpsIOMember) *model.GrpsIOMember {
	// Start with existing member to preserve immutable fields
	updated := &model.GrpsIOMember{
		UID:              existing.UID,
		MailingListUID:   existing.MailingListUID,
		Email:            existing.Email,      // Immutable
		MemberType:       existing.MemberType, // Immutable for now
		GroupsIOMemberID: existing.GroupsIOMemberID,
		GroupsIOGroupID:  existing.GroupsIOGroupID,
		CreatedAt:        existing.CreatedAt,
		Status:           existing.Status,
	}

	// Update mutable fields from payload
	if payload.Username != nil {
		updated.Username = *payload.Username
	} else {
		updated.Username = existing.Username
	}

	if payload.FirstName != nil {
		updated.FirstName = *payload.FirstName
	} else {
		updated.FirstName = existing.FirstName
	}

	if payload.LastName != nil {
		updated.LastName = *payload.LastName
	} else {
		updated.LastName = existing.LastName
	}

	if payload.Organization != nil {
		updated.Organization = *payload.Organization
	} else {
		updated.Organization = existing.Organization
	}

	if payload.JobTitle != nil {
		updated.JobTitle = *payload.JobTitle
	} else {
		updated.JobTitle = existing.JobTitle
	}

	// DeliveryMode and ModStatus are now pointers - apply only when provided
	if payload.DeliveryMode != nil {
		updated.DeliveryMode = *payload.DeliveryMode
	} else {
		updated.DeliveryMode = existing.DeliveryMode
	}

	if payload.ModStatus != nil {
		updated.ModStatus = *payload.ModStatus
	} else {
		updated.ModStatus = existing.ModStatus
	}

	// Note: Access control is managed at the mailing list level

	// Set update timestamp
	updated.UpdatedAt = time.Now().UTC()

	// Preserve other existing fields
	updated.LastReviewedAt = existing.LastReviewedAt
	updated.LastReviewedBy = existing.LastReviewedBy

	return updated
}
