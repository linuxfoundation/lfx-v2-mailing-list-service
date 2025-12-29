// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func TestGrpsIOMailingList_ValidateBasicFields(t *testing.T) {
	tests := []struct {
		name        string
		mailingList func() *GrpsIOMailingList
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid mailing list",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: false,
		},
		{
			name: "empty group name",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "group_name is required",
		},
		{
			name: "invalid group name - starts with number",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "1dev-team",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "group_name must match pattern: ^[a-z][a-z0-9-]*[a-z0-9]$",
		},
		{
			name: "invalid group name - contains uppercase",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "Dev-team",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "group_name must match pattern: ^[a-z][a-z0-9-]*[a-z0-9]$",
		},
		{
			name: "invalid group name - ends with hyphen",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team-",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "group_name must match pattern: ^[a-z][a-z0-9-]*[a-z0-9]$",
		},
		{
			name: "valid group name - single character",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "a1",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: false,
		},
		{
			name: "empty type",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team",
					Type:        "",
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "type is required",
		},
		{
			name: "invalid type",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team",
					Type:        "invalid_type",
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "type must be 'announcement', 'discussion_moderated', or 'discussion_open'",
		},
		{
			name: "empty description",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team",
					Type:        TypeDiscussionOpen,
					Description: "",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "description is required",
		},
		{
			name: "description too short",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team",
					Type:        TypeDiscussionOpen,
					Description: "too short",
					Title:       "Development Team",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "description must be at least 11 characters long",
		},
		{
			name: "empty title",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "",
					ServiceUID:  "service-123",
				}
			},
			expectError: true,
			errorMsg:    "title is required",
		},
		{
			name: "empty service uid",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName:   "dev-team",
					Type:        TypeDiscussionOpen,
					Description: "Development team discussions and updates",
					Title:       "Development Team",
					ServiceUID:  "",
				}
			},
			expectError: true,
			errorMsg:    "parent_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ml := tt.mailingList()
			err := ml.ValidateBasicFields()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.IsType(t, errors.Validation{}, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGrpsIOMailingList_ValidateCommitteeFields(t *testing.T) {
	tests := []struct {
		name        string
		mailingList func() *GrpsIOMailingList
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid committee with allowed voting statuses",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					Committees: []Committee{
						{UID: "committee-123", AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep, CommitteeVotingStatusObserver}},
					},
				}
			},
			expectError: false,
		},
		{
			name: "valid committee without allowed voting statuses",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					Committees: []Committee{
						{UID: "committee-123", AllowedVotingStatuses: []string{}},
					},
				}
			},
			expectError: false,
		},
		{
			name: "no committee no allowed voting statuses",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					Committees: []Committee{},
				}
			},
			expectError: false,
		},
		{
			name: "allowed voting statuses without committee UID",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					Committees: []Committee{
						{UID: "", AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep}},
					},
				}
			},
			expectError: true,
			errorMsg:    "committees[0].uid is required",
		},
		{
			name: "invalid committee allowed voting status",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					Committees: []Committee{
						{UID: "committee-123", AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep, "invalid_status"}},
					},
				}
			},
			expectError: true,
			errorMsg:    "invalid committees[0].allowed_voting_statuses value: invalid_status",
		},
		{
			name: "all valid committee allowed voting statuses",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					Committees: []Committee{
						{
							UID: "committee-123",
							AllowedVotingStatuses: []string{
								CommitteeVotingStatusVotingRep,
								CommitteeVotingStatusAltVotingRep,
								CommitteeVotingStatusObserver,
								CommitteeVotingStatusEmeritus,
								CommitteeVotingStatusNone,
							},
						},
					},
				}
			},
			expectError: false,
		},
		{
			name: "multiple valid committees",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					Committees: []Committee{
						{UID: "committee-1", AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep}},
						{UID: "committee-2", AllowedVotingStatuses: []string{CommitteeVotingStatusObserver}},
					},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ml := tt.mailingList()
			err := ml.ValidateCommitteeFields()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.IsType(t, errors.Validation{}, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGrpsIOMailingList_ValidateGroupNamePrefix(t *testing.T) {
	tests := []struct {
		name                string
		mailingList         func() *GrpsIOMailingList
		parentServiceType   string
		parentServicePrefix string
		expectError         bool
		errorMsg            string
	}{
		{
			name: "primary service - no prefix required",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName: "announcements",
				}
			},
			parentServiceType:   "primary",
			parentServicePrefix: "",
			expectError:         false,
		},
		{
			name: "formation service - valid prefix",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName: "formation-dev",
				}
			},
			parentServiceType:   "formation",
			parentServicePrefix: "formation",
			expectError:         false,
		},
		{
			name: "formation service - invalid prefix",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName: "dev-team",
				}
			},
			parentServiceType:   "formation",
			parentServicePrefix: "formation",
			expectError:         true,
			errorMsg:            "group_name must start with parent service prefix 'formation-' for formation services",
		},
		{
			name: "non-primary service - missing prefix",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName: "dev-team",
				}
			},
			parentServiceType:   "formation",
			parentServicePrefix: "",
			expectError:         true,
			errorMsg:            "parent service prefix is required for non-primary services",
		},
		{
			name: "shared service - valid prefix",
			mailingList: func() *GrpsIOMailingList {
				return &GrpsIOMailingList{
					GroupName: "shared-project-dev",
				}
			},
			parentServiceType:   "shared",
			parentServicePrefix: "shared-project",
			expectError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ml := tt.mailingList()
			err := ml.ValidateGroupNamePrefix(tt.parentServiceType, tt.parentServicePrefix)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.IsType(t, errors.Validation{}, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGrpsIOMailingList_IsCommitteeBased(t *testing.T) {
	tests := []struct {
		name        string
		mailingList *GrpsIOMailingList
		expected    bool
	}{
		{
			name: "single committee without filters",
			mailingList: &GrpsIOMailingList{
				Committees: []Committee{
					{UID: "committee-123", AllowedVotingStatuses: []string{}},
				},
			},
			expected: true,
		},
		{
			name: "single committee with filters",
			mailingList: &GrpsIOMailingList{
				Committees: []Committee{
					{UID: "committee-123", AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep}},
				},
			},
			expected: true,
		},
		{
			name: "multiple committees",
			mailingList: &GrpsIOMailingList{
				Committees: []Committee{
					{UID: "committee-1", AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep}},
					{UID: "committee-2", AllowedVotingStatuses: []string{CommitteeVotingStatusObserver}},
				},
			},
			expected: true,
		},
		{
			name: "empty committees array",
			mailingList: &GrpsIOMailingList{
				Committees: []Committee{},
			},
			expected: false,
		},
		{
			name: "nil committees",
			mailingList: &GrpsIOMailingList{
				Committees: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mailingList.IsCommitteeBased()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGrpsIOMailingList_BuildIndexKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		mailingList *GrpsIOMailingList
	}{
		{
			name: "basic mailing list",
			mailingList: &GrpsIOMailingList{
				ServiceUID: "service-123",
				GroupName:  "dev-team",
			},
		},
		{
			name: "mailing list with special characters",
			mailingList: &GrpsIOMailingList{
				ServiceUID: "service-456",
				GroupName:  "dev-team-special",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := tt.mailingList.BuildIndexKey(ctx)
			key2 := tt.mailingList.BuildIndexKey(ctx)

			// Keys should be consistent
			assert.Equal(t, key1, key2, "Index keys should be consistent for same input")

			// Keys should be valid SHA-256 hex strings (64 characters)
			assert.Len(t, key1, 64, "Index key should be 64 characters (SHA-256 hex)")

			// Keys should only contain hex characters
			for _, char := range key1 {
				assert.True(t, (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'),
					"Index key should only contain hex characters")
			}
		})
	}

	// Test different inputs produce different keys
	t.Run("different inputs produce different keys", func(t *testing.T) {
		ml1 := &GrpsIOMailingList{
			ServiceUID: "service-123",
			GroupName:  "dev-team",
		}
		ml2 := &GrpsIOMailingList{
			ServiceUID: "service-456",
			GroupName:  "dev-team",
		}

		key1 := ml1.BuildIndexKey(ctx)
		key2 := ml2.BuildIndexKey(ctx)

		assert.NotEqual(t, key1, key2, "Different inputs should produce different keys")
	})
}

func TestGrpsIOMailingList_Tags(t *testing.T) {
	tests := []struct {
		name         string
		mailingList  *GrpsIOMailingList
		expectedTags []string
	}{
		{
			name:         "nil mailing list",
			mailingList:  nil,
			expectedTags: nil,
		},
		{
			name: "complete mailing list",
			mailingList: &GrpsIOMailingList{
				UID:        "ml-123",
				ProjectUID: "project-456",
				ServiceUID: "service-789",
				Type:       TypeDiscussionOpen,
				Public:     true,
				Committees: []Committee{
					{UID: "committee-123", AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep, CommitteeVotingStatusObserver}},
				},
			},
			expectedTags: []string{
				"project_uid:project-456",
				"service_uid:service-789",
				"type:discussion_open",
				"public:true",
				"committee_uid:committee-123",
				"committee_voting_status:Voting Rep",
				"committee_voting_status:Observer",
				"groupsio_mailing_list_uid:ml-123",
			},
		},
		{
			name: "minimal mailing list",
			mailingList: &GrpsIOMailingList{
				Public: false,
			},
			expectedTags: []string{
				"public:false",
			},
		},
		{
			name: "mailing list with some fields",
			mailingList: &GrpsIOMailingList{
				ProjectUID: "project-123",
				Type:       TypeAnnouncement,
				Public:     true,
			},
			expectedTags: []string{
				"project_uid:project-123",
				"type:announcement",
				"public:true",
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

func TestValidCommitteeVotingStatuses(t *testing.T) {
	votingStatuses := ValidCommitteeVotingStatuses()

	expectedVotingStatuses := []string{
		CommitteeVotingStatusVotingRep,
		CommitteeVotingStatusAltVotingRep,
		CommitteeVotingStatusObserver,
		CommitteeVotingStatusEmeritus,
		CommitteeVotingStatusNone,
	}

	assert.Equal(t, expectedVotingStatuses, votingStatuses)
	assert.Len(t, votingStatuses, 5, "Should return 5 valid committee voting statuses")
}

func TestIsValidGroupName(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		expected  bool
	}{
		// Valid cases
		{"valid simple", "dev", true},
		{"valid with hyphen", "dev-team", true},
		{"valid with numbers", "dev2-team3", true},
		{"valid starting with letter ending with number", "a1", true},
		{"valid complex", "project-dev-team2", true},

		// Invalid cases - too short
		{"too short - single char", "a", false},
		{"empty string", "", false},

		// Invalid cases - start with wrong character
		{"starts with number", "1dev", false},
		{"starts with hyphen", "-dev", false},
		{"starts with uppercase", "Dev", false},

		// Invalid cases - ends with wrong character
		{"ends with hyphen", "dev-", false},

		// Invalid cases - invalid middle characters
		{"contains uppercase", "dev-Team", false},
		{"contains special char", "dev@team", false},
		{"contains space", "dev team", false},
		{"contains underscore", "dev_team", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidGroupName(tt.groupName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidMailingListType(t *testing.T) {
	tests := []struct {
		name     string
		mlType   string
		expected bool
	}{
		{"valid announcement", TypeAnnouncement, true},
		{"valid discussion moderated", TypeDiscussionModerated, true},
		{"valid discussion open", TypeDiscussionOpen, true},
		{"invalid type", "invalid_type", false},
		{"empty type", "", false},
		{"uppercase type", "ANNOUNCEMENT", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidMailingListType(tt.mlType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"item found", []string{"a", "b", "c"}, "b", true},
		{"item not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
		{"case sensitive", []string{"Test"}, "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests for performance-critical functions
func BenchmarkGrpsIOMailingList_BuildIndexKey(b *testing.B) {
	ctx := context.Background()
	ml := &GrpsIOMailingList{
		ServiceUID: "service-" + uuid.New().String(),
		GroupName:  "dev-team-benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ml.BuildIndexKey(ctx)
	}
}

func BenchmarkGrpsIOMailingList_Tags(b *testing.B) {
	ml := &GrpsIOMailingList{
		UID:        "ml-" + uuid.New().String(),
		ProjectUID: "project-" + uuid.New().String(),
		ServiceUID: "service-" + uuid.New().String(),
		Type:       TypeDiscussionOpen,
		Public:     true,
		Committees: []Committee{
			{UID: "committee-" + uuid.New().String(), AllowedVotingStatuses: []string{CommitteeVotingStatusVotingRep, CommitteeVotingStatusObserver}},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ml.Tags()
	}
}

// Helper function to create test mailing list with valid basic fields
func createValidTestMailingList() *GrpsIOMailingList {
	return &GrpsIOMailingList{
		UID:         uuid.New().String(),
		GroupName:   "test-group",
		Public:      true,
		Type:        TypeDiscussionOpen,
		Description: "Test description with enough characters",
		Title:       "Test Title",
		ServiceUID:  uuid.New().String(),
		ProjectUID:  uuid.New().String(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
