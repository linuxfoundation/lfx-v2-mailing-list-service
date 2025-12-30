// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailingListSyncService_HandleMessage(t *testing.T) {
	ctx := context.Background()

	// Setup mock repository
	mockRepo := mock.NewMockRepository()
	mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
	memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
	memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
	entityReader := mock.NewMockEntityAttributeReader(mockRepo)

	// Create services
	committeeSyncService := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)
	service := NewMailingListSyncService(committeeSyncService)
	require.NotNil(t, service)

	t.Run("handles created event", func(t *testing.T) {
		mailingList := &model.GrpsIOMailingList{
			UID:        "ml-created-1",
			GroupName:  "test-created",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Test Committee",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: true,
		}

		event := model.MailingListCreatedEvent{
			MailingList: mailingList,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListCreatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.NoError(t, err)
	})

	t.Run("handles created event with no committees", func(t *testing.T) {
		mailingList := &model.GrpsIOMailingList{
			UID:        "ml-created-2",
			GroupName:  "test-no-committees",
			ServiceUID: "service-1",
			Committees: []model.Committee{},
			Public:     true,
		}

		event := model.MailingListCreatedEvent{
			MailingList: mailingList,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListCreatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.NoError(t, err)
	})

	t.Run("handles updated event", func(t *testing.T) {
		oldMailingList := &model.GrpsIOMailingList{
			UID:        "ml-updated-1",
			GroupName:  "test-updated",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: true,
		}

		newMailingList := &model.GrpsIOMailingList{
			UID:        "ml-updated-1",
			GroupName:  "test-updated",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep", "Observer"}, // Modified filters
				},
				{
					UID:                   "committee-2",
					Name:                  "Committee 2",
					AllowedVotingStatuses: []string{"Voting Rep"}, // Added committee
				},
			},
			Public: true,
		}

		event := model.MailingListUpdatedEvent{
			OldMailingList: oldMailingList,
			NewMailingList: newMailingList,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListUpdatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.NoError(t, err)
	})

	t.Run("handles unknown subject", func(t *testing.T) {
		msg := &nats.Msg{
			Subject: "unknown.subject",
			Data:    []byte("{}"),
		}

		err := service.HandleMessage(ctx, msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown mailing list event subject")
	})

	t.Run("handles invalid JSON for created event", func(t *testing.T) {
		msg := &nats.Msg{
			Subject: constants.MailingListCreatedSubject,
			Data:    []byte("invalid json"),
		}

		err := service.HandleMessage(ctx, msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})

	t.Run("handles invalid JSON for updated event", func(t *testing.T) {
		msg := &nats.Msg{
			Subject: constants.MailingListUpdatedSubject,
			Data:    []byte("invalid json"),
		}

		err := service.HandleMessage(ctx, msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})

	t.Run("handles nil mailing list in created event", func(t *testing.T) {
		event := model.MailingListCreatedEvent{
			MailingList: nil,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListCreatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mailing list is nil")
	})

	t.Run("handles nil mailing lists in updated event", func(t *testing.T) {
		event := model.MailingListUpdatedEvent{
			OldMailingList: nil,
			NewMailingList: nil,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListUpdatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "old or new mailing list is nil")
	})
}

func TestDetectCommitteeChanges(t *testing.T) {
	t.Run("detects added committees", func(t *testing.T) {
		oldCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
		}

		newCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
			{UID: "committee-3", Name: "Committee 3", AllowedVotingStatuses: []string{"Voting Rep"}},
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 2)
		assert.Len(t, removed, 0)
		assert.Len(t, modified, 0)

		// Verify added committees
		addedUIDs := make([]string, len(added))
		for i, c := range added {
			addedUIDs[i] = c.UID
		}
		assert.Contains(t, addedUIDs, "committee-2")
		assert.Contains(t, addedUIDs, "committee-3")
	})

	t.Run("detects removed committees", func(t *testing.T) {
		oldCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
			{UID: "committee-3", Name: "Committee 3", AllowedVotingStatuses: []string{"Voting Rep"}},
		}

		newCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 0)
		assert.Len(t, removed, 2)
		assert.Len(t, modified, 0)

		// Verify removed committees
		removedUIDs := make([]string, len(removed))
		for i, c := range removed {
			removedUIDs[i] = c.UID
		}
		assert.Contains(t, removedUIDs, "committee-2")
		assert.Contains(t, removedUIDs, "committee-3")
	})

	t.Run("detects modified committees - filter changes", func(t *testing.T) {
		oldCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
		}

		newCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep", "Observer"}}, // Modified
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},              // Unchanged
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 0)
		assert.Len(t, removed, 0)
		assert.Len(t, modified, 1)

		assert.Equal(t, "committee-1", modified[0].old.UID)
		assert.Equal(t, "committee-1", modified[0].new.UID)
		assert.Equal(t, []string{"Voting Rep"}, modified[0].old.AllowedVotingStatuses)
		assert.Equal(t, []string{"Voting Rep", "Observer"}, modified[0].new.AllowedVotingStatuses)
	})

	t.Run("detects multiple changes", func(t *testing.T) {
		oldCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
			{UID: "committee-3", Name: "Committee 3", AllowedVotingStatuses: []string{"Voting Rep"}},
		}

		newCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep", "Observer"}}, // Modified
			{UID: "committee-4", Name: "Committee 4", AllowedVotingStatuses: []string{"Voting Rep"}},             // Added
			// committee-2 and committee-3 removed
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 1)
		assert.Len(t, removed, 2)
		assert.Len(t, modified, 1)

		assert.Equal(t, "committee-4", added[0].UID)
		assert.Equal(t, "committee-1", modified[0].new.UID)

		removedUIDs := make([]string, len(removed))
		for i, c := range removed {
			removedUIDs[i] = c.UID
		}
		assert.Contains(t, removedUIDs, "committee-2")
		assert.Contains(t, removedUIDs, "committee-3")
	})

	t.Run("empty old committees - all added", func(t *testing.T) {
		oldCommittees := []model.Committee{}

		newCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 2)
		assert.Len(t, removed, 0)
		assert.Len(t, modified, 0)
	})

	t.Run("empty new committees - all removed", func(t *testing.T) {
		oldCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
		}

		newCommittees := []model.Committee{}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 0)
		assert.Len(t, removed, 2)
		assert.Len(t, modified, 0)
	})

	t.Run("no changes", func(t *testing.T) {
		committees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
		}

		added, removed, modified := detectCommitteeChanges(committees, committees)

		assert.Len(t, added, 0)
		assert.Len(t, removed, 0)
		assert.Len(t, modified, 0)
	})

	t.Run("order changes only - no functional change", func(t *testing.T) {
		oldCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
		}

		newCommittees := []model.Committee{
			{UID: "committee-2", Name: "Committee 2", AllowedVotingStatuses: []string{"Observer"}},
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 0)
		assert.Len(t, removed, 0)
		assert.Len(t, modified, 0)
	})

	t.Run("filter order changes are detected as modification", func(t *testing.T) {
		oldCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep", "Observer"}},
		}

		newCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Observer", "Voting Rep"}},
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 0)
		assert.Len(t, removed, 0)
		// slices.Equal is order-sensitive, so this should detect a change
		assert.Len(t, modified, 1)
	})

	t.Run("nil committees are treated as empty", func(t *testing.T) {
		var oldCommittees []model.Committee = nil

		newCommittees := []model.Committee{
			{UID: "committee-1", Name: "Committee 1", AllowedVotingStatuses: []string{"Voting Rep"}},
		}

		added, removed, modified := detectCommitteeChanges(oldCommittees, newCommittees)

		assert.Len(t, added, 1)
		assert.Len(t, removed, 0)
		assert.Len(t, modified, 0)
	})
}

// TestMailingListSyncService_Integration tests the full flow with mailing list events
// Note: These tests verify that event handling completes without errors.
// Full member synchronization is tested in TestCommitteeSyncService_IntegrationWithMailingLists
func TestMailingListSyncService_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("created event with committees completes without error", func(t *testing.T) {
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		mailingList := &model.GrpsIOMailingList{
			UID:        "ml-integration-1",
			GroupName:  "test-integration",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Test Committee",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: true,
		}

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		committeeSyncService := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)
		service := NewMailingListSyncService(committeeSyncService)

		event := model.MailingListCreatedEvent{
			MailingList: mailingList,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListCreatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.NoError(t, err)
	})

	t.Run("updated event handles committee additions without error", func(t *testing.T) {
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		oldMailingList := &model.GrpsIOMailingList{
			UID:        "ml-integration-2",
			GroupName:  "test-update",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: true,
		}

		newMailingList := &model.GrpsIOMailingList{
			UID:        "ml-integration-2",
			GroupName:  "test-update",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
				{
					UID:                   "committee-2",
					Name:                  "Committee 2",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: true,
		}

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		committeeSyncService := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)
		service := NewMailingListSyncService(committeeSyncService)

		event := model.MailingListUpdatedEvent{
			OldMailingList: oldMailingList,
			NewMailingList: newMailingList,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListUpdatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.NoError(t, err)
	})

	t.Run("updated event handles committee removals without error", func(t *testing.T) {
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		oldMailingList := &model.GrpsIOMailingList{
			UID:        "ml-integration-3",
			GroupName:  "test-removal",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
				{
					UID:                   "committee-2",
					Name:                  "Committee 2",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: false, // Private list
		}

		newMailingList := &model.GrpsIOMailingList{
			UID:        "ml-integration-3",
			GroupName:  "test-removal",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep"},
				},
			},
			Public: false,
		}

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		committeeSyncService := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)
		service := NewMailingListSyncService(committeeSyncService)

		event := model.MailingListUpdatedEvent{
			OldMailingList: oldMailingList,
			NewMailingList: newMailingList,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListUpdatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.NoError(t, err)
	})

	t.Run("updated event handles filter modifications without error", func(t *testing.T) {
		mockRepo := mock.NewMockRepository()
		mockRepo.ClearAll()

		oldMailingList := &model.GrpsIOMailingList{
			UID:        "ml-integration-4",
			GroupName:  "test-filter-change",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep"}, // Only voting reps
				},
			},
			Public: true,
		}

		newMailingList := &model.GrpsIOMailingList{
			UID:        "ml-integration-4",
			GroupName:  "test-filter-change",
			ServiceUID: "service-1",
			Committees: []model.Committee{
				{
					UID:                   "committee-1",
					Name:                  "Committee 1",
					AllowedVotingStatuses: []string{"Voting Rep", "Observer"}, // Added observers
				},
			},
			Public: true,
		}

		mailingListReader := mock.NewMockGrpsIOReader(mockRepo)
		memberWriter := mock.NewMockGrpsIOMemberWriter(mockRepo)
		memberReader := mock.NewMockGrpsIOMemberReader(mockRepo)
		entityReader := mock.NewMockEntityAttributeReader(mockRepo)

		committeeSyncService := NewCommitteeSyncService(mailingListReader, memberWriter, memberReader, entityReader)
		service := NewMailingListSyncService(committeeSyncService)

		event := model.MailingListUpdatedEvent{
			OldMailingList: oldMailingList,
			NewMailingList: newMailingList,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		msg := &nats.Msg{
			Subject: constants.MailingListUpdatedSubject,
			Data:    data,
		}

		err = service.HandleMessage(ctx, msg)
		assert.NoError(t, err)
	})
}
