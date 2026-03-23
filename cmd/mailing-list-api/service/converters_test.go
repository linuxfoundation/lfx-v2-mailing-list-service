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
		name              string
		input             *model.GrpsIOMember
		expectNil         bool
		expectEmail       string
		expectName        string
		expectCreatedAt   *string
		expectUpdatedAt   *string
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
				UserID:         "user-1",
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
			if tt.expectProjectUID != "" {
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

// ptr is a helper to get a pointer to a string literal.
func ptr(s string) *string { return &s }

// ptrVal safely dereferences a *string, returning "" if nil.
func ptrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
