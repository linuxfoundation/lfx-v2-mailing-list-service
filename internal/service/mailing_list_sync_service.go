// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go"
)

// MailingListSyncService handles mailing list creation/update events and syncs committee members
// Pattern: Delegates committee member operations to CommitteeSyncService for reusability
type MailingListSyncService struct {
	committeeSyncService *CommitteeSyncService
}

// NewMailingListSyncService creates a new mailing list sync service
func NewMailingListSyncService(
	committeeSyncService *CommitteeSyncService,
) *MailingListSyncService {
	return &MailingListSyncService{
		committeeSyncService: committeeSyncService,
	}
}

// HandleMessage routes NATS messages to appropriate handlers based on subject
func (s *MailingListSyncService) HandleMessage(ctx context.Context, msg *nats.Msg) error {
	subject := msg.Subject

	slog.DebugContext(ctx, "received mailing list event", "subject", subject)

	var err error
	switch subject {
	case constants.MailingListCreatedSubject:
		err = s.handleCreated(ctx, msg)
	case constants.MailingListUpdatedSubject:
		err = s.handleUpdated(ctx, msg)
	default:
		slog.WarnContext(ctx, "unknown mailing list event subject", "subject", subject)
		return fmt.Errorf("unknown mailing list event subject: %s", subject)
	}

	if err != nil {
		slog.ErrorContext(ctx, "error processing mailing list event",
			"error", err,
			"subject", subject)
		return err
	}

	return nil
}

// handleCreated processes mailing list created events and syncs all committee members
func (s *MailingListSyncService) handleCreated(ctx context.Context, msg *nats.Msg) error {
	var event model.MailingListCreatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal mailing list created event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	if event.MailingList == nil {
		slog.ErrorContext(ctx, "mailing list is nil in created event")
		return fmt.Errorf("mailing list is nil in created event")
	}

	mailingList := event.MailingList

	slog.InfoContext(ctx, "processing mailing list created event",
		"mailing_list_uid", mailingList.UID,
		"group_name", mailingList.GroupName,
		"committee_count", len(mailingList.Committees))

	// If no committees, nothing to sync
	if len(mailingList.Committees) == 0 {
		slog.DebugContext(ctx, "mailing list has no committees, skipping member sync",
			"mailing_list_uid", mailingList.UID)
		return nil
	}

	// Sync members for each committee - delegate to CommitteeSyncService
	for _, committee := range mailingList.Committees {
		if err := s.committeeSyncService.SyncCommitteeMembersToMailingList(ctx, mailingList, committee); err != nil {
			slog.ErrorContext(ctx, "failed to sync committee members",
				"error", err,
				"mailing_list_uid", mailingList.UID,
				"committee_uid", committee.UID)
			// Continue with other committees even if one fails
			continue
		}
	}

	slog.InfoContext(ctx, "mailing list created event processed successfully",
		"mailing_list_uid", mailingList.UID)

	return nil
}

// handleUpdated processes mailing list updated events and syncs committee member changes
func (s *MailingListSyncService) handleUpdated(ctx context.Context, msg *nats.Msg) error {
	var event model.MailingListUpdatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal mailing list updated event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	if event.OldMailingList == nil || event.NewMailingList == nil {
		slog.ErrorContext(ctx, "old or new mailing list is nil in updated event")
		return fmt.Errorf("old or new mailing list is nil in updated event")
	}

	oldML := event.OldMailingList
	newML := event.NewMailingList

	slog.InfoContext(ctx, "processing mailing list updated event",
		"mailing_list_uid", newML.UID,
		"group_name", newML.GroupName,
		"old_committee_count", len(oldML.Committees),
		"new_committee_count", len(newML.Committees))

	// Detect committee changes
	addedCommittees, removedCommittees, modifiedCommittees := detectCommitteeChanges(oldML.Committees, newML.Committees)

	slog.DebugContext(ctx, "detected committee changes",
		"mailing_list_uid", newML.UID,
		"added", len(addedCommittees),
		"removed", len(removedCommittees),
		"modified", len(modifiedCommittees))

	// Handle removed committees - remove their members
	for _, committee := range removedCommittees {
		if err := s.committeeSyncService.RemoveCommitteeMembersFromMailingList(ctx, newML, committee); err != nil {
			slog.ErrorContext(ctx, "failed to remove committee members",
				"error", err,
				"committee_uid", committee.UID)
			continue
		}
	}

	// Handle added committees - add their members
	for _, committee := range addedCommittees {
		if err := s.committeeSyncService.SyncCommitteeMembersToMailingList(ctx, newML, committee); err != nil {
			slog.ErrorContext(ctx, "failed to sync new committee members",
				"error", err,
				"committee_uid", committee.UID)
			continue
		}
	}

	// Handle modified committees - resync members (filters may have changed)
	for _, change := range modifiedCommittees {
		if err := s.committeeSyncService.ResyncCommitteeMembersForMailingList(ctx, newML, change.old, change.new); err != nil {
			slog.ErrorContext(ctx, "failed to resync modified committee members",
				"error", err,
				"committee_uid", change.new.UID)
			continue
		}
	}

	slog.InfoContext(ctx, "mailing list updated event processed successfully",
		"mailing_list_uid", newML.UID)

	return nil
}

// committeeChange represents a change in committee configuration
type committeeChange struct {
	old model.Committee
	new model.Committee
}

// detectCommitteeChanges compares old and new committee arrays and returns added, removed, and modified committees
func detectCommitteeChanges(oldCommittees, newCommittees []model.Committee) (added, removed []model.Committee, modified []committeeChange) {
	// Build maps for easy lookup
	oldMap := make(map[string]model.Committee)
	for _, c := range oldCommittees {
		oldMap[c.UID] = c
	}

	newMap := make(map[string]model.Committee)
	for _, c := range newCommittees {
		newMap[c.UID] = c
	}

	// Find added and modified committees
	for uid, newCommittee := range newMap {
		if oldCommittee, exists := oldMap[uid]; exists {
			// Committee exists in both - check if filters changed
			if !slices.Equal(oldCommittee.AllowedVotingStatuses, newCommittee.AllowedVotingStatuses) {
				modified = append(modified, committeeChange{old: oldCommittee, new: newCommittee})
			}
		} else {
			// Committee only in new - added
			added = append(added, newCommittee)
		}
	}

	// Find removed committees
	for uid, oldCommittee := range oldMap {
		if _, exists := newMap[uid]; !exists {
			removed = append(removed, oldCommittee)
		}
	}

	return added, removed, modified
}
