// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"
	"time"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/stretchr/testify/assert"
)

func TestConvertDomainToFullResponse(t *testing.T) {
	createdAt := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		domain   *model.GrpsIOService
		expected *mailinglistservice.ServiceFull
	}{
		{
			name: "complete domain to full response conversion",
			domain: &model.GrpsIOService{
				UID:            "service-123",
				Type:           "primary",
				Domain:         "example.groups.io",
				GroupID:        12345,
				Status:         "active",
				GlobalOwners:   []string{"owner1@example.com", "owner2@example.com"},
				Prefix:         "",
				ProjectSlug:    "test-project",
				ProjectName:    "Test Project",
				ProjectUID:     "project-123",
				URL:            "https://example.groups.io/g/test",
				GroupName:      "test-group",
				Public:         true,
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				LastReviewedAt: stringPtr("2023-01-01T10:00:00Z"),
				LastReviewedBy: stringPtr("reviewer-123"),
				Writers:        []string{"writer1", "writer2"},
				Auditors:       []string{"auditor1", "auditor2"},
			},
			expected: &mailinglistservice.ServiceFull{
				UID:            stringPtr("service-123"),
				Type:           "primary",
				Domain:         stringPtr("example.groups.io"),
				GroupID:        int64Ptr(12345),
				Status:         stringPtr("active"),
				GlobalOwners:   []string{"owner1@example.com", "owner2@example.com"},
				Prefix:         stringPtr(""),
				ProjectSlug:    stringPtr("test-project"),
				ProjectName:    stringPtr("Test Project"),
				ProjectUID:     "project-123",
				URL:            stringPtr("https://example.groups.io/g/test"),
				GroupName:      stringPtr("test-group"),
				Public:         true,
				CreatedAt:      stringPtr("2023-01-01T12:00:00Z"),
				UpdatedAt:      stringPtr("2023-01-02T12:00:00Z"),
				LastReviewedAt: stringPtr("2023-01-01T10:00:00Z"),
				LastReviewedBy: stringPtr("reviewer-123"),
				Writers:        []string{"writer1", "writer2"},
				Auditors:       []string{"auditor1", "auditor2"},
			},
		},
		{
			name: "minimal domain to full response conversion",
			domain: &model.GrpsIOService{
				Type:       "formation",
				ProjectUID: "project-456",
				Public:     false,
				CreatedAt:  time.Time{}, // Zero timestamp
				UpdatedAt:  time.Time{}, // Zero timestamp
			},
			expected: &mailinglistservice.ServiceFull{
				UID:          stringPtr(""),
				Type:         "formation",
				Domain:       stringPtr(""),
				GroupID:      int64Ptr(0),
				Status:       stringPtr(""),
				GlobalOwners: nil,
				Prefix:       stringPtr(""),
				ProjectSlug:  stringPtr(""),
				ProjectName:  stringPtr(""),
				ProjectUID:   "project-456",
				URL:          stringPtr(""),
				GroupName:    stringPtr(""),
				Public:       false,
				// CreatedAt and UpdatedAt should be nil when timestamps are zero
				CreatedAt:      nil,
				UpdatedAt:      nil,
				LastReviewedAt: nil,
				LastReviewedBy: nil,
				Writers:        nil,
				Auditors:       nil,
			},
		},
		{
			name:     "nil domain",
			domain:   nil,
			expected: &mailinglistservice.ServiceFull{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertDomainToFullResponse(tt.domain)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertDomainToStandardResponse(t *testing.T) {
	createdAt := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		domain   *model.GrpsIOService
		expected *mailinglistservice.ServiceWithReadonlyAttributes
	}{
		{
			name: "complete domain to standard response conversion",
			domain: &model.GrpsIOService{
				UID:            "service-123",
				Type:           "shared",
				Domain:         "shared.groups.io",
				GroupID:        67890,
				Status:         "inactive",
				GlobalOwners:   []string{"shared@example.com"},
				Prefix:         "shared-prefix",
				ProjectSlug:    "shared-project",
				ProjectName:    "Shared Project",
				ProjectUID:     "project-789",
				URL:            "https://shared.groups.io/g/shared",
				GroupName:      "shared-group",
				Public:         false,
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				LastReviewedAt: stringPtr("2023-01-01T15:00:00Z"),
				LastReviewedBy: stringPtr("reviewer-456"),
				Writers:        []string{"writer3", "writer4"},
				Auditors:       []string{"auditor3"},
			},
			expected: &mailinglistservice.ServiceWithReadonlyAttributes{
				UID:            stringPtr("service-123"),
				Type:           "shared",
				Domain:         stringPtr("shared.groups.io"),
				GroupID:        int64Ptr(67890),
				Status:         stringPtr("inactive"),
				GlobalOwners:   []string{"shared@example.com"},
				Prefix:         stringPtr("shared-prefix"),
				ProjectSlug:    stringPtr("shared-project"),
				ProjectName:    stringPtr("Shared Project"),
				ProjectUID:     "project-789",
				URL:            stringPtr("https://shared.groups.io/g/shared"),
				GroupName:      stringPtr("shared-group"),
				Public:         false,
				CreatedAt:      stringPtr("2023-01-01T12:00:00Z"),
				UpdatedAt:      stringPtr("2023-01-02T12:00:00Z"),
				LastReviewedAt: stringPtr("2023-01-01T15:00:00Z"),
				LastReviewedBy: stringPtr("reviewer-456"),
				Writers:        []string{"writer3", "writer4"},
				Auditors:       []string{"auditor3"},
			},
		},
		{
			name: "domain with zero timestamps",
			domain: &model.GrpsIOService{
				UID:        "service-456",
				Type:       "formation",
				ProjectUID: "project-456",
				Public:     true,
				CreatedAt:  time.Time{}, // Zero timestamp
				UpdatedAt:  time.Time{}, // Zero timestamp
			},
			expected: &mailinglistservice.ServiceWithReadonlyAttributes{
				UID:          stringPtr("service-456"),
				Type:         "formation",
				Domain:       stringPtr(""),
				GroupID:      int64Ptr(0),
				Status:       stringPtr(""),
				GlobalOwners: nil,
				Prefix:       stringPtr(""),
				ProjectSlug:  stringPtr(""),
				ProjectName:  stringPtr(""),
				ProjectUID:   "project-456",
				URL:          stringPtr(""),
				GroupName:    stringPtr(""),
				Public:       true,
				// CreatedAt and UpdatedAt should be nil when timestamps are zero
				CreatedAt:      nil,
				UpdatedAt:      nil,
				LastReviewedAt: nil,
				LastReviewedBy: nil,
				Writers:        nil,
				Auditors:       nil,
			},
		},
		{
			name:     "nil domain",
			domain:   nil,
			expected: &mailinglistservice.ServiceWithReadonlyAttributes{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertDomainToStandardResponse(tt.domain)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMailingListDomainToResponse(t *testing.T) {
	createdAt := time.Date(2023, 2, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2023, 2, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		domain   *model.GrpsIOMailingList
		expected *mailinglistservice.MailingListFull
	}{
		{
			name: "complete mailing list domain to response conversion",
			domain: &model.GrpsIOMailingList{
				UID:              "ml-123",
				GroupName:        "test-mailing-list",
				Public:           true,
				Type:             "discussion_open",
				CommitteeUID:     "committee-123",
				CommitteeName:    "Test Committee",
				CommitteeFilters: []string{"Voting Rep", "Observer"},
				Description:      "This is a comprehensive test mailing list",
				Title:            "Test Mailing List",
				SubjectTag:       "[TEST]",
				ServiceUID:       "parent-service-456",
				ProjectUID:       "project-789",
				ProjectName:      "Test Project",
				ProjectSlug:      "test-project",
				LastReviewedAt:   stringPtr("2023-02-01T08:00:00Z"),
				LastReviewedBy:   stringPtr("reviewer-789"),
				Writers:          []string{"writer5", "writer6"},
				Auditors:         []string{"auditor4", "auditor5"},
				CreatedAt:        createdAt,
				UpdatedAt:        updatedAt,
			},
			expected: &mailinglistservice.MailingListFull{
				UID:              stringPtr("ml-123"),
				GroupName:        stringPtr("test-mailing-list"),
				Public:           true,
				Type:             stringPtr("discussion_open"),
				CommitteeUID:     stringPtr("committee-123"),
				CommitteeFilters: []string{"Voting Rep", "Observer"},
				Description:      stringPtr("This is a comprehensive test mailing list"),
				Title:            stringPtr("Test Mailing List"),
				SubjectTag:       stringPtr("[TEST]"),
				ServiceUID:       stringPtr("parent-service-456"),
				ProjectUID:       stringPtr("project-789"),
				ProjectName:      stringPtr("Test Project"),
				ProjectSlug:      stringPtr("test-project"),
				Writers:          []string{"writer5", "writer6"},
				Auditors:         []string{"auditor4", "auditor5"},
				CreatedAt:        stringPtr("2023-02-01T10:00:00Z"),
				UpdatedAt:        stringPtr("2023-02-02T10:00:00Z"),
				LastReviewedAt:   stringPtr("2023-02-01T08:00:00Z"),
				LastReviewedBy:   stringPtr("reviewer-789"),
			},
		},
		{
			name: "minimal mailing list domain to response conversion",
			domain: &model.GrpsIOMailingList{
				UID:         "ml-456",
				GroupName:   "minimal-list",
				Public:      false,
				Type:        "announcement",
				Description: "Minimal mailing list",
				Title:       "Minimal List",
				ServiceUID:  "parent-789",
				CreatedAt:   time.Time{}, // Zero timestamp
				UpdatedAt:   time.Time{}, // Zero timestamp
			},
			expected: &mailinglistservice.MailingListFull{
				UID:              stringPtr("ml-456"),
				GroupName:        stringPtr("minimal-list"),
				Public:           false,
				Type:             stringPtr("announcement"),
				CommitteeUID:     nil, // Empty string converts to nil
				CommitteeFilters: nil,
				Description:      stringPtr("Minimal mailing list"),
				Title:            stringPtr("Minimal List"),
				SubjectTag:       nil, // Empty string converts to nil
				ServiceUID:       stringPtr("parent-789"),
				ProjectUID:       stringPtr(""),
				ProjectName:      stringPtr(""),
				ProjectSlug:      stringPtr(""),
				Writers:          nil,
				Auditors:         nil,
				// CreatedAt and UpdatedAt should be nil when timestamps are zero
				CreatedAt:      nil,
				UpdatedAt:      nil,
				LastReviewedAt: nil,
				LastReviewedBy: nil,
			},
		},
		{
			name:     "nil mailing list domain",
			domain:   nil,
			expected: &mailinglistservice.MailingListFull{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertMailingListDomainToResponse(tt.domain)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringToPointer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *string
	}{
		{
			name:     "non-empty string",
			input:    "test-string",
			expected: stringPtr("test-string"),
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringToPointer(tt.input)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}
