// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

const committeeMemberEventCallTimeout = 30 * time.Second

// committeeMemberQueueGroup is the NATS queue group used by the mailing list service for
// committee-member event subscriptions, so only one replica of this service processes a given
// event.
const committeeMemberQueueGroup = "mailing-list-service-committee-member-events"

// autoSyncTrigger is the subset of RealtimeAutoSyncOrchestrator's behavior the subscriber needs,
// kept as an interface so the subscriber can be unit tested without the full orchestrator.
type autoSyncTrigger interface {
	TriggerAutoSync(ctx context.Context, mailingListID string) error
}

// CommitteeMemberSubscriber subscribes to committee-api's committee_member created/updated/deleted
// events (see pkg/constants.CommitteeMemberCreatedSubject et al.) and, for each mailing list
// linked to the affected committee with sync_mode "auto_c_to_ml", triggers an immediate
// reconciliation so the membership change is reflected on the mailing list without a manual
// "Sync Now" (issue #4023).
type CommitteeMemberSubscriber struct {
	nc                natsQueueSubscriber
	mailingListReader port.GroupsIOMailingListReader
	autoSyncer        autoSyncTrigger
	logger            *slog.Logger
	subs              []*nats.Subscription

	ctx    context.Context
	cancel context.CancelFunc
}

// NewCommitteeMemberSubscriber creates a new subscriber but does not start it.
func NewCommitteeMemberSubscriber(
	nc natsQueueSubscriber,
	mailingListReader port.GroupsIOMailingListReader,
	autoSyncer autoSyncTrigger,
	logger *slog.Logger,
) *CommitteeMemberSubscriber {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommitteeMemberSubscriber{
		nc:                nc,
		mailingListReader: mailingListReader,
		autoSyncer:        autoSyncer,
		logger:            logger,
	}
}

// committeeMemberSubjects lists the subjects this subscriber reacts to. All three event types
// (created/updated/deleted) are relevant: a created or deleted member can add or remove a
// mailing-list subscription, and an updated member can change voting status in a way that moves
// them in or out of the committee's AllowedVotingStatuses filter.
var committeeMemberSubjects = []string{
	constants.CommitteeMemberCreatedSubject,
	constants.CommitteeMemberDeletedSubject,
	constants.CommitteeMemberUpdatedSubject,
}

// Start registers a NATS QueueSubscribe for each committee-member event subject.
func (s *CommitteeMemberSubscriber) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	for _, subject := range committeeMemberSubjects {
		sub, err := s.nc.QueueSubscribe(subject, committeeMemberQueueGroup, s.handle)
		if err != nil {
			s.stopSubs()
			if s.cancel != nil {
				s.cancel()
			}
			return err
		}
		s.subs = append(s.subs, sub)
	}
	s.logger.Info("committee_member subscriber started",
		"subjects", committeeMemberSubjects,
		"queue_group", committeeMemberQueueGroup,
	)
	return nil
}

// Stop drains all subscriptions (allowing in-flight handlers to complete), then cancels the
// context. Drain must precede cancel so an in-flight TriggerAutoSync call is not aborted mid-ITX-
// job-poll by context cancellation.
func (s *CommitteeMemberSubscriber) Stop() {
	s.stopSubs()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *CommitteeMemberSubscriber) stopSubs() {
	for _, sub := range s.subs {
		if err := sub.Drain(); err != nil {
			s.logger.Warn("error draining committee_member subscription", "subject", sub.Subject, "error", err)
		}
	}
}

func (s *CommitteeMemberSubscriber) handle(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(s.ctx, committeeMemberEventCallTimeout)
	defer cancel()

	committeeUID, err := committeeUIDFromEvent(msg.Data)
	if err != nil {
		s.logger.Warn("failed to parse committee_member event; discarding", "subject", msg.Subject, "error", err)
		return
	}
	if committeeUID == "" {
		s.logger.Warn("committee_member event missing committee_uid; discarding", "subject", msg.Subject)
		return
	}

	if err := processCommitteeMemberEvent(ctx, committeeUID, s.mailingListReader, s.autoSyncer, s.logger); err != nil {
		s.logger.Warn("committee_member auto-sync failed; best-effort, not retrying",
			"error", err,
			"committee_uid", committeeUID,
			"subject", msg.Subject,
		)
	}
}

// committeeUIDFromEvent extracts the committee_uid common to all three committee-member event
// payloads without needing to know which of the three shapes applies: TriggerAutoSync
// recomputes a full sync-diff rather than hand-resolving the single changed member, so the
// per-member fields in the event (email, voting status, ...) are not needed here.
func committeeUIDFromEvent(data []byte) (string, error) {
	var envelope struct {
		CommitteeUID string `json:"committee_uid"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}
	return envelope.CommitteeUID, nil
}

// processCommitteeMemberEvent resolves committeeUID's linked mailing lists and triggers an
// auto-sync reconciliation for each one configured with sync_mode "auto_c_to_ml". Lists with a
// different or absent sync_mode are left alone: TriggerAutoSync's own no-op path would also skip
// them, but filtering here avoids an unnecessary ITX round-trip for lists this event cannot
// possibly affect, and keeps this handler's effect scoped to the issue's committee-to-mailing-list
// direction rather than also quietly reconciling an unrelated ml_to_c list's drift.
func processCommitteeMemberEvent(
	ctx context.Context,
	committeeUID string,
	mailingListReader port.GroupsIOMailingListReader,
	autoSyncer autoSyncTrigger,
	logger *slog.Logger,
) error {
	linkedLists, _, err := mailingListReader.ListMailingLists(ctx, "", committeeUID)
	if err != nil {
		return err
	}

	var firstErr error
	for _, ml := range linkedLists {
		if len(ml.Committees) == 0 || ml.Committees[0].SyncMode != model.SyncModeAutoCommitteeToMailingList {
			continue
		}
		if err := autoSyncer.TriggerAutoSync(ctx, ml.UID); err != nil {
			logger.Warn("auto-sync failed for mailing list linked to committee",
				"mailing_list_id", ml.UID,
				"committee_uid", committeeUID,
				"error", err,
			)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		logger.Info("committee_member event triggered auto-sync",
			"mailing_list_id", ml.UID,
			"committee_uid", committeeUID,
		)
	}
	return firstErr
}
