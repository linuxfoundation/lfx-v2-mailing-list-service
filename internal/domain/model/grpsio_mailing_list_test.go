// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGroupsIOMailingList_Tags(t *testing.T) {
	tests := []struct {
		name         string
		mailingList  *GroupsIOMailingList
		expectedTags []string
	}{
		{
			name:         "nil mailing list",
			mailingList:  nil,
			expectedTags: nil,
		},
		{
			name:        "empty mailing list - only public tag",
			mailingList: &GroupsIOMailingList{},
			expectedTags: []string{
				"public:false",
			},
		},
		{
			name: "public mailing list",
			mailingList: &GroupsIOMailingList{
				Public: true,
			},
			expectedTags: []string{
				"public:true",
			},
		},
		{
			name: "complete mailing list - no committees",
			mailingList: &GroupsIOMailingList{
				UID:            "ml-123",
				ProjectUID:     "project-456",
				ServiceUID:     "service-789",
				GroupName:      "test-group",
				Type:           TypeDiscussionOpen,
				Public:         true,
				AudienceAccess: "public",
			},
			expectedTags: []string{
				"project_uid:project-456",
				"service_uid:service-789",
				"type:discussion_open",
				"public:true",
				"audience_access:public",
				"groupsio_mailing_list_uid:ml-123",
				"group_name:test-group",
			},
		},
		{
			name: "mailing list with single committee - no voting statuses",
			mailingList: &GroupsIOMailingList{
				UID:    "ml-111",
				Public: false,
				Committees: []Committee{
					{UID: "committee-aaa"},
				},
			},
			expectedTags: []string{
				"public:false",
				"committee_uid:committee-aaa",
				"groupsio_mailing_list_uid:ml-111",
			},
		},
		{
			name: "mailing list with single committee - with voting statuses",
			mailingList: &GroupsIOMailingList{
				UID:    "ml-222",
				Public: true,
				Committees: []Committee{
					{
						UID:                   "committee-bbb",
						AllowedVotingStatuses: []string{"Voting Rep", "Alternate Voting Rep"},
					},
				},
			},
			expectedTags: []string{
				"public:true",
				"committee_uid:committee-bbb",
				"committee_voting_status:Voting Rep",
				"committee_voting_status:Alternate Voting Rep",
				"groupsio_mailing_list_uid:ml-222",
			},
		},
		{
			name: "mailing list with multiple committees",
			mailingList: &GroupsIOMailingList{
				UID:    "ml-333",
				Public: false,
				Committees: []Committee{
					{
						UID:                   "committee-ccc",
						AllowedVotingStatuses: []string{"Voting Rep"},
					},
					{
						UID:                   "committee-ddd",
						AllowedVotingStatuses: []string{"Observer"},
					},
				},
			},
			expectedTags: []string{
				"public:false",
				"committee_uid:committee-ccc",
				"committee_voting_status:Voting Rep",
				"committee_uid:committee-ddd",
				"committee_voting_status:Observer",
				"groupsio_mailing_list_uid:ml-333",
			},
		},
		{
			name: "committee with empty UID - skips committee_uid tag but keeps voting statuses",
			mailingList: &GroupsIOMailingList{
				Public: false,
				Committees: []Committee{
					{
						UID:                   "",
						AllowedVotingStatuses: []string{"Voting Rep"},
					},
				},
			},
			expectedTags: []string{
				"public:false",
				"committee_voting_status:Voting Rep",
			},
		},
		{
			name: "mailing list with only project UID",
			mailingList: &GroupsIOMailingList{
				ProjectUID: "project-only",
			},
			expectedTags: []string{
				"project_uid:project-only",
				"public:false",
			},
		},
		{
			name: "mailing list with only service UID",
			mailingList: &GroupsIOMailingList{
				ServiceUID: "service-only",
			},
			expectedTags: []string{
				"service_uid:service-only",
				"public:false",
			},
		},
		{
			name: "mailing list with only type",
			mailingList: &GroupsIOMailingList{
				Type: "announcement",
			},
			expectedTags: []string{
				"type:announcement",
				"public:false",
			},
		},
		{
			name: "mailing list with only audience access",
			mailingList: &GroupsIOMailingList{
				AudienceAccess: "invite_only",
			},
			expectedTags: []string{
				"public:false",
				"audience_access:invite_only",
			},
		},
		{
			name: "mailing list with only group name",
			mailingList: &GroupsIOMailingList{
				GroupName: "my-group",
			},
			expectedTags: []string{
				"public:false",
				"group_name:my-group",
			},
		},
		{
			name: "tag order - project before service before type before public",
			mailingList: &GroupsIOMailingList{
				ProjectUID: "proj-1",
				ServiceUID: "svc-1",
				Type:       TypeDiscussionModerated,
				Public:     true,
			},
			expectedTags: []string{
				"project_uid:proj-1",
				"service_uid:svc-1",
				"type:discussion_moderated",
				"public:true",
			},
		},
		{
			name: "tag order - committees appear after audience_access and before uid",
			mailingList: &GroupsIOMailingList{
				UID:            "ml-order",
				AudienceAccess: "approval_required",
				Public:         false,
				Committees: []Committee{
					{UID: "committee-order"},
				},
			},
			expectedTags: []string{
				"public:false",
				"audience_access:approval_required",
				"committee_uid:committee-order",
				"groupsio_mailing_list_uid:ml-order",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := tt.mailingList.Tags()
			assert.Equal(t, tt.expectedTags, tags)
		})
	}
}

func TestGroupsIOMailingListSettings_Tags(t *testing.T) {
	tests := []struct {
		name         string
		settings     *GroupsIOMailingListSettings
		expectedTags []string
	}{
		{
			name:         "nil settings",
			settings:     nil,
			expectedTags: nil,
		},
		{
			name:         "empty UID",
			settings:     &GroupsIOMailingListSettings{},
			expectedTags: nil,
		},
		{
			name: "with UID",
			settings: &GroupsIOMailingListSettings{
				UID: "ml-settings-123",
			},
			expectedTags: []string{
				"ml-settings-123",
				"mailing_list_uid:ml-settings-123",
			},
		},
		{
			name: "UID produces bare uid tag and prefixed tag",
			settings: &GroupsIOMailingListSettings{
				UID: "settings-uid-456",
			},
			expectedTags: []string{
				"settings-uid-456",
				"mailing_list_uid:settings-uid-456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := tt.settings.Tags()
			assert.Equal(t, tt.expectedTags, tags)
		})
	}
}

func BenchmarkGroupsIOMailingList_Tags(b *testing.B) {
	ml := &GroupsIOMailingList{
		UID:            "ml-" + uuid.New().String(),
		ProjectUID:     "project-" + uuid.New().String(),
		ServiceUID:     "service-" + uuid.New().String(),
		GroupName:      "benchmark-group",
		Type:           "discussion_open",
		Public:         true,
		AudienceAccess: "public",
		Committees: []Committee{
			{
				UID:                   "committee-" + uuid.New().String(),
				AllowedVotingStatuses: []string{"Voting Rep", "Alternate Voting Rep"},
			},
		},
	}

	for b.Loop() {
		_ = ml.Tags()
	}
}

func BenchmarkGroupsIOMailingListSettings_Tags(b *testing.B) {
	settings := &GroupsIOMailingListSettings{
		UID: "ml-settings-" + uuid.New().String(),
	}

	for b.Loop() {
		_ = settings.Tags()
	}
}

// createValidTestMailingList returns a fully populated GroupsIOMailingList for use in tests
// across the model package.
func createValidTestMailingList() *GroupsIOMailingList {
	return &GroupsIOMailingList{
		UID:            "ml-" + uuid.New().String(),
		ProjectUID:     "project-" + uuid.New().String(),
		ServiceUID:     "service-" + uuid.New().String(),
		GroupName:      "test-group",
		Type:           TypeDiscussionOpen,
		Public:         true,
		AudienceAccess: "public",
		Description:    "A valid test mailing list",
		Title:          "Test Mailing List",
		Committees: []Committee{
			{
				UID:                   "committee-" + uuid.New().String(),
				AllowedVotingStatuses: []string{"Voting Rep"},
			},
		},
	}
}
