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
				GroupID:      int64Ptr(12345),
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
				GroupID:      int64Ptr(0),
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
			result := svc.convertGrpsIOServiceCreatePayloadToDomain(tt.payload)

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
			result := svc.convertGrpsIOMailingListPayloadToDomain(tt.payload)

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
				GroupID:        int64Ptr(12345),
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
				GroupID:        int64Ptr(12345),
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
				GroupID:     int64Ptr(67890),
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
				GroupID:     int64Ptr(67890),
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
			result := svc.convertGrpsIOServiceUpdatePayloadToDomain(tt.existing, tt.payload)

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

func TestConvertMemberUpdatePayloadToDomain(t *testing.T) {
	now := time.Now().UTC()
	existingMember := &model.GrpsIOMember{
		UID:            "member-123",
		MailingListUID: "ml-456",
		Email:          "existing@example.com",
		Username:       "existinguser",
		FirstName:      "Existing",
		LastName:       "User",
		Organization:   "Existing Corp",
		JobTitle:       "Existing Engineer",
		DeliveryMode:   "digest",     // Existing preference
		ModStatus:      "moderator",  // Existing permission
		MemberType:     "direct",
		Status:         "active",
		CreatedAt:      now.Add(-24 * time.Hour),
		UpdatedAt:      now.Add(-1 * time.Hour),
	}

	tests := []struct {
		name     string
		existing *model.GrpsIOMember
		payload  *mailinglistservice.UpdateGrpsioMailingListMemberPayload
		expected *model.GrpsIOMember
	}{
		{
			name:     "partial update - only name fields, clear other mutable fields (PUT semantics)",
			existing: existingMember,
			payload: &mailinglistservice.UpdateGrpsioMailingListMemberPayload{
				FirstName: stringPtr("Updated"),
				LastName:  stringPtr("Name"),
				// DeliveryMode and ModStatus are nil - PUT semantics clears them to ""
			},
			expected: &model.GrpsIOMember{
				UID:            "member-123",
				MailingListUID: "ml-456",
				Email:          "existing@example.com",
				Username:       "",            // CLEARED (PUT semantics)
				FirstName:      "Updated",     // Updated
				LastName:       "Name",        // Updated
				Organization:   "",            // CLEARED (PUT semantics)
				JobTitle:       "",            // CLEARED (PUT semantics)
				DeliveryMode:   "",            // CLEARED (PUT semantics)
				ModStatus:      "",            // CLEARED (PUT semantics)
				MemberType:     "direct",      // IMMUTABLE
				Status:         "active",      // IMMUTABLE
				CreatedAt:      existingMember.CreatedAt,
			},
		},
		{
			name:     "partial update - only delivery mode, clear other mutable fields (PUT semantics)",
			existing: existingMember,
			payload: &mailinglistservice.UpdateGrpsioMailingListMemberPayload{
				DeliveryMode: "normal",
				// All other fields nil - PUT semantics clears them to ""
			},
			expected: &model.GrpsIOMember{
				UID:            "member-123",
				MailingListUID: "ml-456",
				Email:          "existing@example.com",
				Username:       "",                   // CLEARED (PUT semantics)
				FirstName:      "",                   // CLEARED (PUT semantics)
				LastName:       "",                   // CLEARED (PUT semantics)
				Organization:   "",                   // CLEARED (PUT semantics)
				JobTitle:       "",                   // CLEARED (PUT semantics)
				DeliveryMode:   "normal",             // Updated
				ModStatus:      "",                   // CLEARED (PUT semantics)
				MemberType:     "direct",             // IMMUTABLE
				Status:         "active",             // IMMUTABLE
				CreatedAt:      existingMember.CreatedAt,
			},
		},
		{
			name:     "complete update - all fields provided",
			existing: existingMember,
			payload: &mailinglistservice.UpdateGrpsioMailingListMemberPayload{
				Username:     stringPtr("newuser"),
				FirstName:    stringPtr("New"),
				LastName:     stringPtr("Person"),
				Organization: stringPtr("New Corp"),
				JobTitle:     stringPtr("New Role"),
				DeliveryMode: "none",
				ModStatus:    "owner",
			},
			expected: &model.GrpsIOMember{
				UID:            "member-123",
				MailingListUID: "ml-456",
				Email:          "existing@example.com",  // Immutable
				Username:       "newuser",              // Updated
				FirstName:      "New",                  // Updated
				LastName:       "Person",               // Updated
				Organization:   "New Corp",             // Updated
				JobTitle:       "New Role",             // Updated
				DeliveryMode:   "none",                 // Updated
				ModStatus:      "owner",                // Updated
				MemberType:     "direct",               // Immutable
				Status:         "active",               // Immutable
				CreatedAt:      existingMember.CreatedAt, // Immutable
			},
		},
		{
			name:     "empty update - no fields provided, all mutable fields cleared (PUT semantics)",
			existing: existingMember,
			payload: &mailinglistservice.UpdateGrpsioMailingListMemberPayload{
				// All fields nil - PUT semantics clears all mutable fields
			},
			expected: &model.GrpsIOMember{
				UID:            "member-123",
				MailingListUID: "ml-456",
				Email:          "existing@example.com",
				Username:       "",           // CLEARED (PUT semantics)
				FirstName:      "",           // CLEARED (PUT semantics)
				LastName:       "",           // CLEARED (PUT semantics)
				Organization:   "",           // CLEARED (PUT semantics)
				JobTitle:       "",           // CLEARED (PUT semantics)
				DeliveryMode:   "",           // CLEARED (PUT semantics)
				ModStatus:      "",           // CLEARED (PUT semantics)
				MemberType:     "direct",     // IMMUTABLE
				Status:         "active",     // IMMUTABLE
				CreatedAt:      existingMember.CreatedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertGrpsIOMemberUpdatePayloadToDomain(tt.payload, tt.existing)

			// Check all fields except UpdatedAt timestamp
			assert.Equal(t, tt.expected.UID, result.UID)
			assert.Equal(t, tt.expected.MailingListUID, result.MailingListUID)
			assert.Equal(t, tt.expected.Email, result.Email)
			assert.Equal(t, tt.expected.Username, result.Username)
			assert.Equal(t, tt.expected.FirstName, result.FirstName)
			assert.Equal(t, tt.expected.LastName, result.LastName)
			assert.Equal(t, tt.expected.Organization, result.Organization)
			assert.Equal(t, tt.expected.JobTitle, result.JobTitle)
			assert.Equal(t, tt.expected.DeliveryMode, result.DeliveryMode, "DeliveryMode should follow PUT semantics (nil clears to empty)")
			assert.Equal(t, tt.expected.ModStatus, result.ModStatus, "ModStatus should follow PUT semantics (nil clears to empty)")
			assert.Equal(t, tt.expected.MemberType, result.MemberType)
			assert.Equal(t, tt.expected.Status, result.Status)
			assert.Equal(t, tt.expected.CreatedAt, result.CreatedAt)

			// Verify UpdatedAt is set to current time
			assert.False(t, result.UpdatedAt.IsZero(), "UpdatedAt should be set")
			assert.True(t, result.UpdatedAt.After(tt.existing.UpdatedAt), "UpdatedAt should be newer than existing")
		})
	}
}

func TestConvertServiceUpdatePayloadToDomain(t *testing.T) {
	now := time.Now().UTC()
	existingService := &model.GrpsIOService{
		UID:         "service-123",
		Type:        "primary",
		Domain:      "existing.domain.com",
		GroupID:     int64Ptr(12345),
		Status:      "active",
		Public:      true,  // Existing setting
		GlobalOwners: []string{"existing@example.com"},
		ProjectUID:  "project-123",
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now.Add(-1 * time.Hour),
	}

	tests := []struct {
		name     string
		existing *model.GrpsIOService
		payload  *mailinglistservice.UpdateGrpsioServicePayload
		expected *model.GrpsIOService
	}{
		{
			name:     "partial update - only status, clear other mutable fields (PUT semantics)",
			existing: existingService,
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				UID:    stringPtr("service-123"),
				Status: stringPtr("updated"),
				// Public is nil - PUT semantics clears it to false
			},
			expected: &model.GrpsIOService{
				UID:          "service-123",
				Type:         "primary",          // IMMUTABLE
				Domain:       "existing.domain.com", // IMMUTABLE
				GroupID:      int64Ptr(12345),              // IMMUTABLE
				Status:       "updated",          // Updated
				Public:       false,              // CLEARED (PUT semantics)
				GlobalOwners: nil,                // CLEARED (PUT semantics)
				ProjectUID:   "project-123",      // IMMUTABLE
				CreatedAt:    existingService.CreatedAt, // IMMUTABLE
			},
		},
		{
			name:     "partial update - only public field, clear other mutable fields (PUT semantics)",
			existing: existingService,
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				UID:    stringPtr("service-123"),
				Public: false,
				// All other fields nil - PUT semantics clears mutable fields
			},
			expected: &model.GrpsIOService{
				UID:          "service-123",
				Type:         "primary",          // IMMUTABLE
				Domain:       "existing.domain.com", // IMMUTABLE
				GroupID:      int64Ptr(12345),              // IMMUTABLE
				Status:       "",                 // CLEARED (PUT semantics)
				Public:       false,              // Updated
				GlobalOwners: nil,                // CLEARED (PUT semantics)
				ProjectUID:   "project-123",      // IMMUTABLE
				CreatedAt:    existingService.CreatedAt, // IMMUTABLE
			},
		},
		{
			name:     "complete update - all mutable fields provided",
			existing: existingService,
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				UID:          stringPtr("service-123"),
				Type:         "formation",
				Status:       stringPtr("disabled"),
				Public:       false,
				GlobalOwners: []string{"new1@example.com", "new2@example.com"},
				ProjectUID:   "new-project-456",
			},
			expected: &model.GrpsIOService{
				UID:          "service-123",
				Type:         "primary",          // IMMUTABLE (can't change service type)
				Domain:       "existing.domain.com", // IMMUTABLE
				GroupID:      int64Ptr(12345),              // IMMUTABLE
				Status:       "disabled",         // Updated
				Public:       false,              // Updated
				GlobalOwners: []string{"new1@example.com", "new2@example.com"}, // Updated
				ProjectUID:   "project-123",      // IMMUTABLE (can't change project)
				CreatedAt:    existingService.CreatedAt, // IMMUTABLE
			},
		},
		{
			name:     "empty update - no fields provided, all mutable fields cleared (PUT semantics)",
			existing: existingService,
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				UID: stringPtr("service-123"),
				// All fields nil - PUT semantics clears all mutable fields
			},
			expected: &model.GrpsIOService{
				UID:          "service-123",
				Type:         "primary",          // IMMUTABLE
				Domain:       "existing.domain.com", // IMMUTABLE
				GroupID:      int64Ptr(12345),              // IMMUTABLE
				Status:       "",                 // CLEARED (PUT semantics)
				Public:       false,              // CLEARED to default false (PUT semantics)
				GlobalOwners: nil,                // CLEARED (PUT semantics)
				ProjectUID:   "project-123",      // IMMUTABLE
				CreatedAt:    existingService.CreatedAt, // IMMUTABLE
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertGrpsIOServiceUpdatePayloadToDomain(tt.existing, tt.payload)

			// Check all fields except UpdatedAt timestamp
			assert.Equal(t, tt.expected.UID, result.UID)
			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.Domain, result.Domain)
			assert.Equal(t, tt.expected.GroupID, result.GroupID)
			assert.Equal(t, tt.expected.Status, result.Status)
			assert.Equal(t, tt.expected.Public, result.Public, "Public field should follow PUT semantics (nil clears to false)")
			assert.Equal(t, tt.expected.GlobalOwners, result.GlobalOwners)
			assert.Equal(t, tt.expected.ProjectUID, result.ProjectUID)
			assert.Equal(t, tt.expected.CreatedAt, result.CreatedAt)

			// Verify UpdatedAt is set to current time
			assert.False(t, result.UpdatedAt.IsZero(), "UpdatedAt should be set")
			assert.True(t, result.UpdatedAt.After(tt.existing.UpdatedAt), "UpdatedAt should be newer than existing")
		})
	}
}

func TestConvertMailingListUpdatePayloadToDomain(t *testing.T) {
	now := time.Now().UTC()
	existingMailingList := &model.GrpsIOMailingList{
		UID:         "ml-123",
		GroupName:   "existing-group",
		Public:      false,  // Existing setting
		Type:        "discussion_moderated",
		Description: "Existing description for the group",
		Title:       "Existing Title",
		ServiceUID:  "service-123",
		ProjectUID:  "project-123",
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now.Add(-1 * time.Hour),
	}

	tests := []struct {
		name     string
		existing *model.GrpsIOMailingList
		payload  *mailinglistservice.UpdateGrpsioMailingListPayload
		expected *model.GrpsIOMailingList
	}{
		{
			name:     "partial update - only title, clear other mutable fields (PUT semantics)",
			existing: existingMailingList,
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				Title: "Updated Title",
				// All other fields nil - PUT semantics clears mutable fields
			},
			expected: &model.GrpsIOMailingList{
				UID:         "ml-123",            // IMMUTABLE
				GroupName:   "existing-group",    // IMMUTABLE
				Public:      false,               // CLEARED to default false
				Type:        "",                  // CLEARED (PUT semantics)
				Description: "",                  // CLEARED (PUT semantics)
				Title:       "Updated Title",     // Updated
				ServiceUID:  "",                  // CLEARED (PUT semantics)
				ProjectUID:  "project-123",       // IMMUTABLE
				CreatedAt:   existingMailingList.CreatedAt, // IMMUTABLE
			},
		},
		{
			name:     "partial update - only public field, clear other mutable fields (PUT semantics)",
			existing: existingMailingList,
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				Public: true,
				// All other fields nil - PUT semantics clears mutable fields
			},
			expected: &model.GrpsIOMailingList{
				UID:         "ml-123",            // IMMUTABLE
				GroupName:   "existing-group",    // IMMUTABLE
				Public:      true,                // Updated
				Type:        "",                  // CLEARED (PUT semantics)
				Description: "",                  // CLEARED (PUT semantics)
				Title:       "",                  // CLEARED (PUT semantics)
				ServiceUID:  "",                  // CLEARED (PUT semantics)
				ProjectUID:  "project-123",       // IMMUTABLE
				CreatedAt:   existingMailingList.CreatedAt, // IMMUTABLE
			},
		},
		{
			name:     "complete update - all fields provided",
			existing: existingMailingList,
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				GroupName:   "new-group",
				Public:      true,
				Type:        "discussion_open",
				Description: "New description that is long enough",
				Title:       "New Title",
				ServiceUID:  "new-service-456",
			},
			expected: &model.GrpsIOMailingList{
				UID:         "ml-123",            // IMMUTABLE
				GroupName:   "existing-group",    // IMMUTABLE (can't change group name)
				Public:      true,                // Updated
				Type:        "discussion_open",   // Updated
				Description: "New description that is long enough", // Updated
				Title:       "New Title",         // Updated
				ServiceUID:  "new-service-456",   // Updated
				ProjectUID:  "project-123",       // IMMUTABLE
				CreatedAt:   existingMailingList.CreatedAt, // PRESERVED (immutable)
			},
		},
		{
			name:     "empty update - no fields provided, all mutable fields cleared (PUT semantics)",
			existing: existingMailingList,
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				// All fields nil - PUT semantics clears all mutable fields
			},
			expected: &model.GrpsIOMailingList{
				UID:         "ml-123",            // IMMUTABLE
				GroupName:   "existing-group",    // IMMUTABLE
				Public:      false,               // CLEARED to default false
				Type:        "",                  // CLEARED (PUT semantics)
				Description: "",                  // CLEARED (PUT semantics)
				Title:       "",                  // CLEARED (PUT semantics)
				ServiceUID:  "",                  // CLEARED (PUT semantics)
				ProjectUID:  "project-123",       // IMMUTABLE
				CreatedAt:   existingMailingList.CreatedAt, // IMMUTABLE
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mailingListService{}
			result := svc.convertGrpsIOMailingListUpdatePayloadToDomain(tt.existing, tt.payload)

			// Check all fields except UpdatedAt timestamp
			assert.Equal(t, tt.expected.UID, result.UID)
			assert.Equal(t, tt.expected.GroupName, result.GroupName)
			assert.Equal(t, tt.expected.Public, result.Public, "Public field should follow PUT semantics (nil clears to false)")
			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.Description, result.Description)
			assert.Equal(t, tt.expected.Title, result.Title)
			assert.Equal(t, tt.expected.ServiceUID, result.ServiceUID)
			assert.Equal(t, tt.expected.ProjectUID, result.ProjectUID)
			assert.Equal(t, tt.expected.CreatedAt, result.CreatedAt)

			// Verify UpdatedAt is set to current time
			assert.False(t, result.UpdatedAt.IsZero(), "UpdatedAt should be set")
			assert.True(t, result.UpdatedAt.After(tt.existing.UpdatedAt), "UpdatedAt should be newer than existing")
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
