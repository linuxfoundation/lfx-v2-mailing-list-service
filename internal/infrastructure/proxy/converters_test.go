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
	strPtr := func(v string) *string { return &v }

	tests := []struct {
		name              string
		input             *model.GroupsIOMailingList
		expectCommitteeID *string
		expectProjectID   string
		expectParentID    string
		expectName        string
		expectDescription string
		expectType        string
		expectAccess      string
	}{
		{
			name:              "nil committees produces nil CommitteeID",
			input:             &model.GroupsIOMailingList{Committees: nil},
			expectCommitteeID: nil,
		},
		{
			name:              "empty committees slice produces pointer to empty string (clear)",
			input:             &model.GroupsIOMailingList{Committees: []model.Committee{}},
			expectCommitteeID: strPtr(""),
		},
		{
			name: "only first committee UID is serialized",
			input: &model.GroupsIOMailingList{
				Committees: []model.Committee{{UID: "first"}, {UID: "second"}},
			},
			expectCommitteeID: strPtr("first"),
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
			expectCommitteeID: strPtr("c-1"),
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
		expectUserID     string
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
				UserID:       "user-1",
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
			expectUserID:     "user-1",
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
			s.Equal(tt.expectUserID, got.UserID)
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
