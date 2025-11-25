// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
	"github.com/nats-io/nats.go"
)

// CommitteeSyncService handles committee member synchronization to mailing lists
// Pattern: mirrors grpsIOWebhookProcessor - ONE file with routing + business logic
type CommitteeSyncService struct {
	mailingListReader port.GrpsIOMailingListReader
	memberWriter      port.GrpsIOMemberWriter
	memberReader      port.GrpsIOMemberReader
}

// NewCommitteeSyncService creates a new committee sync service
func NewCommitteeSyncService(
	mailingListReader port.GrpsIOMailingListReader,
	memberWriter port.GrpsIOMemberWriter,
	memberReader port.GrpsIOMemberReader,
) *CommitteeSyncService {
	return &CommitteeSyncService{
		mailingListReader: mailingListReader,
		memberWriter:      memberWriter,
		memberReader:      memberReader,
	}
}

// HandleMessage routes NATS messages to appropriate handlers based on subject
// Pattern: mirrors grpsIOWebhookProcessor.ProcessEvent but returns error for acknowledgment
func (s *CommitteeSyncService) HandleMessage(ctx context.Context, msg *nats.Msg) error {
	subject := msg.Subject

	slog.DebugContext(ctx, "received committee event", "subject", subject)

	var err error
	switch subject {
	case constants.CommitteeMemberCreatedSubject:
		err = s.handleCreated(ctx, msg)
	case constants.CommitteeMemberDeletedSubject:
		err = s.handleDeleted(ctx, msg)
	case constants.CommitteeMemberUpdatedSubject:
		err = s.handleUpdated(ctx, msg)
	default:
		slog.WarnContext(ctx, "unknown committee event subject", "subject", subject)
		return fmt.Errorf("unknown committee event subject: %s", subject)
	}

	if err != nil {
		slog.ErrorContext(ctx, "error processing committee event",
			"error", err,
			"subject", subject)
		return err
	}

	return nil
}

// handleCreated processes committee member created events
func (s *CommitteeSyncService) handleCreated(ctx context.Context, msg *nats.Msg) error {
	var event model.CommitteeMemberCreatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal committee member created event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	slog.InfoContext(ctx, "processing committee member created event",
		"member_uid", event.MemberUID,
		"committee_uid", event.CommitteeUID,
		"project_uid", event.ProjectUID,
		"email", redaction.RedactEmail(event.Member.Email),
		"voting_status", event.Member.VotingStatus)

	// Find all mailing lists for this committee
	mailingLists, err := s.mailingListReader.GetMailingListsByCommittee(ctx, event.CommitteeUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing lists for committee",
			"error", err,
			"committee_uid", event.CommitteeUID)
		return fmt.Errorf("failed to get mailing lists: %w", err)
	}

	if len(mailingLists) == 0 {
		slog.InfoContext(ctx, "no mailing lists found for committee (nothing to sync)",
			"committee_uid", event.CommitteeUID)
		return nil
	}

	// For each mailing list, check if member should be added based on filters
	for _, ml := range mailingLists {
		// Check if this member's voting status matches the list's filters
		if matchesFilter(event.Member.VotingStatus, ml.CommitteeFilters) {
			slog.InfoContext(ctx, "adding committee member to matching mailing list",
				"mailing_list_uid", ml.UID,
				"group_name", ml.GroupName,
				"email", redaction.RedactEmail(event.Member.Email),
				"voting_status", event.Member.VotingStatus)

			if err := s.addMemberToList(ctx, ml, event.Member); err != nil {
				slog.ErrorContext(ctx, "failed to add member to list",
					"error", err,
					"mailing_list_uid", ml.UID)
				// Continue with other lists even if one fails
				continue
			}
		} else {
			slog.DebugContext(ctx, "member voting status does not match list filters",
				"mailing_list_uid", ml.UID,
				"voting_status", event.Member.VotingStatus,
				"filters", ml.CommitteeFilters)
		}
	}

	slog.InfoContext(ctx, "committee member created event processed successfully",
		"member_uid", event.MemberUID)

	return nil
}

// handleDeleted processes committee member deleted events
func (s *CommitteeSyncService) handleDeleted(ctx context.Context, msg *nats.Msg) error {
	var event model.CommitteeMemberDeletedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal committee member deleted event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	slog.InfoContext(ctx, "processing committee member deleted event",
		"member_uid", event.MemberUID,
		"committee_uid", event.CommitteeUID,
		"project_uid", event.ProjectUID,
		"email", redaction.RedactEmail(event.Email))

	// Find all mailing lists for this committee
	mailingLists, err := s.mailingListReader.GetMailingListsByCommittee(ctx, event.CommitteeUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing lists for committee",
			"error", err,
			"committee_uid", event.CommitteeUID)
		return fmt.Errorf("failed to get mailing lists: %w", err)
	}

	if len(mailingLists) == 0 {
		slog.InfoContext(ctx, "no mailing lists found for committee (nothing to sync)",
			"committee_uid", event.CommitteeUID)
		return nil
	}

	// Remove or convert member from each mailing list
	for _, ml := range mailingLists {
		slog.InfoContext(ctx, "removing committee member from mailing list",
			"mailing_list_uid", ml.UID,
			"group_name", ml.GroupName,
			"email", redaction.RedactEmail(event.Email),
			"public", ml.Public)

		if err := s.removeMemberFromList(ctx, ml, event.Email); err != nil {
			slog.ErrorContext(ctx, "failed to remove member from list",
				"error", err,
				"mailing_list_uid", ml.UID)
			// Continue with other lists even if one fails
			continue
		}
	}

	slog.InfoContext(ctx, "committee member deleted event processed successfully",
		"member_uid", event.MemberUID)

	return nil
}

// handleUpdated processes committee member updated events
func (s *CommitteeSyncService) handleUpdated(ctx context.Context, msg *nats.Msg) error {
	var event model.CommitteeMemberUpdatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal committee member updated event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	slog.InfoContext(ctx, "processing committee member updated event",
		"member_uid", event.MemberUID,
		"committee_uid", event.CommitteeUID,
		"project_uid", event.ProjectUID,
		"old_email", redaction.RedactEmail(event.OldMember.Email),
		"new_email", redaction.RedactEmail(event.NewMember.Email),
		"old_voting_status", event.OldMember.VotingStatus,
		"new_voting_status", event.NewMember.VotingStatus)

	// Check if anything actually changed
	emailChanged := event.OldMember.Email != event.NewMember.Email
	statusChanged := event.OldMember.VotingStatus != event.NewMember.VotingStatus

	if !emailChanged && !statusChanged {
		slog.DebugContext(ctx, "no email or voting status change, skipping sync",
			"member_uid", event.MemberUID)
		return nil
	}

	// Query mailing lists ONCE (performance optimization and race condition prevention)
	mailingLists, err := s.mailingListReader.GetMailingListsByCommittee(ctx, event.CommitteeUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get mailing lists for committee", "error", err)
		return fmt.Errorf("failed to get mailing lists for committee %s: %w", event.CommitteeUID, err)
	}

	if len(mailingLists) == 0 {
		slog.InfoContext(ctx, "no mailing lists found for committee (nothing to sync)",
			"committee_uid", event.CommitteeUID)
		return nil
	}

	// Log what changed for observability
	if emailChanged {
		slog.InfoContext(ctx, "committee member email changed",
			"member_uid", event.MemberUID,
			"old_email", redaction.RedactEmail(event.OldMember.Email),
			"new_email", redaction.RedactEmail(event.NewMember.Email))
	}
	if statusChanged {
		slog.InfoContext(ctx, "committee member voting status changed",
			"member_uid", event.MemberUID,
			"old_status", event.OldMember.VotingStatus,
			"new_status", event.NewMember.VotingStatus)
	}

	// Process each mailing list with consolidated logic
	// This handles all combinations: email-only, status-only, or both changes
	for _, ml := range mailingLists {
		oldMatch := matchesFilter(event.OldMember.VotingStatus, ml.CommitteeFilters)
		newMatch := matchesFilter(event.NewMember.VotingStatus, ml.CommitteeFilters)

		// Determine actions based on combined state
		// Remove old member if: (1) email changed and was in list, OR (2) status changed and no longer matches
		shouldRemove := oldMatch && (emailChanged || !newMatch)
		// Add new member if: (1) email changed and matches filters, OR (2) status changed and now matches
		shouldAdd := newMatch && (emailChanged || !oldMatch)

		if shouldRemove {
			slog.InfoContext(ctx, "removing member from mailing list",
				"mailing_list_uid", ml.UID,
				"email", redaction.RedactEmail(event.OldMember.Email),
				"reason", getRemovalReason(emailChanged, statusChanged))
			if err := s.removeMemberFromList(ctx, ml, event.OldMember.Email); err != nil {
				slog.ErrorContext(ctx, "failed to remove member from list",
					"error", err,
					"mailing_list_uid", ml.UID)
				continue // Continue with other lists even if one fails
			}
		}

		if shouldAdd {
			slog.InfoContext(ctx, "adding member to mailing list",
				"mailing_list_uid", ml.UID,
				"email", redaction.RedactEmail(event.NewMember.Email),
				"reason", getAdditionReason(emailChanged, statusChanged))
			if err := s.addMemberToList(ctx, ml, event.NewMember); err != nil {
				slog.ErrorContext(ctx, "failed to add member to list",
					"error", err,
					"mailing_list_uid", ml.UID)
				continue // Continue with other lists even if one fails
			}
		}

		// If both shouldRemove and shouldAdd are false, no change needed for this list
	}

	slog.InfoContext(ctx, "committee member updated event processed successfully",
		"member_uid", event.MemberUID)

	return nil
}

// getRemovalReason returns a human-readable reason for member removal
func getRemovalReason(emailChanged, statusChanged bool) string {
	if emailChanged && statusChanged {
		return "email_and_status_changed"
	} else if emailChanged {
		return "email_changed"
	}
	return "status_no_longer_matches"
}

// getAdditionReason returns a human-readable reason for member addition
func getAdditionReason(emailChanged, statusChanged bool) string {
	if emailChanged && statusChanged {
		return "new_email_and_status_matches"
	} else if emailChanged {
		return "new_email_matches_filters"
	}
	return "status_now_matches"
}

// addMemberToList adds a committee member to a mailing list
func (s *CommitteeSyncService) addMemberToList(ctx context.Context, mailingList *model.GrpsIOMailingList, memberData model.CommitteeMemberEventData) error {
	// Check if member already exists (idempotency)
	existing, revision, err := s.memberReader.GetMemberByEmail(ctx, mailingList.UID, memberData.Email)
	if err == nil && existing != nil {
		slog.InfoContext(ctx, "member already exists in mailing list (idempotent)",
			"mailing_list_uid", mailingList.UID,
			"email", redaction.RedactEmail(memberData.Email),
			"existing_member_type", existing.MemberType)

		// If existing member is "direct" type, upgrade to "committee"
		if existing.MemberType == "direct" {
			slog.InfoContext(ctx, "upgrading direct member to committee member",
				"member_uid", existing.UID,
				"email", redaction.RedactEmail(memberData.Email))

			existing.MemberType = "committee"
			existing.Source = constants.SourceCommittee
			_, _, err = s.memberWriter.UpdateGrpsIOMember(ctx, existing.UID, existing, revision)
			if err != nil {
				return fmt.Errorf("failed to upgrade member %s from direct to committee type: %w",
					existing.UID,
					err)
			}

			slog.InfoContext(ctx, "member upgraded from direct to committee type",
				"member_uid", existing.UID,
				"mailing_list_uid", mailingList.UID)
		}
		return nil
	}

	// Check for errors other than NotFound
	var notFoundErr errors.NotFound
	if err != nil && !stderrors.As(err, &notFoundErr) {
		return fmt.Errorf("failed to check existing member in list %s for email %s: %w",
			mailingList.UID,
			redaction.RedactEmail(memberData.Email),
			err)
	}

	// Create new committee member
	member := &model.GrpsIOMember{
		UID:            uuid.New().String(),
		MailingListUID: mailingList.UID,
		Source:         constants.SourceCommittee, // Committee sync events
		Username:       memberData.Username,
		FirstName:      memberData.FirstName,
		LastName:       memberData.LastName,
		Email:          memberData.Email,
		Organization:   memberData.Organization.Name,
		JobTitle:       memberData.JobTitle,
		MemberType:     "committee", // Committee members
		DeliveryMode:   "email",     // Default delivery mode
		ModStatus:      "none",      // Always "none" - no role mapping
		Status:         "normal",
	}

	_, _, err = s.memberWriter.CreateGrpsIOMember(ctx, member)
	if err != nil {
		return fmt.Errorf("failed to create committee member in list %s (email: %s): %w",
			mailingList.UID,
			redaction.RedactEmail(memberData.Email),
			err)
	}

	slog.InfoContext(ctx, "committee member added to mailing list",
		"member_uid", member.UID,
		"mailing_list_uid", mailingList.UID,
		"email", redaction.RedactEmail(memberData.Email))

	return nil
}

// removeMemberFromList removes or converts a committee member based on list visibility
func (s *CommitteeSyncService) removeMemberFromList(ctx context.Context, mailingList *model.GrpsIOMailingList, email string) error {
	// Find member by email
	existing, revision, err := s.memberReader.GetMemberByEmail(ctx, mailingList.UID, email)
	if err != nil {
		var notFoundErr errors.NotFound
		if stderrors.As(err, &notFoundErr) {
			slog.InfoContext(ctx, "member not found in mailing list (idempotent)",
				"mailing_list_uid", mailingList.UID,
				"email", redaction.RedactEmail(email))
			return nil // Already removed
		}
		return fmt.Errorf("failed to look up member in list %s for email %s: %w",
			mailingList.UID,
			redaction.RedactEmail(email),
			err)
	}

	// Only process if member type is "committee"
	if existing.MemberType != "committee" {
		slog.InfoContext(ctx, "member is not committee type, skipping",
			"member_uid", existing.UID,
			"member_type", existing.MemberType)
		return nil
	}

	// Public lists: convert to "direct" type
	// Private lists: delete member
	if mailingList.Public {
		slog.InfoContext(ctx, "converting committee member to direct member (public list)",
			"member_uid", existing.UID,
			"mailing_list_uid", mailingList.UID,
			"email", redaction.RedactEmail(email))

		existing.MemberType = "direct"
		_, _, err = s.memberWriter.UpdateGrpsIOMember(ctx, existing.UID, existing, revision)
		if err != nil {
			return fmt.Errorf("failed to convert member %s to direct type in list %s: %w",
				existing.UID,
				mailingList.UID,
				err)
		}
	} else {
		slog.InfoContext(ctx, "deleting committee member (private list)",
			"member_uid", existing.UID,
			"mailing_list_uid", mailingList.UID,
			"email", redaction.RedactEmail(email))

		err = s.memberWriter.DeleteGrpsIOMember(ctx, existing.UID, revision, existing)
		if err != nil {
			return fmt.Errorf("failed to delete committee member %s from list %s: %w",
				existing.UID,
				mailingList.UID,
				err)
		}
	}

	return nil
}

// matchesFilter checks if a voting status matches any of the mailing list's committee filters
func matchesFilter(votingStatus string, filters []string) bool {
	if len(filters) == 0 {
		return false // No filters means no committee members
	}
	return slices.Contains(filters, votingStatus)
}
