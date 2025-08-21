// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// convertDomainToFullResponse converts domain model to full response (for CREATE operations)
// Following committee service pattern: convertDomainToFullResponse
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

	// Handle timestamps - committee pattern: convert time.Time to *string
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
// Following committee service pattern: convertBaseToResponse
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

	// Handle timestamps - committee pattern: convert time.Time to *string
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
