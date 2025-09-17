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
func (s *mailingListService) convertGrpsIOServiceUpdatePayloadToDomain(existing *model.GrpsIOService, p *mailinglistservice.UpdateGrpsioServicePayload) *model.GrpsIOService {
	// Check for nil payload or existing to avoid panic
	if p == nil || p.UID == nil || existing == nil {
		return &model.GrpsIOService{}
	}

	now := time.Now()
	return &model.GrpsIOService{
		// Preserve immutable fields from existing service
		UID:            *p.UID,
		Domain:         existing.Domain,
		GroupID:        existing.GroupID,
		Prefix:         existing.Prefix,
		ProjectSlug:    existing.ProjectSlug,
		ProjectName:    existing.ProjectName,
		CreatedAt:      existing.CreatedAt,
		LastReviewedAt: existing.LastReviewedAt,
		LastReviewedBy: existing.LastReviewedBy,

		// Update mutable fields (PUT semantics - all fields provided)
		Type:         p.Type,
		Status:       payloadStringValue(p.Status),
		ProjectUID:   p.ProjectUID,
		Public:       p.Public,
		GlobalOwners: p.GlobalOwners,
		Writers:      p.Writers,
		Auditors:     p.Auditors,
		UpdatedAt:    now,
	}
}

// convertGrpsIOMailingListUpdatePayloadToDomain converts an update payload to domain model
func (s *mailingListService) convertGrpsIOMailingListUpdatePayloadToDomain(existing *model.GrpsIOMailingList, payload *mailinglistservice.UpdateGrpsioMailingListPayload) *model.GrpsIOMailingList {
	// Create updated mailing list from payload data (PUT semantics)
	return &model.GrpsIOMailingList{
		// Preserve immutable/readonly fields
		UID:            existing.UID,
		ProjectUID:     existing.ProjectUID,
		ProjectName:    existing.ProjectName,
		ProjectSlug:    existing.ProjectSlug,
		CreatedAt:      existing.CreatedAt,
		LastReviewedAt: existing.LastReviewedAt,
		LastReviewedBy: existing.LastReviewedBy,

		// Update all mutable fields (PUT semantics - all fields provided)
		GroupName:        payload.GroupName,
		Public:           payload.Public,
		Type:             payload.Type,
		Description:      payload.Description,
		Title:            payload.Title,
		ServiceUID:       payload.ServiceUID,
		CommitteeUID:     payloadStringValue(payload.CommitteeUID),
		SubjectTag:       payloadStringValue(payload.SubjectTag),
		CommitteeFilters: payload.CommitteeFilters,
		Writers:          payload.Writers,
		Auditors:         payload.Auditors,
		UpdatedAt:        time.Now().UTC(),
	}
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
	// Create updated member from payload data (PUT semantics)
	return &model.GrpsIOMember{
		// Preserve immutable fields
		UID:              existing.UID,
		MailingListUID:   existing.MailingListUID,
		Email:            existing.Email,      // Immutable
		MemberType:       existing.MemberType, // Immutable for now
		GroupsIOMemberID: existing.GroupsIOMemberID,
		GroupsIOGroupID:  existing.GroupsIOGroupID,
		CreatedAt:        existing.CreatedAt,
		Status:           existing.Status,
		LastReviewedAt:   existing.LastReviewedAt,
		LastReviewedBy:   existing.LastReviewedBy,

		// Update all mutable fields (PUT semantics - all fields provided)
		Username:     payloadStringValue(payload.Username),
		FirstName:    payloadStringValue(payload.FirstName),
		LastName:     payloadStringValue(payload.LastName),
		Organization: payloadStringValue(payload.Organization),
		JobTitle:     payloadStringValue(payload.JobTitle),
		DeliveryMode: payload.DeliveryMode,
		ModStatus:    payload.ModStatus,
		UpdatedAt:    time.Now().UTC(),
	}
}
