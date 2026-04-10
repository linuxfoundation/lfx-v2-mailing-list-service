// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type ProxyConvertersSuite struct {
	suite.Suite
}

func TestProxyConverters(t *testing.T) {
	suite.Run(t, new(ProxyConvertersSuite))
}

func (s *ProxyConvertersSuite) TestFromWireService() {
	groupID42 := int64(42)
	ts1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		input           *serviceWire
		expectNil       bool
		expectGroupID   *int64
		expectUID       string
		expectProject   string
		expectType      string
		expectDomain    string
		expectPrefix    string
		expectStatus    string
		expectCreatedAt time.Time
		expectUpdatedAt time.Time
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name:          "zero GroupID produces nil pointer",
			input:         &serviceWire{ID: "svc-1", GroupID: 0},
			expectUID:     "svc-1",
			expectGroupID: nil,
		},
		{
			name:          "non-zero GroupID populates pointer",
			input:         &serviceWire{ID: "svc-1", GroupID: 42},
			expectUID:     "svc-1",
			expectGroupID: &groupID42,
		},
		{
			name:            "valid RFC3339 timestamps are parsed",
			input:           &serviceWire{ID: "svc-1", CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-06-01T12:00:00Z"},
			expectUID:       "svc-1",
			expectCreatedAt: ts1,
			expectUpdatedAt: ts2,
		},
		{
			name:            "empty timestamps produce zero time",
			input:           &serviceWire{ID: "svc-1"},
			expectUID:       "svc-1",
			expectCreatedAt: time.Time{},
			expectUpdatedAt: time.Time{},
		},
		{
			name: "fields map correctly",
			input: &serviceWire{
				ID:        "svc-1",
				ProjectID: "proj-sfid",
				Type:      "v2_primary",
				GroupID:   42,
				Domain:    "groups.io",
				Prefix:    "linux",
				Status:    "active",
			},
			expectUID:     "svc-1",
			expectProject: "proj-sfid",
			expectType:    "v2_primary",
			expectGroupID: &groupID42,
			expectDomain:  "groups.io",
			expectPrefix:  "linux",
			expectStatus:  "active",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := fromWireService(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectUID, got.UID)
			s.Equal(tt.expectProject, got.ProjectUID)
			s.Equal(tt.expectType, got.Type)
			s.Equal(tt.expectDomain, got.Domain)
			s.Equal(tt.expectPrefix, got.Prefix)
			s.Equal(tt.expectStatus, got.Status)
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)
			if tt.expectGroupID == nil {
				s.Nil(got.GroupID)
			} else {
				s.Require().NotNil(got.GroupID)
				s.Equal(*tt.expectGroupID, *got.GroupID)
			}
		})
	}
}

func (s *ProxyConvertersSuite) TestToWireServiceRequest() {
	id42 := int64(42)

	tests := []struct {
		name            string
		input           *model.GroupsIOService
		expectGroupID   int64
		expectProjectID string
		expectType      string
		expectDomain    string
		expectPrefix    string
		expectStatus    string
	}{
		{
			name:          "nil GroupID pointer produces zero int",
			input:         &model.GroupsIOService{GroupID: nil},
			expectGroupID: 0,
		},
		{
			name: "fields map correctly",
			input: &model.GroupsIOService{
				ProjectUID: "proj-sfid",
				Type:       "v2_primary",
				GroupID:    &id42,
				Domain:     "groups.io",
				Prefix:     "linux",
				Status:     "active",
			},
			expectGroupID:   42,
			expectProjectID: "proj-sfid",
			expectType:      "v2_primary",
			expectDomain:    "groups.io",
			expectPrefix:    "linux",
			expectStatus:    "active",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := toWireServiceRequest(tt.input)
			s.Require().NotNil(got)
			s.Equal(tt.expectGroupID, got.GroupID)
			s.Equal(tt.expectProjectID, got.ProjectID)
			s.Equal(tt.expectType, got.Type)
			s.Equal(tt.expectDomain, got.Domain)
			s.Equal(tt.expectPrefix, got.Prefix)
			s.Equal(tt.expectStatus, got.Status)
		})
	}
}

func (s *ProxyConvertersSuite) TestFromWireSubgroup() {
	ts1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		input             *subgroupWire
		expectNil         bool
		expectUID         string
		expectProjectUID  string
		expectServiceUID  string
		expectGroupName   string
		expectDescription string
		expectType        string
		expectAccess      string
		expectCommittees  []model.Committee
		expectCreatedAt   time.Time
		expectUpdatedAt   time.Time
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name:      "UID is derived from numeric GroupID",
			input:     &subgroupWire{GroupID: 12345},
			expectUID: "12345",
		},
		{
			name:             "non-empty CommitteeID populates Committees slice",
			input:            &subgroupWire{GroupID: 1, CommitteeID: "committee-uuid"},
			expectUID:        "1",
			expectCommittees: []model.Committee{{UID: "committee-uuid"}},
		},
		{
			name:             "empty CommitteeID leaves Committees nil",
			input:            &subgroupWire{GroupID: 1, CommitteeID: ""},
			expectUID:        "1",
			expectCommittees: nil,
		},
		{
			name:            "valid RFC3339 timestamps are parsed",
			input:           &subgroupWire{GroupID: 1, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-06-01T12:00:00Z"},
			expectUID:       "1",
			expectCreatedAt: ts1,
			expectUpdatedAt: ts2,
		},
		{
			name:            "empty timestamps produce zero time",
			input:           &subgroupWire{GroupID: 1},
			expectUID:       "1",
			expectCreatedAt: time.Time{},
			expectUpdatedAt: time.Time{},
		},
		{
			name: "fields map correctly",
			input: &subgroupWire{
				GroupID:        42,
				ProjectID:      "proj-sfid",
				ParentID:       "svc-sfid",
				Name:           "My List",
				Description:    "A description",
				Type:           "announcement",
				AudienceAccess: "public",
			},
			expectUID:         "42",
			expectProjectUID:  "proj-sfid",
			expectServiceUID:  "svc-sfid",
			expectGroupName:   "My List",
			expectDescription: "A description",
			expectType:        "announcement",
			expectAccess:      "public",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := fromWireSubgroup(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectUID, got.UID)
			s.Equal(tt.expectProjectUID, got.ProjectUID)
			s.Equal(tt.expectServiceUID, got.ServiceUID)
			s.Equal(tt.expectGroupName, got.GroupName)
			s.Equal(tt.expectDescription, got.Description)
			s.Equal(tt.expectType, got.Type)
			s.Equal(tt.expectAccess, got.AudienceAccess)
			s.Equal(tt.expectCommittees, got.Committees)
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)
		})
	}
}

func (s *ProxyConvertersSuite) TestToWireSubgroupRequest() {
	tests := []struct {
		name              string
		input             *model.GroupsIOMailingList
		expectCommitteeID string
		expectProjectID   string
		expectParentID    string
		expectName        string
		expectDescription string
		expectType        string
		expectAccess      string
	}{
		{
			name:              "nil committees produces empty CommitteeID",
			input:             &model.GroupsIOMailingList{Committees: nil},
			expectCommitteeID: "",
		},
		{
			name:              "empty committees slice produces empty CommitteeID",
			input:             &model.GroupsIOMailingList{Committees: []model.Committee{}},
			expectCommitteeID: "",
		},
		{
			name: "only first committee UID is serialized",
			input: &model.GroupsIOMailingList{
				Committees: []model.Committee{{UID: "first"}, {UID: "second"}},
			},
			expectCommitteeID: "first",
		},
		{
			name: "fields map correctly",
			input: &model.GroupsIOMailingList{
				ProjectUID:     "proj-sfid",
				ServiceUID:     "svc-sfid",
				GroupName:      "My List",
				Description:    "Desc",
				Type:           "announcement",
				AudienceAccess: "public",
				Committees:     []model.Committee{{UID: "c-1"}},
			},
			expectCommitteeID: "c-1",
			expectProjectID:   "proj-sfid",
			expectParentID:    "svc-sfid",
			expectName:        "My List",
			expectDescription: "Desc",
			expectType:        "announcement",
			expectAccess:      "public",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := toWireSubgroupRequest(tt.input)
			s.Require().NotNil(got)
			s.Equal(tt.expectCommitteeID, got.CommitteeID)
			s.Equal(tt.expectProjectID, got.ProjectID)
			s.Equal(tt.expectParentID, got.ParentID)
			s.Equal(tt.expectName, got.Name)
			s.Equal(tt.expectDescription, got.Description)
			s.Equal(tt.expectType, got.Type)
			s.Equal(tt.expectAccess, got.AudienceAccess)
		})
	}
}

func (s *ProxyConvertersSuite) TestFromWireArtifactUser() {
	tests := []struct {
		name               string
		input              *artifactUserWire
		expectNil          bool
		expectID           string
		expectUsername     string
		expectName         string
		expectEmail        string
		expectProfilePic   string
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name: "fields map correctly",
			input: &artifactUserWire{
				ID:             "user-1",
				Username:       "alice",
				Name:           "Alice Smith",
				Email:          "alice@example.com",
				ProfilePicture: "https://example.com/pic.jpg",
			},
			expectID:         "user-1",
			expectUsername:   "alice",
			expectName:       "Alice Smith",
			expectEmail:      "alice@example.com",
			expectProfilePic: "https://example.com/pic.jpg",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := fromWireArtifactUser(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectID, got.ID)
			s.Equal(tt.expectUsername, got.Username)
			s.Equal(tt.expectName, got.Name)
			s.Equal(tt.expectEmail, got.Email)
			s.Equal(tt.expectProfilePic, got.ProfilePicture)
		})
	}
}

func (s *ProxyConvertersSuite) TestFromWireArtifact() {
	trueVal := true
	msgID1 := uint64(264364315)
	ts := time.Date(2026, 4, 2, 2, 9, 1, 0, time.UTC)

	tests := []struct {
		name                    string
		input                   *artifactWire
		expectNil               bool
		expectArtifactID        string
		expectGroupID           uint64
		expectProjectID         string
		expectCommitteeID       string
		expectType              string
		expectMediaType         string
		expectFilename          string
		expectLinkURL           string
		expectDownloadURL       string
		expectS3Key             string
		expectFileUploaded      *bool
		expectFileUploadStatus  string
		expectFileUploadedAt    *time.Time
		expectMessageIDs        []uint64
		expectLastPostedAt      *time.Time
		expectLastPostedMsgID   *uint64
		expectDescription       string
		expectCreatedByID       string
		expectLastModByID       string
		expectCreatedAt         time.Time
		expectUpdatedAt         time.Time
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name:  "valid RFC3339 created_at and last_modified_at are parsed",
			input: &artifactWire{CreatedAt: "2026-04-02T02:09:01Z", UpdatedAt: "2026-04-02T02:09:01Z"},
			expectCreatedAt: ts,
			expectUpdatedAt: ts,
		},
		{
			name:            "empty timestamps produce zero time",
			input:           &artifactWire{},
			expectCreatedAt: time.Time{},
			expectUpdatedAt: time.Time{},
		},
		{
			name:                 "valid file_uploaded_at is parsed into pointer",
			input:                &artifactWire{FileUploadedAt: "2026-04-02T02:09:01Z"},
			expectFileUploadedAt: &ts,
		},
		{
			name:                 "empty file_uploaded_at produces nil pointer",
			input:                &artifactWire{},
			expectFileUploadedAt: nil,
		},
		{
			name:               "valid last_posted_at is parsed into pointer",
			input:              &artifactWire{LastPostedAt: "2026-04-02T02:09:01Z"},
			expectLastPostedAt: &ts,
		},
		{
			name:               "empty last_posted_at produces nil pointer",
			input:              &artifactWire{},
			expectLastPostedAt: nil,
		},
		{
			name:               "file_uploaded pointer is passed through when set",
			input:              &artifactWire{FileUploaded: &trueVal},
			expectFileUploaded: &trueVal,
		},
		{
			name:               "nil file_uploaded pointer remains nil",
			input:              &artifactWire{FileUploaded: nil},
			expectFileUploaded: nil,
		},
		{
			name:                  "last_posted_message_id pointer is passed through when set",
			input:                 &artifactWire{LastPostedMessageID: &msgID1},
			expectLastPostedMsgID: &msgID1,
		},
		{
			name:                  "nil last_posted_message_id pointer remains nil",
			input:                 &artifactWire{LastPostedMessageID: nil},
			expectLastPostedMsgID: nil,
		},
		{
			name: "created_by and last_modified_by are mapped",
			input: &artifactWire{
				CreatedBy:      &artifactUserWire{ID: "user-1"},
				LastModifiedBy: &artifactUserWire{ID: "user-2"},
			},
			expectCreatedByID: "user-1",
			expectLastModByID: "user-2",
		},
		{
			name:  "nil created_by and last_modified_by produce nil",
			input: &artifactWire{},
		},
		{
			name: "all fields map correctly",
			input: &artifactWire{
				ArtifactID:          "a323373e-8553-578f-9aba-0235940641e3",
				GroupID:             118856,
				ProjectID:           "a09P000000DsQSFIA3",
				CommitteeID:         "committee-uuid",
				Type:                "file",
				MediaType:           "text/plain",
				Filename:            "test.txt",
				LinkURL:             "",
				DownloadURL:         "https://example.com/download",
				S3Key:               "group-artifacts/118856/test.txt",
				FileUploaded:        &trueVal,
				FileUploadStatus:    "completed",
				FileUploadedAt:      "2026-04-02T02:09:01Z",
				MessageIDs:          []uint64{264364315},
				LastPostedAt:        "2026-04-02T02:09:01Z",
				LastPostedMessageID: &msgID1,
				Description:         "a test file",
				CreatedBy:           &artifactUserWire{ID: "user-1"},
				LastModifiedBy:      &artifactUserWire{ID: "user-2"},
				CreatedAt:           "2026-04-02T02:09:01Z",
				UpdatedAt:           "2026-04-02T02:09:01Z",
			},
			expectArtifactID:       "a323373e-8553-578f-9aba-0235940641e3",
			expectGroupID:          118856,
			expectProjectID:        "a09P000000DsQSFIA3",
			expectCommitteeID:      "committee-uuid",
			expectType:             "file",
			expectMediaType:        "text/plain",
			expectFilename:         "test.txt",
			expectDownloadURL:      "https://example.com/download",
			expectS3Key:            "group-artifacts/118856/test.txt",
			expectFileUploaded:     &trueVal,
			expectFileUploadStatus: "completed",
			expectFileUploadedAt:   &ts,
			expectMessageIDs:       []uint64{264364315},
			expectLastPostedAt:     &ts,
			expectLastPostedMsgID:  &msgID1,
			expectDescription:      "a test file",
			expectCreatedByID:      "user-1",
			expectLastModByID:      "user-2",
			expectCreatedAt:        ts,
			expectUpdatedAt:        ts,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := fromWireArtifact(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectArtifactID, got.ArtifactID)
			s.Equal(tt.expectGroupID, got.GroupID)
			s.Equal(tt.expectProjectID, got.ProjectUID)
			s.Equal(tt.expectCommitteeID, got.CommitteeUID)
			s.Equal(tt.expectType, got.Type)
			s.Equal(tt.expectMediaType, got.MediaType)
			s.Equal(tt.expectFilename, got.Filename)
			s.Equal(tt.expectLinkURL, got.LinkURL)
			s.Equal(tt.expectDownloadURL, got.DownloadURL)
			s.Equal(tt.expectS3Key, got.S3Key)
			s.Equal(tt.expectFileUploaded, got.FileUploaded)
			s.Equal(tt.expectFileUploadStatus, got.FileUploadStatus)
			s.Equal(tt.expectMessageIDs, got.MessageIDs)
			s.Equal(tt.expectDescription, got.Description)
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)

			if tt.expectFileUploadedAt == nil {
				s.Nil(got.FileUploadedAt)
			} else {
				s.Require().NotNil(got.FileUploadedAt)
				s.Equal(*tt.expectFileUploadedAt, *got.FileUploadedAt)
			}
			if tt.expectLastPostedAt == nil {
				s.Nil(got.LastPostedAt)
			} else {
				s.Require().NotNil(got.LastPostedAt)
				s.Equal(*tt.expectLastPostedAt, *got.LastPostedAt)
			}
			if tt.expectLastPostedMsgID == nil {
				s.Nil(got.LastPostedMessageID)
			} else {
				s.Require().NotNil(got.LastPostedMessageID)
				s.Equal(*tt.expectLastPostedMsgID, *got.LastPostedMessageID)
			}
			if tt.expectCreatedByID == "" {
				s.Nil(got.CreatedBy)
			} else {
				s.Require().NotNil(got.CreatedBy)
				s.Equal(tt.expectCreatedByID, got.CreatedBy.ID)
			}
			if tt.expectLastModByID == "" {
				s.Nil(got.LastModifiedBy)
			} else {
				s.Require().NotNil(got.LastModifiedBy)
				s.Equal(tt.expectLastModByID, got.LastModifiedBy.ID)
			}
		})
	}
}

func (s *ProxyConvertersSuite) TestFromWireMember() {
	memberID42 := int64(42)
	memberID99 := int64(99)
	ts1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		input            *memberWire
		expectNil        bool
		expectUID        string
		expectMemberID   *int64
		expectEmail      string
		expectFullName   string
		expectUsername   string
		expectDelivery   string
		expectModStatus  string
		expectStatus     string
		expectMemberType string
		expectVoting     string
		expectOrg        string
		expectJobTitle   string
		expectRole       string
		expectCreatedAt  time.Time
		expectUpdatedAt  time.Time
	}{
		{
			name:      "nil input returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			// GET-style: string ID set, MemberID is zero.
			name:           "GET-style: string ID used as UID, parsed into MemberID pointer",
			input:          &memberWire{ID: "99", MemberID: 0},
			expectUID:      "99",
			expectMemberID: &memberID99,
		},
		{
			// POST-style: MemberID set, ID is empty.
			name:           "POST-style: int MemberID stringified as UID",
			input:          &memberWire{ID: "", MemberID: 42},
			expectUID:      "42",
			expectMemberID: &memberID42,
		},
		{
			// Both present: string ID wins for UID; MemberID pointer comes from int field.
			name:           "both present: string ID wins for UID, MemberID pointer from int field",
			input:          &memberWire{ID: "str-id", MemberID: 42},
			expectUID:      "str-id",
			expectMemberID: &memberID42,
		},
		{
			// Non-numeric string ID: parse fails, pointer stays nil.
			name:           "non-numeric string ID leaves MemberID pointer nil",
			input:          &memberWire{ID: "non-numeric", MemberID: 0},
			expectUID:      "non-numeric",
			expectMemberID: nil,
		},
		{
			name:            "valid RFC3339 timestamps are parsed",
			input:           &memberWire{MemberID: 1, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-06-01T12:00:00Z"},
			expectUID:       "1",
			expectMemberID:  func() *int64 { v := int64(1); return &v }(),
			expectCreatedAt: ts1,
			expectUpdatedAt: ts2,
		},
		{
			name: "fields map correctly",
			input: &memberWire{
				MemberID:     99,
				Email:        "alice@example.com",
				Name:         "Alice Smith",
				Username:     "alice",
				DeliveryMode: "single",
				ModStatus:    "none",
				Status:       "normal",
				MemberType:   "committee",
				VotingStatus: "approved",
				Organization: "Acme",
				JobTitle:     "Engineer",
				Role:         "member",
			},
			expectUID:        "99",
			expectMemberID:   &memberID99,
			expectEmail:      "alice@example.com",
			expectFullName:   "Alice Smith",
			expectUsername:   "alice",
			expectDelivery:   "single",
			expectModStatus:  "none",
			expectStatus:     "normal",
			expectMemberType: "committee",
			expectVoting:     "approved",
			expectOrg:        "Acme",
			expectJobTitle:   "Engineer",
			expectRole:       "member",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := fromWireMember(tt.input)
			if tt.expectNil {
				s.Nil(got)
				return
			}
			s.Require().NotNil(got)
			s.Equal(tt.expectUID, got.UID)
			s.Equal(tt.expectEmail, got.Email)
			s.Equal(tt.expectFullName, got.GroupsFullName)
			s.Equal(tt.expectUsername, got.Username)
			s.Equal(tt.expectDelivery, got.DeliveryMode)
			s.Equal(tt.expectModStatus, got.ModStatus)
			s.Equal(tt.expectStatus, got.Status)
			s.Equal(tt.expectMemberType, got.MemberType)
			s.Equal(tt.expectVoting, got.VotingStatus)
			s.Equal(tt.expectOrg, got.Organization)
			s.Equal(tt.expectJobTitle, got.JobTitle)
			s.Equal(tt.expectRole, got.Role)
			s.Equal(tt.expectCreatedAt, got.CreatedAt)
			s.Equal(tt.expectUpdatedAt, got.UpdatedAt)
			if tt.expectMemberID == nil {
				s.Nil(got.MemberID)
			} else {
				s.Require().NotNil(got.MemberID)
				s.Equal(*tt.expectMemberID, *got.MemberID)
			}
		})
	}
}
