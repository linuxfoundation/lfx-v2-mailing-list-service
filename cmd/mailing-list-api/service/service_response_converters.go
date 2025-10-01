// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// convertGrpsIOServiceDomainToFullResponse converts domain model to full response (for CREATE operations)
// Following convertGrpsIOServiceDomainToFullResponse
func (s *mailingListService) convertGrpsIOServiceDomainToFullResponse(service *model.GrpsIOService) *mailinglistservice.GrpsIoServiceFull {
	if service == nil {
		return &mailinglistservice.GrpsIoServiceFull{}
	}

	result := &mailinglistservice.GrpsIoServiceFull{
		UID:          &service.UID,
		Type:         service.Type,
		Domain:       &service.Domain,
		GroupID:      service.GroupID,
		Status:       &service.Status,
		GlobalOwners: service.GlobalOwners,
		Prefix:       &service.Prefix,
		ProjectSlug:  &service.ProjectSlug,
		ProjectName:  &service.ProjectName,
		ProjectUID:   service.ProjectUID,
		URL:          &service.URL,
		GroupName:    &service.GroupName,
		Public:       service.Public,
	}

	// Handle timestamps
	if !service.CreatedAt.IsZero() {
		createdAt := service.CreatedAt.Format(time.RFC3339)
		result.CreatedAt = &createdAt
	}

	if !service.UpdatedAt.IsZero() {
		updatedAt := service.UpdatedAt.Format(time.RFC3339)
		result.UpdatedAt = &updatedAt
	}

	result.Writers = service.Writers
	result.Auditors = service.Auditors
	result.LastReviewedAt = service.LastReviewedAt
	result.LastReviewedBy = service.LastReviewedBy

	return result
}

// convertGrpsIOServiceDomainToStandardResponse converts domain model to standard response (for GET/UPDATE operations)
// convertBaseToResponse
func (s *mailingListService) convertGrpsIOServiceDomainToStandardResponse(service *model.GrpsIOService) *mailinglistservice.GrpsIoServiceWithReadonlyAttributes {
	if service == nil {
		return &mailinglistservice.GrpsIoServiceWithReadonlyAttributes{}
	}

	result := &mailinglistservice.GrpsIoServiceWithReadonlyAttributes{
		UID:          &service.UID,
		Type:         service.Type,
		Domain:       &service.Domain,
		GroupID:      service.GroupID,
		Status:       &service.Status,
		GlobalOwners: service.GlobalOwners,
		Prefix:       &service.Prefix,
		ProjectSlug:  &service.ProjectSlug,
		ProjectName:  &service.ProjectName,
		ProjectUID:   service.ProjectUID,
		URL:          &service.URL,
		GroupName:    &service.GroupName,
		Public:       service.Public,
	}

	// Handle timestamps
	if !service.CreatedAt.IsZero() {
		createdAt := service.CreatedAt.Format(time.RFC3339)
		result.CreatedAt = &createdAt
	}

	if !service.UpdatedAt.IsZero() {
		updatedAt := service.UpdatedAt.Format(time.RFC3339)
		result.UpdatedAt = &updatedAt
	}

	result.LastReviewedAt = service.LastReviewedAt
	result.LastReviewedBy = service.LastReviewedBy
	result.Writers = service.Writers
	result.Auditors = service.Auditors

	return result
}

// convertGrpsIOMailingListDomainToResponse converts domain mailing list to full response (for CREATE operations)
func (s *mailingListService) convertGrpsIOMailingListDomainToResponse(ml *model.GrpsIOMailingList) *mailinglistservice.GrpsIoMailingListFull {
	if ml == nil {
		return &mailinglistservice.GrpsIoMailingListFull{}
	}

	result := &mailinglistservice.GrpsIoMailingListFull{
		UID:              &ml.UID,
		GroupName:        &ml.GroupName,
		Public:           ml.Public,
		Type:             &ml.Type,
		CommitteeUID:     stringToPointer(ml.CommitteeUID),
		CommitteeFilters: ml.CommitteeFilters,
		Description:      &ml.Description,
		Title:            &ml.Title,
		SubjectTag:       stringToPointer(ml.SubjectTag),
		ServiceUID:       &ml.ServiceUID,
		ProjectUID:       &ml.ProjectUID,  // This is inherited from parent in orchestrator
		ProjectName:      &ml.ProjectName, // Inherited from parent service
		ProjectSlug:      &ml.ProjectSlug, // Inherited from parent service
		Writers:          ml.Writers,
		Auditors:         ml.Auditors,
	}

	// Handle timestamps
	if !ml.CreatedAt.IsZero() {
		createdAt := ml.CreatedAt.Format(time.RFC3339)
		result.CreatedAt = &createdAt
	}

	if !ml.UpdatedAt.IsZero() {
		updatedAt := ml.UpdatedAt.Format(time.RFC3339)
		result.UpdatedAt = &updatedAt
	}

	result.LastReviewedAt = ml.LastReviewedAt
	result.LastReviewedBy = ml.LastReviewedBy

	return result
}

// convertGrpsIOMailingListDomainToStandardResponse converts a domain mailing list to GOA standard response type
func (s *mailingListService) convertGrpsIOMailingListDomainToStandardResponse(mailingList *model.GrpsIOMailingList) *mailinglistservice.GrpsIoMailingListWithReadonlyAttributes {
	if mailingList == nil {
		return &mailinglistservice.GrpsIoMailingListWithReadonlyAttributes{}
	}

	response := &mailinglistservice.GrpsIoMailingListWithReadonlyAttributes{
		UID:              &mailingList.UID,
		GroupName:        &mailingList.GroupName,
		Public:           mailingList.Public,
		Type:             &mailingList.Type,
		CommitteeUID:     stringToPointer(mailingList.CommitteeUID),
		CommitteeFilters: mailingList.CommitteeFilters,
		Description:      &mailingList.Description,
		Title:            &mailingList.Title,
		SubjectTag:       stringToPointer(mailingList.SubjectTag),
		ServiceUID:       &mailingList.ServiceUID,
		ProjectUID:       stringToPointer(mailingList.ProjectUID),
		ProjectName:      stringToPointer(mailingList.ProjectName),
		ProjectSlug:      stringToPointer(mailingList.ProjectSlug),
		Writers:          mailingList.Writers,
		Auditors:         mailingList.Auditors,
	}

	// Convert timestamps
	if !mailingList.CreatedAt.IsZero() {
		createdAt := mailingList.CreatedAt.Format(time.RFC3339)
		response.CreatedAt = &createdAt
	}
	if !mailingList.UpdatedAt.IsZero() {
		updatedAt := mailingList.UpdatedAt.Format(time.RFC3339)
		response.UpdatedAt = &updatedAt
	}

	// Note: LastReviewedAt/By fields are not in MailingListWithReadonlyAttributes
	// They might be in a different response type or future enhancement

	return response
}

// convertGrpsIOMemberToResponse converts domain member model to API response
func (s *mailingListService) convertGrpsIOMemberToResponse(member *model.GrpsIOMember) *mailinglistservice.GrpsIoMemberWithReadonlyAttributes {
	if member == nil {
		return &mailinglistservice.GrpsIoMemberWithReadonlyAttributes{}
	}

	result := &mailinglistservice.GrpsIoMemberWithReadonlyAttributes{
		UID:            &member.UID,
		MailingListUID: &member.MailingListUID,
		Username:       &member.Username,
		FirstName:      &member.FirstName,
		LastName:       &member.LastName,
		Email:          &member.Email,
		Organization:   &member.Organization,
		JobTitle:       &member.JobTitle,
		MemberType:     member.MemberType,
		DeliveryMode:   member.DeliveryMode,
		ModStatus:      member.ModStatus,
		Status:         &member.Status,
	}

	// Handle optional GroupsIO fields
	if member.GroupsIOMemberID != nil && *member.GroupsIOMemberID > 0 {
		result.GroupsioMemberID = member.GroupsIOMemberID
	}
	if member.GroupsIOGroupID != nil && *member.GroupsIOGroupID > 0 {
		result.GroupsioGroupID = member.GroupsIOGroupID
	}

	// Handle timestamps
	if !member.CreatedAt.IsZero() {
		createdAt := member.CreatedAt.Format(time.RFC3339)
		result.CreatedAt = &createdAt
	}
	if !member.UpdatedAt.IsZero() {
		updatedAt := member.UpdatedAt.Format(time.RFC3339)
		result.UpdatedAt = &updatedAt
	}

	// Handle optional string fields (nullable in domain model)
	if member.LastReviewedAt != nil && *member.LastReviewedAt != "" {
		result.LastReviewedAt = member.LastReviewedAt
	}
	if member.LastReviewedBy != nil && *member.LastReviewedBy != "" {
		result.LastReviewedBy = member.LastReviewedBy
	}

	return result
}

// convertGrpsIOMemberDomainToResponse converts domain member to GOA response
func (s *mailingListService) convertGrpsIOMemberDomainToResponse(member *model.GrpsIOMember) *mailinglistservice.GrpsIoMemberFull {
	response := &mailinglistservice.GrpsIoMemberFull{
		UID:            member.UID,
		MailingListUID: member.MailingListUID,
		FirstName:      member.FirstName,
		LastName:       member.LastName,
		Email:          member.Email,
		MemberType:     member.MemberType,
		DeliveryMode:   member.DeliveryMode,
		ModStatus:      member.ModStatus,
		Status:         member.Status,
	}

	// Handle optional fields
	if member.Username != "" {
		response.Username = &member.Username
	}
	if member.Organization != "" {
		response.Organization = &member.Organization
	}
	if member.JobTitle != "" {
		response.JobTitle = &member.JobTitle
	}
	if member.GroupsIOMemberID != nil && *member.GroupsIOMemberID != 0 {
		response.GroupsioMemberID = member.GroupsIOMemberID
	}
	if member.GroupsIOGroupID != nil && *member.GroupsIOGroupID != 0 {
		response.GroupsioGroupID = member.GroupsIOGroupID
	}
	if member.LastReviewedAt != nil {
		response.LastReviewedAt = member.LastReviewedAt
	}
	if member.LastReviewedBy != nil {
		response.LastReviewedBy = member.LastReviewedBy
	}
	// Note: Access control is managed at the mailing list level

	// Convert timestamps
	if !member.CreatedAt.IsZero() {
		response.CreatedAt = member.CreatedAt.Format(time.RFC3339)
	}
	if !member.UpdatedAt.IsZero() {
		response.UpdatedAt = member.UpdatedAt.Format(time.RFC3339)
	}

	return response
}

// stringToPointer converts empty string to nil pointer, non-empty string to pointer
func stringToPointer(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
