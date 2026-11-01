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

// committeeMemberListTimeout bounds the ListMailingLists lookup that resolves a committee's
// linked mailing lists, separately from the per-list committeeMemberEventCallTimeout budget spent
// on each list's own TriggerAutoSync call.
const committeeMemberListTimeout = 10 * time.Second

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
			s.stopSubs(context.Background())
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

// committeeMemberDrainTimeout bounds, per subscription, how long Stop waits for an in-flight
// handler to finish draining before moving on and cancelling the context regardless, so a stuck
// drain cannot hang shutdown forever. Set comfortably above committeeMemberEventCallTimeout (the
// budget for a single linked list's TriggerAutoSync call) so a handler that is partway through
// one list's ITX round-trip when Stop is called is not routinely cut off by the drain deadline
// itself; it is not a guarantee that every list in a multi-list batch finishes draining.
const committeeMemberDrainTimeout = committeeMemberEventCallTimeout + 10*time.Second

// Stop drains all subscriptions (allowing an in-flight handler to finish its current list, up to
// committeeMemberDrainTimeout, further bounded by deadlineCtx's own deadline if it is sooner), then
// cancels the context. deadlineCtx should be the same shutdown deadline shared with the HTTP server
// shutdown, so this drain cannot by itself run the process past the pod's termination grace period
// — pass context.Background() to fall back to committeeMemberDrainTimeout alone.
// Drain must precede cancel so an in-flight TriggerAutoSync call is not aborted mid-ITX-job-poll by
// context cancellation: *nats.Subscription.Drain only *starts* an asynchronous drain and returns
// immediately, so stopSubs polls IsDraining until it clears (or the deadline elapses) instead of
// treating Drain's return as completion.
func (s *CommitteeMemberSubscriber) Stop(deadlineCtx context.Context) {
	s.stopSubs(deadlineCtx)
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *CommitteeMemberSubscriber) stopSubs(deadlineCtx context.Context) {
	deadline := time.Now().Add(committeeMemberDrainTimeout)
	if d, ok := deadlineCtx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	for _, sub := range s.subs {
		if err := sub.Drain(); err != nil {
			s.logger.Warn("error draining committee_member subscription", "subject", sub.Subject, "error", err)
			continue
		}
		for sub.IsDraining() && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *CommitteeMemberSubscriber) handle(msg *nats.Msg) {
	committeeUID, err := committeeUIDFromEvent(msg.Data)
	if err != nil {
		s.logger.Warn("failed to parse committee_member event; discarding", "subject", msg.Subject, "error", err)
		return
	}
	if committeeUID == "" {
		s.logger.Warn("committee_member event missing committee_uid; discarding", "subject", msg.Subject)
		return
	}

	if err := processCommitteeMemberEvent(s.ctx, committeeUID, s.mailingListReader, s.autoSyncer, s.logger); err != nil {
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
//
// committee-api's CommitteeEvent envelope (committee_message.go) carries the actual payload
// nested under "data", not at the top level: created/deleted events flatten the member (and so
// committee_uid) directly under data, while the updated event nests the member (and its
// committee_uid) one level deeper under data.member — see CommitteeMemberCreatedEventData/
// CommitteeMemberDeletedEventData (embedded *CommitteeMember) vs CommitteeMemberUpdateEventData
// (MemberUID/OldMember/Member) in that file.
func committeeUIDFromEvent(data []byte) (string, error) {
	var envelope struct {
		Data struct {
			CommitteeUID string `json:"committee_uid"`
			Member       *struct {
				CommitteeUID string `json:"committee_uid"`
			} `json:"member"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}
	if envelope.Data.CommitteeUID != "" {
		return envelope.Data.CommitteeUID, nil
	}
	if envelope.Data.Member != nil {
		return envelope.Data.Member.CommitteeUID, nil
	}
	return "", nil
}

// processCommitteeMemberEvent resolves committeeUID's linked mailing lists and triggers an
// auto-sync reconciliation for each one configured with sync_mode "auto_c_to_ml". Lists with a
// different or absent sync_mode are left alone: TriggerAutoSync's own no-op path would also skip
// them, but filtering here avoids an unnecessary ITX round-trip for lists this event cannot
// possibly affect, and keeps this handler's effect scoped to the issue's committee-to-mailing-list
// direction rather than also quietly reconciling an unrelated ml_to_c list's drift.
//
// Each linked list gets its own committeeMemberEventCallTimeout budget rather than sharing one
// deadline for the whole batch: a committee linked to several mailing lists would otherwise have
// its later lists starved (or outright skipped) by the earlier ones' ITX round-trips eating into a
// single shared window.
func processCommitteeMemberEvent(
	ctx context.Context,
	committeeUID string,
	mailingListReader port.GroupsIOMailingListReader,
	autoSyncer autoSyncTrigger,
	logger *slog.Logger,
) error {
	listLookupCtx, listLookupCancel := context.WithTimeout(ctx, committeeMemberListTimeout)
	linkedLists, _, err := mailingListReader.ListMailingLists(listLookupCtx, "", committeeUID)
	listLookupCancel()
	if err != nil {
		return err
	}

	var firstErr error
	for _, ml := range linkedLists {
		if len(ml.Committees) == 0 || ml.Committees[0].SyncMode != model.SyncModeAutoCommitteeToMailingList {
			continue
		}
		listCtx, cancel := context.WithTimeout(ctx, committeeMemberEventCallTimeout)
		err := autoSyncer.TriggerAutoSync(listCtx, ml.UID)
		cancel()
		if err != nil {
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
