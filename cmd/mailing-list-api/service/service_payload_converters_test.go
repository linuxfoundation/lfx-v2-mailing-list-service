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

func TestConvertCreatePayloadToDomain(t *testing.T) {
	tests := []struct {
		name     string
		payload  *mailinglistservice.CreateGrpsioServicePayload
		expected *model.GrpsIOService
	}{
		{
			name: "complete payload conversion",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "primary",
				Domain:       stringPtr("example.groups.io"),
				GroupID:      int64Ptr(12345),
				Status:       stringPtr("active"),
				GlobalOwners: []string{"owner1@example.com", "owner2@example.com"},
				Prefix:       stringPtr("test-prefix"),
				ProjectSlug:  stringPtr("test-project"),
				ProjectUID:   "project-123",
				URL:          stringPtr("https://example.groups.io/g/test-group"),
				GroupName:    stringPtr("test-group"),
				Public:       true,
				Writers:      []string{"writer1", "writer2"},
				Auditors:     []string{"auditor1", "auditor2"},
			},
			expected: &model.GrpsIOService{
				Type:         "primary",
				Domain:       "example.groups.io",
				GroupID:      12345,
				Status:       "active",
				GlobalOwners: []string{"owner1@example.com", "owner2@example.com"},
				Prefix:       "test-prefix",
				ProjectSlug:  "test-project",
				ProjectUID:   "project-123",
				URL:          "https://example.groups.io/g/test-group",
				GroupName:    "test-group",
				Public:       true,
				Writers:      []string{"writer1", "writer2"},
				Auditors:     []string{"auditor1", "auditor2"},
			},
		},
		{
			name: "minimal payload conversion",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:       "formation",
				ProjectUID: "project-456",
				Public:     false,
			},
			expected: &model.GrpsIOService{
				Type:         "formation",
				Domain:       "",
				GroupID:      0,
				Status:       "",
				GlobalOwners: nil,
				Prefix:       "",
				ProjectSlug:  "",
				ProjectUID:   "project-456",
				URL:          "",
				GroupName:    "",
				Public:       false,
				Writers:      nil,
				Auditors:     nil,
			},
		},
		{
			name:     "nil payload",
			payload:  nil,
			expected: &model.GrpsIOService{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertCreatePayloadToDomain(tt.payload)

			// Check all fields except timestamps
			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.Domain, result.Domain)
			assert.Equal(t, tt.expected.GroupID, result.GroupID)
			assert.Equal(t, tt.expected.Status, result.Status)
			assert.Equal(t, tt.expected.GlobalOwners, result.GlobalOwners)
			assert.Equal(t, tt.expected.Prefix, result.Prefix)
			assert.Equal(t, tt.expected.ProjectSlug, result.ProjectSlug)
			assert.Equal(t, tt.expected.ProjectUID, result.ProjectUID)
			assert.Equal(t, tt.expected.URL, result.URL)
			assert.Equal(t, tt.expected.GroupName, result.GroupName)
			assert.Equal(t, tt.expected.Public, result.Public)
			assert.Equal(t, tt.expected.Writers, result.Writers)
			assert.Equal(t, tt.expected.Auditors, result.Auditors)

			// Verify timestamps are set for non-nil payloads
			if tt.payload != nil {
				assert.False(t, result.CreatedAt.IsZero())
				assert.False(t, result.UpdatedAt.IsZero())
			}
		})
	}
}

func TestConvertMailingListPayloadToDomain(t *testing.T) {
	tests := []struct {
		name     string
		payload  *mailinglistservice.CreateGrpsioMailingListPayload
		expected *model.GrpsIOMailingList
	}{
		{
			name: "complete mailing list payload conversion",
			payload: &mailinglistservice.CreateGrpsioMailingListPayload{
				GroupName:        "test-mailing-list",
				Public:           true,
				Type:             "discussion_open",
				CommitteeUID:     stringPtr("committee-123"),
				CommitteeFilters: []string{"Voting Rep", "Observer"},
				Description:      "This is a test mailing list description",
				Title:            "Test Mailing List",
				SubjectTag:       stringPtr("[TEST]"),
				ServiceUID:       "parent-service-456",
				Writers:          []string{"writer1", "writer2"},
				Auditors:         []string{"auditor1", "auditor2"},
			},
			expected: &model.GrpsIOMailingList{
				GroupName:        "test-mailing-list",
				Public:           true,
				Type:             "discussion_open",
				CommitteeUID:     "committee-123",
				CommitteeFilters: []string{"Voting Rep", "Observer"},
				Description:      "This is a test mailing list description",
				Title:            "Test Mailing List",
				SubjectTag:       "[TEST]",
				ServiceUID:       "parent-service-456",
				Writers:          []string{"writer1", "writer2"},
				Auditors:         []string{"auditor1", "auditor2"},
			},
		},
		{
			name: "minimal mailing list payload conversion",
			payload: &mailinglistservice.CreateGrpsioMailingListPayload{
				GroupName:   "minimal-list",
				Public:      false,
				Type:        "announcement",
				Description: "Minimal description for testing",
				Title:       "Minimal List",
				ServiceUID:  "parent-789",
			},
			expected: &model.GrpsIOMailingList{
				GroupName:        "minimal-list",
				Public:           false,
				Type:             "announcement",
				CommitteeUID:     "",
				CommitteeFilters: nil,
				Description:      "Minimal description for testing",
				Title:            "Minimal List",
				SubjectTag:       "",
				ServiceUID:       "parent-789",
				Writers:          nil,
				Auditors:         nil,
			},
		},
		{
			name:     "nil mailing list payload",
			payload:  nil,
			expected: &model.GrpsIOMailingList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertMailingListPayloadToDomain(tt.payload)

			// Check all fields except timestamps
			assert.Equal(t, tt.expected.GroupName, result.GroupName)
			assert.Equal(t, tt.expected.Public, result.Public)
			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.CommitteeUID, result.CommitteeUID)
			assert.Equal(t, tt.expected.CommitteeFilters, result.CommitteeFilters)
			assert.Equal(t, tt.expected.Description, result.Description)
			assert.Equal(t, tt.expected.Title, result.Title)
			assert.Equal(t, tt.expected.SubjectTag, result.SubjectTag)
			assert.Equal(t, tt.expected.ServiceUID, result.ServiceUID)
			assert.Equal(t, tt.expected.Writers, result.Writers)
			assert.Equal(t, tt.expected.Auditors, result.Auditors)

			// Verify timestamps are set for non-nil payloads
			if tt.payload != nil {
				assert.False(t, result.CreatedAt.IsZero())
				assert.False(t, result.UpdatedAt.IsZero())
			}
		})
	}
}

func TestConvertUpdatePayloadToDomain(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		existing *model.GrpsIOService
		payload  *mailinglistservice.UpdateGrpsioServicePayload
		expected *model.GrpsIOService
	}{
		{
			name: "complete update payload conversion",
			existing: &model.GrpsIOService{
				Type:           "primary",
				UID:            "service-123",
				Domain:         "example.groups.io",
				GroupID:        12345,
				Prefix:         "",
				ProjectSlug:    "test-project",
				ProjectName:    "Test Project",
				ProjectUID:     "project-123",
				URL:            "https://example.groups.io/g/test",
				GroupName:      "test-group",
				CreatedAt:      baseTime,
				LastReviewedAt: stringPtr("2023-01-01T10:00:00Z"),
				LastReviewedBy: stringPtr("reviewer-123"),
			},
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				UID:          stringPtr("service-123"),
				Status:       stringPtr("inactive"),
				GlobalOwners: []string{"newowner@example.com"},
				Public:       true,
				Writers:      []string{"writer1", "writer2"},
				Auditors:     []string{"auditor1"},
			},
			expected: &model.GrpsIOService{
				Type:           "primary",
				UID:            "service-123",
				Domain:         "example.groups.io",
				GroupID:        12345,
				Status:         "inactive",
				GlobalOwners:   []string{"newowner@example.com"},
				Prefix:         "",
				ProjectSlug:    "test-project",
				ProjectName:    "Test Project",
				ProjectUID:     "project-123",
				URL:            "https://example.groups.io/g/test",
				GroupName:      "test-group",
				Public:         true,
				CreatedAt:      baseTime,
				LastReviewedAt: stringPtr("2023-01-01T10:00:00Z"),
				LastReviewedBy: stringPtr("reviewer-123"),
				Writers:        []string{"writer1", "writer2"},
				Auditors:       []string{"auditor1"},
			},
		},
		{
			name: "minimal update payload conversion",
			existing: &model.GrpsIOService{
				Type:        "formation",
				UID:         "service-456",
				Domain:      "test.groups.io",
				GroupID:     67890,
				ProjectUID:  "project-456",
				ProjectSlug: "test-formation",
				CreatedAt:   baseTime,
			},
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				UID:    stringPtr("service-456"),
				Public: false,
			},
			expected: &model.GrpsIOService{
				Type:        "formation",
				UID:         "service-456",
				Domain:      "test.groups.io",
				GroupID:     67890,
				Status:      "",
				ProjectUID:  "project-456",
				ProjectSlug: "test-formation",
				Public:      false,
				CreatedAt:   baseTime,
			},
		},
		{
			name:     "nil payload",
			existing: nil,
			payload:  nil,
			expected: &model.GrpsIOService{},
		},
		{
			name:     "nil UID in payload",
			existing: &model.GrpsIOService{UID: "test-123"},
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				UID: nil,
			},
			expected: &model.GrpsIOService{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertUpdatePayloadToDomain(tt.existing, tt.payload)

			// Check all fields except UpdatedAt timestamp
			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.UID, result.UID)
			assert.Equal(t, tt.expected.Domain, result.Domain)
			assert.Equal(t, tt.expected.GroupID, result.GroupID)
			assert.Equal(t, tt.expected.Status, result.Status)
			assert.Equal(t, tt.expected.GlobalOwners, result.GlobalOwners)
			assert.Equal(t, tt.expected.Prefix, result.Prefix)
			assert.Equal(t, tt.expected.ProjectSlug, result.ProjectSlug)
			assert.Equal(t, tt.expected.ProjectName, result.ProjectName)
			assert.Equal(t, tt.expected.ProjectUID, result.ProjectUID)
			assert.Equal(t, tt.expected.URL, result.URL)
			assert.Equal(t, tt.expected.GroupName, result.GroupName)
			assert.Equal(t, tt.expected.Public, result.Public)
			assert.Equal(t, tt.expected.CreatedAt, result.CreatedAt)
			assert.Equal(t, tt.expected.LastReviewedAt, result.LastReviewedAt)
			assert.Equal(t, tt.expected.LastReviewedBy, result.LastReviewedBy)
			assert.Equal(t, tt.expected.Writers, result.Writers)
			assert.Equal(t, tt.expected.Auditors, result.Auditors)

			// Verify UpdatedAt is set for valid payloads
			if tt.payload != nil && tt.payload.UID != nil && tt.existing != nil {
				assert.False(t, result.UpdatedAt.IsZero())
			}
		})
	}
}

// Helper functions for creating pointers to primitives
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
