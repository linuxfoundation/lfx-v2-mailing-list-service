// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type ServiceConvertersSuite struct {
	suite.Suite
}

func TestServiceConverters(t *testing.T) {
	suite.Run(t, new(ServiceConvertersSuite))
}

func (s *ServiceConvertersSuite) TestConvertMember() {
	nonZeroTime := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		input           *model.GrpsIOMember
		expectNil       bool
		expectEmail     string
		expectName      string
		expectCreatedAt *string
		expectUpdatedAt *string
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name: "zero timestamps produce nil CreatedAt and UpdatedAt",
			input: &model.GrpsIOMember{
				Email:     "alice@example.com",
				CreatedAt: time.Time{},
				UpdatedAt: time.Time{},
			},
			expectEmail:     "alice@example.com",
			expectCreatedAt: nil,
			expectUpdatedAt: nil,
		},
		{
			name: "non-zero timestamps are formatted as RFC3339",
			input: &model.GrpsIOMember{
				CreatedAt: nonZeroTime,
				UpdatedAt: nonZeroTime,
			},
			expectCreatedAt: ptr("2024-03-15T10:00:00Z"),
			expectUpdatedAt: ptr("2024-03-15T10:00:00Z"),
		},
		{
			name: "fields map correctly",
			input: &model.GrpsIOMember{
				UID:            "m-1",
				Email:          "alice@example.com",
				GroupsFullName: "Alice Smith",
				MemberType:     "committee",
				DeliveryMode:   "single",
				ModStatus:      "none",
				Status:         "normal",
				Organization:   "Acme",
				JobTitle:       "Engineer",
				Username:       "alice",
				Role:           "member",
				VotingStatus:   "approved",
			},
			expectEmail: "alice@example.com",
			expectName:  "Alice Smith",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := convertMember(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectEmail, ptrVal(got.Email))
			s.Equal(tt.expectName, ptrVal(got.Name))
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)
		})
	}
}

func (s *ServiceConvertersSuite) TestConvertMailingList() {
	nonZeroTime := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name                string
		input               *model.GroupsIOMailingList
		expectNil           bool
		expectCommitteeUID  *string
		expectCreatedAt     *string
		expectUpdatedAt     *string
		expectName          string
		expectProjectUID    string
		expectProjectUIDNil bool
		expectServiceID     string
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name: "no committees produces nil CommitteeUID",
			input: &model.GroupsIOMailingList{
				Committees: nil,
			},
			expectCommitteeUID: nil,
		},
		{
			name: "first committee UID is extracted",
			input: &model.GroupsIOMailingList{
				Committees: []model.Committee{{UID: "c-1"}, {UID: "c-2"}},
			},
			expectCommitteeUID: ptr("c-1"),
		},
		{
			name: "zero timestamps produce nil CreatedAt and UpdatedAt",
			input: &model.GroupsIOMailingList{
				CreatedAt: time.Time{},
				UpdatedAt: time.Time{},
			},
			expectCreatedAt: nil,
			expectUpdatedAt: nil,
		},
		{
			name: "non-zero timestamps are formatted as RFC3339",
			input: &model.GroupsIOMailingList{
				CreatedAt: nonZeroTime,
				UpdatedAt: nonZeroTime,
			},
			expectCreatedAt: ptr("2024-03-15T10:00:00Z"),
			expectUpdatedAt: ptr("2024-03-15T10:00:00Z"),
		},
		{
			name: "fields map correctly",
			input: &model.GroupsIOMailingList{
				UID:        "ml-1",
				GroupName:  "My List",
				ProjectUID: "proj-1",
				ServiceUID: "svc-1",
			},
			expectName:       "My List",
			expectProjectUID: "proj-1",
			expectServiceID:  "svc-1",
		},
		{
			name:                "empty ProjectUID produces nil project_uid field",
			input:               &model.GroupsIOMailingList{ProjectUID: ""},
			expectProjectUIDNil: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := convertMailingList(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectCommitteeUID, got.CommitteeUID)
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)
			if tt.expectName != "" {
				s.Equal(tt.expectName, ptrVal(got.Name))
			}
			if tt.expectProjectUIDNil {
				s.Nil(got.ProjectUID)
			} else if tt.expectProjectUID != "" {
				s.Equal(tt.expectProjectUID, ptrVal(got.ProjectUID))
			}
			if tt.expectServiceID != "" {
				s.Equal(tt.expectServiceID, ptrVal(got.ServiceID))
			}
		})
	}
}

func (s *ServiceConvertersSuite) TestConvertService() {
	nonZeroTime := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		input           *model.GroupsIOService
		expectNil       bool
		expectCreatedAt *string
		expectUpdatedAt *string
		expectUID       string
		expectProject   string
		expectType      string
		expectDomain    string
		expectPrefix    string
		expectStatus    string
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name: "zero timestamps produce nil CreatedAt and UpdatedAt",
			input: &model.GroupsIOService{
				CreatedAt: time.Time{},
				UpdatedAt: time.Time{},
			},
			expectCreatedAt: nil,
			expectUpdatedAt: nil,
		},
		{
			name: "non-zero timestamps are formatted as RFC3339",
			input: &model.GroupsIOService{
				CreatedAt: nonZeroTime,
				UpdatedAt: nonZeroTime,
			},
			expectCreatedAt: ptr("2024-03-15T10:00:00Z"),
			expectUpdatedAt: ptr("2024-03-15T10:00:00Z"),
		},
		{
			name: "fields map correctly",
			input: &model.GroupsIOService{
				UID:        "svc-1",
				ProjectUID: "proj-1",
				Type:       "v2_primary",
				Domain:     "groups.io",
				Prefix:     "linux",
				Status:     "active",
			},
			expectUID:     "svc-1",
			expectProject: "proj-1",
			expectType:    "v2_primary",
			expectDomain:  "groups.io",
			expectPrefix:  "linux",
			expectStatus:  "active",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := convertService(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)
			if tt.expectUID != "" {
				s.Equal(tt.expectUID, ptrVal(got.ID))
			}
			if tt.expectProject != "" {
				s.Equal(tt.expectProject, ptrVal(got.ProjectUID))
			}
			if tt.expectType != "" {
				s.Equal(tt.expectType, ptrVal(got.Type))
			}
			if tt.expectDomain != "" {
				s.Equal(tt.expectDomain, ptrVal(got.Domain))
			}
			if tt.expectPrefix != "" {
				s.Equal(tt.expectPrefix, ptrVal(got.Prefix))
			}
			if tt.expectStatus != "" {
				s.Equal(tt.expectStatus, ptrVal(got.Status))
			}
		})
	}
}

func (s *ServiceConvertersSuite) TestConvertArtifactUser() {
	tests := []struct {
		name      string
		input     *model.ArtifactUser
		expectNil bool
		expectID  string
		expectAll bool
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name: "fields map correctly",
			input: &model.ArtifactUser{
				ID:             "u-1",
				Username:       "alice",
				Name:           "Alice Smith",
				Email:          "alice@example.com",
				ProfilePicture: "https://example.com/pic.png",
			},
			expectAll: true,
		},
		{
			name:     "empty fields produce nil pointers via NonEmptyString",
			input:    &model.ArtifactUser{},
			expectID: "",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := convertArtifactUser(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			if tt.expectAll {
				s.Equal("u-1", ptrVal(got.ID))
				s.Equal("alice", ptrVal(got.Username))
				s.Equal("Alice Smith", ptrVal(got.Name))
				s.Equal("alice@example.com", ptrVal(got.Email))
				s.Equal("https://example.com/pic.png", ptrVal(got.ProfilePicture))
			} else {
				s.Nil(got.ID)
			}
		})
	}
}

func (s *ServiceConvertersSuite) TestConvertArtifact() {
	nonZeroTime := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	fileUploadedTrue := true
	lastMsgID := uint64(42)
	groupID := uint64(99)

	tests := []struct {
		name                  string
		input                 *model.GroupsIOArtifact
		expectNil             bool
		expectCreatedAt       *string
		expectUpdatedAt       *string
		expectFileUploadedAt  *string
		expectLastPostedAt    *string
		expectFileUploaded    *bool
		expectLastPostedMsgID *uint64
		expectArtifactID      string
		expectGroupID         *uint64
		expectCreatedBy       bool
		expectLastModifiedBy  bool
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name: "zero created_at and updated_at produce nil pointers",
			input: &model.GroupsIOArtifact{
				CreatedAt: time.Time{},
				UpdatedAt: time.Time{},
			},
			expectCreatedAt: nil,
			expectUpdatedAt: nil,
		},
		{
			name: "non-zero created_at and updated_at are formatted as RFC3339",
			input: &model.GroupsIOArtifact{
				CreatedAt: nonZeroTime,
				UpdatedAt: nonZeroTime,
			},
			expectCreatedAt: ptr("2024-03-15T10:00:00Z"),
			expectUpdatedAt: ptr("2024-03-15T10:00:00Z"),
		},
		{
			name: "nil file_uploaded_at produces nil",
			input: &model.GroupsIOArtifact{
				FileUploadedAt: nil,
			},
			expectFileUploadedAt: nil,
		},
		{
			name: "non-zero file_uploaded_at is formatted as RFC3339",
			input: &model.GroupsIOArtifact{
				FileUploadedAt: &nonZeroTime,
			},
			expectFileUploadedAt: ptr("2024-03-15T10:00:00Z"),
		},
		{
			name: "nil last_posted_at produces nil",
			input: &model.GroupsIOArtifact{
				LastPostedAt: nil,
			},
			expectLastPostedAt: nil,
		},
		{
			name: "non-zero last_posted_at is formatted as RFC3339",
			input: &model.GroupsIOArtifact{
				LastPostedAt: &nonZeroTime,
			},
			expectLastPostedAt: ptr("2024-03-15T10:00:00Z"),
		},
		{
			name: "file_uploaded is nil when type is link",
			input: &model.GroupsIOArtifact{
				Type:         "link",
				FileUploaded: &fileUploadedTrue,
			},
			expectFileUploaded: nil,
		},
		{
			name: "file_uploaded is passed through when type is file",
			input: &model.GroupsIOArtifact{
				Type:         "file",
				FileUploaded: &fileUploadedTrue,
			},
			expectFileUploaded: &fileUploadedTrue,
		},
		{
			name: "file_uploaded is nil when type is file but FileUploaded is nil",
			input: &model.GroupsIOArtifact{
				Type:         "file",
				FileUploaded: nil,
			},
			expectFileUploaded: nil,
		},
		{
			name: "nil last_posted_message_id produces nil",
			input: &model.GroupsIOArtifact{
				LastPostedMessageID: nil,
			},
			expectLastPostedMsgID: nil,
		},
		{
			name: "non-nil last_posted_message_id is passed through",
			input: &model.GroupsIOArtifact{
				LastPostedMessageID: &lastMsgID,
			},
			expectLastPostedMsgID: &lastMsgID,
		},
		{
			name: "scalar fields map correctly",
			input: &model.GroupsIOArtifact{
				ArtifactID: "art-1",
				GroupID:    groupID,
				CreatedAt:  nonZeroTime,
				UpdatedAt:  nonZeroTime,
			},
			expectArtifactID: "art-1",
			expectGroupID:    &groupID,
			expectCreatedAt:  ptr("2024-03-15T10:00:00Z"),
			expectUpdatedAt:  ptr("2024-03-15T10:00:00Z"),
		},
		{
			name: "created_by and last_modified_by are converted",
			input: &model.GroupsIOArtifact{
				CreatedBy:      &model.ArtifactUser{ID: "u-1"},
				LastModifiedBy: &model.ArtifactUser{ID: "u-2"},
			},
			expectCreatedBy:      true,
			expectLastModifiedBy: true,
		},
		{
			name: "nil created_by and last_modified_by produce nil",
			input: &model.GroupsIOArtifact{
				CreatedBy:      nil,
				LastModifiedBy: nil,
			},
			expectCreatedBy:      false,
			expectLastModifiedBy: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := convertArtifact(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)
			s.Equal(tt.expectFileUploadedAt, got.FileUploadedAt)
			s.Equal(tt.expectLastPostedAt, got.LastPostedAt)
			s.Equal(tt.expectFileUploaded, got.FileUploaded)
			s.Equal(tt.expectLastPostedMsgID, got.LastPostedMessageID)
			if tt.expectArtifactID != "" {
				s.Equal(tt.expectArtifactID, ptrVal(got.ArtifactID))
			}
			if tt.expectGroupID != nil {
				s.Equal(*tt.expectGroupID, *got.GroupID)
			}
			if tt.expectCreatedBy {
				s.NotNil(got.CreatedBy)
			} else if !tt.expectCreatedBy && tt.input != nil && tt.input.CreatedBy == nil {
				s.Nil(got.CreatedBy)
			}
			if tt.expectLastModifiedBy {
				s.NotNil(got.LastModifiedBy)
			} else if !tt.expectLastModifiedBy && tt.input != nil && tt.input.LastModifiedBy == nil {
				s.Nil(got.LastModifiedBy)
			}
		})
	}
}

// ptr is a helper to get a pointer to a string literal.
func ptr(s string) *string { return &s }

// ptrVal safely dereferences a *string, returning "" if nil.
func ptrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
