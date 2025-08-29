// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// convertCreatePayloadToDomain converts GOA payload to domain model
// convertPayloadToDomain
func (s *mailingListService) convertCreatePayloadToDomain(p *mailinglistservice.CreateGrpsioServicePayload) *model.GrpsIOService {
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

// convertMailingListPayloadToDomain converts GOA mailing list payload to domain model
func (s *mailingListService) convertMailingListPayloadToDomain(p *mailinglistservice.CreateGrpsioMailingListPayload) *model.GrpsIOMailingList {
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

// convertUpdatePayloadToDomain converts GOA update payload to domain model
// convertPayloadToUpdateBase
func (s *mailingListService) convertUpdatePayloadToDomain(existing *model.GrpsIOService, p *mailinglistservice.UpdateGrpsioServicePayload) *model.GrpsIOService {
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
