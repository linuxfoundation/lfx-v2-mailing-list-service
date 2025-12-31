// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
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
		Type:             p.Type,
		Domain:           payloadStringValue(p.Domain),
		GroupID:          payloadInt64Ptr(p.GroupID),
		Status:           payloadStringValue(p.Status),
		GlobalOwners:     p.GlobalOwners,
		Prefix:           payloadStringValue(p.Prefix),
		ParentServiceUID: payloadStringValue(p.ParentServiceUID),
		ProjectSlug:      payloadStringValue(p.ProjectSlug),
		ProjectUID:       p.ProjectUID,
		URL:              payloadStringValue(p.URL),
		GroupName:        payloadStringValue(p.GroupName),
		Public:           p.Public,
		Writers:          p.Writers,
		Auditors:         p.Auditors,
		Source:           constants.SourceAPI, // API operations always use api source
		CreatedAt:        now,
		UpdatedAt:        now,
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
		GroupName:      p.GroupName,
		Public:         p.Public,
		Type:           p.Type,
		AudienceAccess: p.AudienceAccess,
		Committees:     convertCommitteesToDomain(p.Committees),
		Description:    p.Description,
		Title:          p.Title,
		SubjectTag:     payloadStringValue(p.SubjectTag),
		ServiceUID:     p.ServiceUID,
		// project_uid is intentionally NOT set here - it will be inherited from parent in orchestrator
		Source:    constants.SourceAPI, // API operations always use api source
		Writers:   p.Writers,
		Auditors:  p.Auditors,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return mailingList
}

// convertCommitteesToDomain converts GOA Committee array to domain model Committee array
func convertCommitteesToDomain(committees []*mailinglistservice.Committee) []model.Committee {
	if committees == nil {
		return nil
	}

	result := make([]model.Committee, 0, len(committees))
	for _, c := range committees {
		if c == nil {
			continue
		}
		result = append(result, model.Committee{
			UID:                   c.UID,
			Name:                  payloadStringValue(c.Name), // Name is read-only, but may be passed through
			AllowedVotingStatuses: c.AllowedVotingStatuses,
		})
	}
	return result
}

// convertGrpsIOServiceUpdatePayloadToDomain converts GOA update payload to domain model
func (s *mailingListService) convertGrpsIOServiceUpdatePayloadToDomain(existing *model.GrpsIOService, p *mailinglistservice.UpdateGrpsioServicePayload) *model.GrpsIOService {
	// Check for nil payload or existing to avoid panic
	if p == nil || p.UID == nil || existing == nil {
		return &model.GrpsIOService{}
	}

	now := time.Now()
	return &model.GrpsIOService{
		// Preserve ALL immutable fields from existing service
		UID:              *p.UID,
		Type:             existing.Type, // Fixed: preserve from existing, not payload
		Domain:           existing.Domain,
		GroupID:          existing.GroupID,
		Prefix:           existing.Prefix,
		ProjectSlug:      existing.ProjectSlug,
		ProjectName:      existing.ProjectName,
		ParentServiceUID: existing.ParentServiceUID,
		URL:              existing.URL,       // Fixed: add missing field preservation
		GroupName:        existing.GroupName, // Fixed: add missing field preservation
		CreatedAt:        existing.CreatedAt,
		LastReviewedAt:   existing.LastReviewedAt,
		LastReviewedBy:   existing.LastReviewedBy,

		// Update mutable fields (PUT semantics - complete replacement)
		Status:       payloadStringValue(p.Status), // nil → ""
		ProjectUID:   existing.ProjectUID,          // IMMUTABLE (keep as is)
		Public:       p.Public,                     // Direct assignment
		GlobalOwners: p.GlobalOwners,               // nil → nil
		Writers:      p.Writers,                    // nil → nil
		Auditors:     p.Auditors,                   // nil → nil
		UpdatedAt:    now,
	}
}

// convertGrpsIOMailingListUpdatePayloadToDomain converts an update payload to domain model
func (s *mailingListService) convertGrpsIOMailingListUpdatePayloadToDomain(existing *model.GrpsIOMailingList, payload *mailinglistservice.UpdateGrpsioMailingListPayload) *model.GrpsIOMailingList {
	// Create updated mailing list from payload data (PUT semantics)
	return &model.GrpsIOMailingList{
		// Preserve immutable/readonly fields
		UID:            existing.UID,
		GroupName:      existing.GroupName, // Fixed: GroupName is immutable, preserve from existing
		ProjectUID:     existing.ProjectUID,
		ProjectName:    existing.ProjectName,
		ProjectSlug:    existing.ProjectSlug,
		CreatedAt:      existing.CreatedAt,
		LastReviewedAt: existing.LastReviewedAt,
		LastReviewedBy: existing.LastReviewedBy,

		// Update all mutable fields (PUT semantics - complete replacement)
		Public:         payload.Public,                                // Direct assignment
		AudienceAccess: payload.AudienceAccess,                        // Direct assignment
		Type:           payload.Type,                                  // Direct assignment
		Description:    payload.Description,                           // Direct assignment
		Title:          payload.Title,                                 // Direct assignment
		ServiceUID:     payload.ServiceUID,                            // Direct assignment
		Committees:     convertCommitteesToDomain(payload.Committees), // nil → nil
		SubjectTag:     payloadStringValue(payload.SubjectTag),        // nil → ""
		Writers:        payload.Writers,                               // nil → nil
		Auditors:       payload.Auditors,                              // nil → nil
		UpdatedAt:      time.Now().UTC(),
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
		Source:         constants.SourceAPI, // API operations always use api source
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

		// Update all mutable fields (PUT semantics - complete replacement)
		Username:     payloadStringValue(payload.Username),     // nil → ""
		FirstName:    payloadStringValue(payload.FirstName),    // nil → ""
		LastName:     payloadStringValue(payload.LastName),     // nil → ""
		Organization: payloadStringValue(payload.Organization), // nil → ""
		JobTitle:     payloadStringValue(payload.JobTitle),     // nil → ""
		DeliveryMode: payload.DeliveryMode,                     // Direct (always has value)
		ModStatus:    payload.ModStatus,                        // Direct (always has value)
		UpdatedAt:    time.Now().UTC(),
	}
}

// convertWebhookGroupInfo converts webhook group data to domain model
func (s *mailingListService) convertWebhookGroupInfo(m map[string]any) (*model.GroupInfo, error) {
	if m == nil {
		return nil, errors.NewValidation("group info is nil")
	}

	group := &model.GroupInfo{}

	// Required field: ID
	if id, ok := m["id"].(float64); ok {
		group.ID = int(id)
	} else {
		return nil, errors.NewValidation("group id is missing or invalid")
	}

	// Required field: Name
	if name, ok := m["name"].(string); ok {
		group.Name = name
	} else {
		return nil, errors.NewValidation("group name is missing or invalid")
	}

	// Required field: ParentGroupID
	if parentGroupID, ok := m["parent_group_id"].(float64); ok {
		group.ParentGroupID = int(parentGroupID)
	} else {
		return nil, errors.NewValidation("parent_group_id is missing or invalid")
	}

	return group, nil
}

// convertWebhookMemberInfo converts webhook member data to domain model
func (s *mailingListService) convertWebhookMemberInfo(m map[string]any) (*model.MemberInfo, error) {
	if m == nil {
		return nil, errors.NewValidation("member info is nil")
	}

	member := &model.MemberInfo{}

	// Required field: ID
	if id, ok := m["id"].(float64); ok {
		member.ID = int(id)
	} else {
		return nil, errors.NewValidation("member id is missing or invalid")
	}

	// Required field: GroupID
	if groupID, ok := m["group_id"].(float64); ok {
		member.GroupID = uint64(groupID)
	} else {
		return nil, errors.NewValidation("group_id is missing or invalid")
	}

	// Required field: Email
	if email, ok := m["email"].(string); ok {
		member.Email = email
	} else {
		return nil, errors.NewValidation("email is missing or invalid")
	}

	// Required field: Status
	if status, ok := m["status"].(string); ok {
		member.Status = status
	} else {
		return nil, errors.NewValidation("status is missing or invalid")
	}

	// Optional fields: UserID, GroupName
	if userID, ok := m["user_id"].(float64); ok {
		member.UserID = int(userID)
	}
	if groupName, ok := m["group_name"].(string); ok {
		member.GroupName = groupName
	}

	return member, nil
}
