// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// convertDomainToFullResponse converts domain model to full response (for CREATE operations)
// Following convertDomainToFullResponse
func (s *mailingListService) convertDomainToFullResponse(service *model.GrpsIOService) *mailinglistservice.ServiceFull {
	if service == nil {
		return &mailinglistservice.ServiceFull{}
	}

	result := &mailinglistservice.ServiceFull{
		UID:          &service.UID,
		Type:         service.Type,
		Domain:       &service.Domain,
		GroupID:      &service.GroupID,
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

// convertDomainToStandardResponse converts domain model to standard response (for GET/UPDATE operations)
// convertBaseToResponse
func (s *mailingListService) convertDomainToStandardResponse(service *model.GrpsIOService) *mailinglistservice.ServiceWithReadonlyAttributes {
	if service == nil {
		return &mailinglistservice.ServiceWithReadonlyAttributes{}
	}

	result := &mailinglistservice.ServiceWithReadonlyAttributes{
		UID:          &service.UID,
		Type:         service.Type,
		Domain:       &service.Domain,
		GroupID:      &service.GroupID,
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

// convertMailingListDomainToResponse converts domain mailing list to full response (for CREATE operations)
func (s *mailingListService) convertMailingListDomainToResponse(ml *model.GrpsIOMailingList) *mailinglistservice.MailingListFull {
	if ml == nil {
		return &mailinglistservice.MailingListFull{}
	}

	result := &mailinglistservice.MailingListFull{
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

// stringToPointer converts empty string to nil pointer, non-empty string to pointer
func stringToPointer(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
