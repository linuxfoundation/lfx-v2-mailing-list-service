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
func (s *mailingListService) convertGrpsIOServiceDomainToFullResponse(service *model.GrpsIOService, settings *model.GrpsIOServiceSettings) *mailinglistservice.GrpsIoServiceFull {
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

	// Only set ParentServiceUID if it's non-empty
	if service.ParentServiceUID != "" {
		result.ParentServiceUID = &service.ParentServiceUID
	}

	// Populate writers and auditors from settings
	if settings != nil {
		if len(settings.Writers) > 0 {
			result.Writers = make([]*mailinglistservice.UserInfo, len(settings.Writers))
			for i, writer := range settings.Writers {
				result.Writers[i] = &mailinglistservice.UserInfo{
					Name:     &writer.Name,
					Email:    &writer.Email,
					Username: &writer.Username,
					Avatar:   &writer.Avatar,
				}
			}
		}
		if len(settings.Auditors) > 0 {
			result.Auditors = make([]*mailinglistservice.UserInfo, len(settings.Auditors))
			for i, auditor := range settings.Auditors {
				result.Auditors[i] = &mailinglistservice.UserInfo{
					Name:     &auditor.Name,
					Email:    &auditor.Email,
					Username: &auditor.Username,
					Avatar:   &auditor.Avatar,
				}
			}
		}
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

	// Only set ParentServiceUID if it's non-empty
	if service.ParentServiceUID != "" {
		result.ParentServiceUID = &service.ParentServiceUID
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

	return result
}

// convertGrpsIOMailingListDomainToResponse converts domain mailing list to full response (for CREATE operations)
func (s *mailingListService) convertGrpsIOMailingListDomainToResponse(ml *model.GrpsIOMailingList, settings *model.GrpsIOMailingListSettings) *mailinglistservice.GrpsIoMailingListFull {
	if ml == nil {
		return &mailinglistservice.GrpsIoMailingListFull{}
	}

	result := &mailinglistservice.GrpsIoMailingListFull{
		UID:            &ml.UID,
		GroupName:      &ml.GroupName,
		Public:         ml.Public,
		AudienceAccess: ml.AudienceAccess,
		Type:           &ml.Type,
		Committees:     convertCommitteesToResponse(ml.Committees),
		Description:    &ml.Description,
		Title:          &ml.Title,
		SubjectTag:     stringToPointer(ml.SubjectTag),
		ServiceUID:     &ml.ServiceUID,
		ProjectUID:     &ml.ProjectUID,  // This is inherited from parent in orchestrator
		ProjectName:    &ml.ProjectName, // Inherited from parent service
		ProjectSlug:    &ml.ProjectSlug, // Inherited from parent service
	}

	// Add writers and auditors from settings
	if settings != nil {
		result.Writers = convertUserInfoDomainToResponse(settings.Writers)
		result.Auditors = convertUserInfoDomainToResponse(settings.Auditors)
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

	return result
}

// convertCommitteesToResponse converts domain Committee array to GOA Committee array
func convertCommitteesToResponse(committees []model.Committee) []*mailinglistservice.Committee {
	if committees == nil {
		return nil
	}

	result := make([]*mailinglistservice.Committee, 0, len(committees))
	for _, c := range committees {
		result = append(result, &mailinglistservice.Committee{
			UID:                   c.UID,
			Name:                  stringToPointer(c.Name),
			AllowedVotingStatuses: c.AllowedVotingStatuses,
		})
	}
	return result
}

// convertGrpsIOMailingListDomainToStandardResponse converts a domain mailing list to GOA standard response type
func (s *mailingListService) convertGrpsIOMailingListDomainToStandardResponse(mailingList *model.GrpsIOMailingList) *mailinglistservice.GrpsIoMailingListWithReadonlyAttributes {
	if mailingList == nil {
		return &mailinglistservice.GrpsIoMailingListWithReadonlyAttributes{}
	}

	response := &mailinglistservice.GrpsIoMailingListWithReadonlyAttributes{
		UID:            &mailingList.UID,
		GroupName:      &mailingList.GroupName,
		Public:         mailingList.Public,
		AudienceAccess: mailingList.AudienceAccess,
		Type:           &mailingList.Type,
		Committees:     convertCommitteesToResponse(mailingList.Committees),
		Description:    &mailingList.Description,
		Title:          &mailingList.Title,
		SubjectTag:     stringToPointer(mailingList.SubjectTag),
		ServiceUID:     &mailingList.ServiceUID,
		ProjectUID:     stringToPointer(mailingList.ProjectUID),
		ProjectName:    stringToPointer(mailingList.ProjectName),
		ProjectSlug:    stringToPointer(mailingList.ProjectSlug),
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

// convertGrpsIOServiceSettingsDomainToResponse converts domain settings to GOA response
func (s *mailingListService) convertGrpsIOServiceSettingsDomainToResponse(settings *model.GrpsIOServiceSettings) *mailinglistservice.GrpsIoServiceSettings {
	createdAt := settings.CreatedAt.Format(time.RFC3339)
	updatedAt := settings.UpdatedAt.Format(time.RFC3339)

	response := &mailinglistservice.GrpsIoServiceSettings{
		UID:             &settings.UID,
		Writers:         convertUserInfoDomainToResponse(settings.Writers),
		Auditors:        convertUserInfoDomainToResponse(settings.Auditors),
		LastReviewedAt:  settings.LastReviewedAt,
		LastReviewedBy:  settings.LastReviewedBy,
		LastAuditedBy:   settings.LastAuditedBy,
		LastAuditedTime: settings.LastAuditedTime,
		CreatedAt:       &createdAt,
		UpdatedAt:       &updatedAt,
	}

	return response
}

// convertUserInfoDomainToResponse converts domain UserInfo array to GOA UserInfo array
func convertUserInfoDomainToResponse(domainUsers []model.UserInfo) []*mailinglistservice.UserInfo {
	if domainUsers == nil {
		return []*mailinglistservice.UserInfo{}
	}

	users := make([]*mailinglistservice.UserInfo, len(domainUsers))
	for i, u := range domainUsers {
		users[i] = &mailinglistservice.UserInfo{
			Name:     &u.Name,
			Email:    &u.Email,
			Username: &u.Username,
			Avatar:   &u.Avatar,
		}
	}
	return users
}

// convertGrpsIOMailingListSettingsDomainToResponse converts domain mailing list settings to GOA response
func (s *mailingListService) convertGrpsIOMailingListSettingsDomainToResponse(settings *model.GrpsIOMailingListSettings) *mailinglistservice.GrpsIoMailingListSettings {
	createdAt := settings.CreatedAt.Format(time.RFC3339)
	updatedAt := settings.UpdatedAt.Format(time.RFC3339)

	response := &mailinglistservice.GrpsIoMailingListSettings{
		UID:             &settings.UID,
		Writers:         convertUserInfoDomainToResponse(settings.Writers),
		Auditors:        convertUserInfoDomainToResponse(settings.Auditors),
		LastReviewedAt:  settings.LastReviewedAt,
		LastReviewedBy:  settings.LastReviewedBy,
		LastAuditedBy:   settings.LastAuditedBy,
		LastAuditedTime: settings.LastAuditedTime,
		CreatedAt:       &createdAt,
		UpdatedAt:       &updatedAt,
	}

	return response
}
