// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGrpsIOMember_Tags(t *testing.T) {
	tests := []struct {
		name         string
		member       *GrpsIOMember
		expectedTags []string
	}{
		{
			name:         "nil member",
			member:       nil,
			expectedTags: nil,
		},
		{
			name: "complete member",
			member: &GrpsIOMember{
				UID:            "member-123",
				MailingListUID: "mailing-list-456",
				Username:       "testuser",
				Email:          "test@example.com",
				Status:         "normal",
			},
			expectedTags: []string{
				"member-123",
				"member_uid:member-123",
				"mailing_list_uid:mailing-list-456",
				"username:testuser",
				"email:test@example.com",
				"status:normal",
			},
		},
		{
			name: "minimal member - empty fields",
			member: &GrpsIOMember{
				UID:            "",
				MailingListUID: "",
				Username:       "",
				Email:          "",
				Status:         "",
			},
			expectedTags: nil,
		},
		{
			name: "member with partial fields",
			member: &GrpsIOMember{
				UID:            "member-789",
				MailingListUID: "mailing-list-999",
				Email:          "partial@example.com",
			},
			expectedTags: []string{
				"member-789",
				"member_uid:member-789",
				"mailing_list_uid:mailing-list-999",
				"email:partial@example.com",
			},
		},
		{
			name: "member with only username",
			member: &GrpsIOMember{
				Username: "onlyusername",
			},
			expectedTags: []string{
				"username:onlyusername",
			},
		},
		{
			name: "member with only status",
			member: &GrpsIOMember{
				Status: "pending",
			},
			expectedTags: []string{
				"status:pending",
			},
		},
		{
			name: "member with all tag-generating fields",
			member: &GrpsIOMember{
				UID:            "comprehensive-member",
				MailingListUID: "comprehensive-list",
				Username:       "comprehensive-user",
				Email:          "comprehensive@example.com",
				Status:         "normal",
			},
			expectedTags: []string{
				"comprehensive-member",
				"member_uid:comprehensive-member",
				"mailing_list_uid:comprehensive-list",
				"username:comprehensive-user",
				"email:comprehensive@example.com",
				"status:normal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := tt.member.Tags()
			assert.Equal(t, tt.expectedTags, tags)
		})
	}
}

func BenchmarkGrpsIOMember_Tags(b *testing.B) {
	member := &GrpsIOMember{
		UID:            "member-" + uuid.New().String(),
		MailingListUID: "mailing-list-" + uuid.New().String(),
		Username:       "benchmark-member",
		Email:          "benchmark@example.com",
		Status:         "normal",
	}

	for b.Loop() {
		_ = member.Tags()
	}
}
