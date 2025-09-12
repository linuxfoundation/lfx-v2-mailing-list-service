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
	return &model.GrpsIOService{
		// Preserve immutable fields from existing service
		Type:           existing.Type,
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

		// Update only mutable fields
		Status:       payloadStringValue(p.Status),
		GlobalOwners: p.GlobalOwners,
		Public:       p.Public,
		Writers:      p.Writers,
		Auditors:     p.Auditors,
		UpdatedAt:    now,
	}
}

// convertGrpsIOMailingListUpdatePayloadToDomain converts an update payload to domain model
func (s *mailingListService) convertGrpsIOMailingListUpdatePayloadToDomain(payload *mailinglistservice.UpdateGrpsioMailingListPayload) *model.GrpsIOMailingList {
	// Create a new mailing list from payload data
	mailingList := &model.GrpsIOMailingList{
		GroupName:   payload.GroupName,
		Public:      payload.Public,
		Type:        payload.Type,
		Description: payload.Description,
		Title:       payload.Title,
		ServiceUID:  payload.ServiceUID,
	}

	// Handle pointer fields
	if payload.CommitteeUID != nil {
		mailingList.CommitteeUID = *payload.CommitteeUID
	}
	if payload.SubjectTag != nil {
		mailingList.SubjectTag = *payload.SubjectTag
	}

	// Handle slice fields
	if payload.CommitteeFilters != nil {
		mailingList.CommitteeFilters = payload.CommitteeFilters
	}
	if payload.Writers != nil {
		mailingList.Writers = payload.Writers
	}
	if payload.Auditors != nil {
		mailingList.Auditors = payload.Auditors
	}

	return mailingList
}

// convertGrpsIOMemberPayloadToDomain converts GOA member payload to domain model
func (s *mailingListService) convertGrpsIOMemberPayloadToDomain(payload *mailinglistservice.CreateGrpsioMailingListMemberPayload) *model.GrpsIOMember {
	member := &model.GrpsIOMember{
		MailingListUID: payload.UID,
		Email:          payload.Email,
		MemberType:     payload.MemberType,
		DeliveryMode:   payload.DeliveryMode,
		ModStatus:      payload.ModStatus,
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

	// DeliveryMode and ModStatus are strings in the payload, not pointers
	if payload.DeliveryMode != "" {
		updated.DeliveryMode = payload.DeliveryMode
	} else {
		updated.DeliveryMode = existing.DeliveryMode
	}

	if payload.ModStatus != "" {
		updated.ModStatus = payload.ModStatus
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
