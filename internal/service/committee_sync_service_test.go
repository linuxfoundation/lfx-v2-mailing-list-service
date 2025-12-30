// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitteeSyncService_HandleMessage(t *testing.T) {
	ctx := context.Background()

	// Setup mock repository
	mockRepo := mock.NewMockRepository()
	mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
	memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
	memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
	entityReader := mock.NewMockEntityAttributeReader(mockRepo)

	// Create service
	service := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)
	require.NotNil(t, service)

	t.Run("handles created event", func(t *testing.T) {
		event := model.CommitteeMemberCreatedEvent{
			MemberUID:    "member-123",
			CommitteeUID: "committee-1",
			ProjectUID:   "project-456",
			Member: model.CommitteeMemberEventData{
				Email:        "test@example.com",
				FirstName:    "John",
				LastName:     "Doe",
				Username:     "johndoe",
				VotingStatus: "Voting Rep",
				Organization: model.Organization{Name: "ACME Corp"},
				JobTitle:     "Engineer",
			},
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.CommitteeMemberCreatedSubject,
			Data:    data,
		}

		// Should not panic
		service.HandleMessage(ctx, msg)
	})

	t.Run("handles deleted event", func(t *testing.T) {
		event := model.CommitteeMemberDeletedEvent{
			MemberUID:    "member-123",
			CommitteeUID: "committee-1",
			ProjectUID:   "project-456",
			Email:        "test@example.com",
			Timestamp:    time.Now(),
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.CommitteeMemberDeletedSubject,
			Data:    data,
		}

		// Should not panic
		service.HandleMessage(ctx, msg)
	})

	t.Run("handles updated event", func(t *testing.T) {
		event := model.CommitteeMemberUpdatedEvent{
			MemberUID:    "member-123",
			CommitteeUID: "committee-1",
			ProjectUID:   "project-456",
			OldMember: model.CommitteeMemberEventData{
				Email:        "old@example.com",
				FirstName:    "John",
				LastName:     "Doe",
				Username:     "johndoe",
				VotingStatus: "Observer",
				Organization: model.Organization{Name: "ACME Corp"},
				JobTitle:     "Engineer",
			},
			NewMember: model.CommitteeMemberEventData{
				Email:        "new@example.com",
				FirstName:    "John",
				LastName:     "Doe",
				Username:     "johndoe",
				VotingStatus: "Voting Rep",
				Organization: model.Organization{Name: "ACME Corp"},
				JobTitle:     "Senior Engineer",
			},
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.CommitteeMemberUpdatedSubject,
			Data:    data,
		}

		// Should not panic
		service.HandleMessage(ctx, msg)
	})

	t.Run("handles unknown subject", func(t *testing.T) {
		msg := &nats.Msg{
			Subject: "unknown.subject",
			Data:    []byte("{}"),
		}

		// Should not panic, should log warning
		service.HandleMessage(ctx, msg)
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		msg := &nats.Msg{
			Subject: constants.CommitteeMemberCreatedSubject,
			Data:    []byte("invalid json"),
		}

		// Should not panic, should log error
		service.HandleMessage(ctx, msg)
	})
}

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		name         string
		votingStatus string
		filters      []string
		expected     bool
	}{
		{
			name:         "matches voting rep",
			votingStatus: "Voting Rep",
			filters:      []string{"Voting Rep", "Observer"},
			expected:     true,
		},
		{
			name:         "matches observer",
			votingStatus: "Observer",
			filters:      []string{"Voting Rep", "Observer"},
			expected:     true,
		},
		{
			name:         "does not match",
			votingStatus: "Emeritus",
			filters:      []string{"Voting Rep", "Observer"},
			expected:     false,
		},
		{
			name:         "empty filters",
			votingStatus: "Voting Rep",
			filters:      []string{},
			expected:     false,
		},
		{
			name:         "nil filters",
			votingStatus: "Voting Rep",
			filters:      nil,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesFilter(tt.votingStatus, tt.filters)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommitteeSyncService_AddMemberToList(t *testing.T) {
	ctx := context.Background()

	// Setup mock repository
	mockRepo := mock.NewMockRepository()
	mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
	memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
	memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
	entityReader := mock.NewMockEntityAttributeReader(mockRepo)

	service := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)

	mailingList := &model.GrpsIOMailingList{
		UID:       "list-123",
		GroupName: "dev",
		Public:    true,
	}

	memberData := model.CommitteeMemberEventData{
		Email:        "newmember@example.com",
		FirstName:    "Jane",
		LastName:     "Smith",
		Username:     "janesmith",
		VotingStatus: "Voting Rep",
		Organization: model.Organization{Name: "ACME Corp"},
		JobTitle:     "Developer",
	}

	t.Run("adds new member successfully", func(t *testing.T) {
		err := service.addMemberToList(ctx, mailingList, memberData)
		assert.NoError(t, err)
	})

	t.Run("handles existing member (idempotent)", func(t *testing.T) {
		// Add member twice - should be idempotent
		err := service.addMemberToList(ctx, mailingList, memberData)
		assert.NoError(t, err)

		err = service.addMemberToList(ctx, mailingList, memberData)
		assert.NoError(t, err)
	})
}

func TestCommitteeSyncService_RemoveMemberFromList(t *testing.T) {
	ctx := context.Background()

	// Setup mock repository
	mockRepo := mock.NewMockRepository()
	mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
	memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
	memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
	entityReader := mock.NewMockEntityAttributeReader(mockRepo)

	service := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)

	t.Run("handles member not found (idempotent)", func(t *testing.T) {
		mailingList := &model.GrpsIOMailingList{
			UID:    "list-123",
			Public: true,
		}

		err := service.removeMemberFromList(ctx, mailingList, "nonexistent@example.com")
		assert.NoError(t, err) // Should be idempotent
	})
}

// TestCommitteeSyncService_IntegrationWithMailingLists tests the full flow with actual mailing lists
func TestCommitteeSyncService_IntegrationWithMailingLists(t *testing.T) {
	ctx := context.Background()

	t.Run("creates member when voting status matches filters", func(t *testing.T) {
		// Setup with pre-populated mailing list
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		mailingList := &model.GrpsIOMailingList{
			UID:        "test-list-1",
			GroupName:  "test-dev",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-123",
					Name:                  "Test Committee",
					AllowedVotingStatuses: []string{"Voting Rep", "Observer"},
				},
			},
			Public: true,
		}
		mockRepo.AddMailingList(mailingList)

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		service := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)

		event := model.CommitteeMemberCreatedEvent{
			MemberUID:    "member-123",
			CommitteeUID: "committee-123",
			ProjectUID:   "project-456",
			Member: model.CommitteeMemberEventData{
				Email:        "voter@example.com",
				FirstName:    "John",
				LastName:     "Doe",
				Username:     "johndoe",
				VotingStatus: "Voting Rep",
				Organization: model.Organization{Name: "ACME"},
				JobTitle:     "Engineer",
			},
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.CommitteeMemberCreatedSubject,
			Data:    data,
		}

		service.HandleMessage(ctx, msg)

		// Verify member was created
		members := mockRepo.GetMembersForMailingList("test-list-1")
		assert.Equal(t, 1, len(members))
		assert.Equal(t, "voter@example.com", members[0].Email)
		assert.Equal(t, "committee", members[0].MemberType)
		assert.Equal(t, "none", members[0].ModStatus)
	})

	t.Run("does not create member when voting status does not match", func(t *testing.T) {
		// Setup with pre-populated mailing list
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		mailingList := &model.GrpsIOMailingList{
			UID:        "test-list-2",
			GroupName:  "test-voting-only",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-456",
					Name:                  "Voting Committee",
					AllowedVotingStatuses: []string{"Voting Rep"}, // Only voting reps
				},
			},
			Public: true,
		}
		mockRepo.AddMailingList(mailingList)

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		service := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)

		event := model.CommitteeMemberCreatedEvent{
			MemberUID:    "member-456",
			CommitteeUID: "committee-456",
			ProjectUID:   "project-456",
			Member: model.CommitteeMemberEventData{
				Email:        "emeritus@example.com",
				VotingStatus: "Emeritus", // Not in filters
			},
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.CommitteeMemberCreatedSubject,
			Data:    data,
		}

		service.HandleMessage(ctx, msg)

		// Verify member was NOT created
		members := mockRepo.GetMembersForMailingList("test-list-2")
		assert.Equal(t, 0, len(members))
	})

	t.Run("deletes member from private list, converts on public list", func(t *testing.T) {
		// Setup with both public and private lists
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		publicList := &model.GrpsIOMailingList{
			UID:        "public-list",
			GroupName:  "public-dev",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-789",
					Name:                  "Public Committee",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: true,
		}
		mockRepo.AddMailingList(publicList)

		privateList := &model.GrpsIOMailingList{
			UID:        "private-list",
			GroupName:  "private-dev",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-789",
					Name:                  "Private Committee",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: false,
		}
		mockRepo.AddMailingList(privateList)

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		service := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)

		// First create a member in both lists
		memberData := model.CommitteeMemberEventData{
			Email:        "member@example.com",
			FirstName:    "Test",
			LastName:     "User",
			VotingStatus: "Voting Rep",
		}
		service.addMemberToList(ctx, publicList, memberData)
		service.addMemberToList(ctx, privateList, memberData)

		// Verify members created
		assert.Equal(t, 1, len(mockRepo.GetMembersForMailingList("public-list")))
		assert.Equal(t, 1, len(mockRepo.GetMembersForMailingList("private-list")))

		// Now delete the member
		deleteEvent := model.CommitteeMemberDeletedEvent{
			MemberUID:    "member-123",
			CommitteeUID: "committee-789",
			ProjectUID:   "project-456",
			Email:        "member@example.com",
		}

		data, err := json.Marshal(deleteEvent)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.CommitteeMemberDeletedSubject,
			Data:    data,
		}

		service.HandleMessage(ctx, msg)

		// Verify public list member still exists but converted to "direct"
		publicMembers := mockRepo.GetMembersForMailingList("public-list")
		assert.Equal(t, 1, len(publicMembers))
		assert.Equal(t, "direct", publicMembers[0].MemberType)

		// Verify private list member was deleted
		privateMembers := mockRepo.GetMembersForMailingList("private-list")
		assert.Equal(t, 0, len(privateMembers))
	})

	t.Run("handles voting status change - adds and removes appropriately", func(t *testing.T) {
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		mailingList := &model.GrpsIOMailingList{
			UID:        "test-list-3",
			GroupName:  "test-voting",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-999",
					Name:                  "Test Committee",
					AllowedVotingStatuses: []string{"Voting Rep"}, // Only voting reps
				},
			},
			Public: true,
		}
		mockRepo.AddMailingList(mailingList)

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		service := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)

		// Update event: Observer -> Voting Rep (should add)
		updateEvent := model.CommitteeMemberUpdatedEvent{
			MemberUID:    "member-789",
			CommitteeUID: "committee-999",
			ProjectUID:   "project-456",
			OldMember: model.CommitteeMemberEventData{
				Email:        "user@example.com",
				VotingStatus: "Observer", // Not in filters
			},
			NewMember: model.CommitteeMemberEventData{
				Email:        "user@example.com",
				VotingStatus: "Voting Rep", // Now matches
			},
		}

		data, err := json.Marshal(updateEvent)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.CommitteeMemberUpdatedSubject,
			Data:    data,
		}

		service.HandleMessage(ctx, msg)

		// Verify member was added
		members := mockRepo.GetMembersForMailingList("test-list-3")
		assert.Equal(t, 1, len(members))
		assert.Equal(t, "user@example.com", members[0].Email)
	})
}
