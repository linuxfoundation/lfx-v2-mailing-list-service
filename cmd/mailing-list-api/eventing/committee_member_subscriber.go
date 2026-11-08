// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// committeeMemberEventCallTimeout bounds a single linked mailing list's TriggerAutoSync call.
// TriggerAutoSync runs a sync-diff job and then, if there's anything to apply, a sync-apply job —
// each of which can itself block for up to service.SyncJobPollTimeout while ITX processes it (see
// pollSyncDiffJob/pollSyncApplyJob). A 30s budget here was shorter than either job's own polling
// window, so context.WithTimeout silently clipped the *earlier* of the two deadlines: the ITX job
// would still be running when this context expired, and TriggerAutoSync would report a timeout
// failure with no retry, even though the job itself hadn't actually failed. Sized to 2x
// SyncJobPollTimeout (diff then apply, worst case) plus a fixed buffer for the ListMailingLists
// lookup, GetMailingList call, and the cross-list CheckSubscriber guard in between.
const committeeMemberEventCallTimeout = 2*service.SyncJobPollTimeout + 30*time.Second

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

// autoSyncCoalescer wraps an autoSyncTrigger so that the full compute-and-apply reconciliation
// cycle for a given mailing list is serialized per mailing list UID: two reconciliations for the
// same list can never have their ITX compute/apply phases interleaved, whether they originate from
// different committeeMemberSubjects subscriptions (each gets its own delivery goroutine), from two
// different committees linked to the same mailing list, or from a dirty re-run of an in-flight call
// (see below). This also closes the specific race reported on PR #104 by
// @copilot-pull-request-reviewer: a created-event reconciliation computes an add, a deleted-event
// reconciliation for the same list runs to completion as a no-op before the add is applied, and the
// first call then applies its now-stale add afterward, leaving a removed member subscribed. With
// calls for the same list serialized, the deleted-event run either happens entirely before the
// created-event run starts (and is superseded by it, which recomputes from current state), or is
// coalesced into a dirty re-run that happens entirely after the created-event run finishes (also
// recomputing from current state) -- never interleaved with it.
//
// Serializing without a naive blocking mutex matters here because TriggerAutoSync recomputes a full
// ComputeSyncDiff+ApplySyncDiff reconciliation rather than applying a point delta: if a call arrives
// while another is already running for the same list, blocking it until the first finishes and then
// running it anyway would just repeat the same ITX round-trip the first call already made redundant.
// Instead, a call that arrives while one is running marks the in-flight run "dirty" and returns
// immediately; the in-flight run re-runs once more on the dirty call's behalf after it finishes. A
// sync-diff recompute is idempotent and always reflects ITX's current state, so this is always
// enough to pick up whatever the coalesced call would have applied, without needing a queue,
// retry/backoff, or external coordination.
//
// Each attempt (the initial run and every dirty-triggered re-run) gets its own fresh
// committeeMemberEventCallTimeout budget off the caller's ctx, rather than the whole loop sharing one
// deadline computed from when the first attempt started: that budget is sized for exactly one
// TriggerAutoSync call, so a re-run sharing the first call's already-ticking (and possibly
// near-exhausted) deadline could itself be starved mid-poll -- silently dropping the very update
// this coalescer exists to rescue, which is the same "timed out, not retried" failure mode the
// committeeMemberEventCallTimeout sizing fix (see its doc comment) closed for a single call.
//
// Cross-replica note: this is an in-process mutex, not a distributed lock, and only guards against
// interleaving within a single process. charts/lfx-v2-mailing-list-service/values.yaml pins
// replicaCount to 1, which bounds the *steady-state* number of processes handling committee_member
// events to one -- but the Deployment has no explicit rollout strategy, so a default Kubernetes
// RollingUpdate can still run the old and new pod concurrently for the duration of a deploy. During
// that overlap window, two processes each with their own coalescer can reconcile the same mailing
// list at the same time, so this guarantee is single-process only, not an absolute guarantee that
// only one process ever runs. If this service is ever run with replicaCount > 1 (or needs the
// rollout overlap window closed), this in-process coalescing alone would not prevent two different
// replicas from running the compute/apply cycle for the same mailing list concurrently (the
// committeeMemberQueueGroup only balances load within a single subject's deliveries, it does not
// serialize across the three committeeMemberSubjects or across replicas for the same list); a
// distributed lock or single-writer design would be needed at that point.
type autoSyncCoalescer struct {
	next autoSyncTrigger

	mu    sync.Mutex
	state map[string]*coalescerState
}

// coalescerState tracks, per mailing list, whether a TriggerAutoSync call is currently running and
// whether another call arrived while it was running.
type coalescerState struct {
	running bool
	dirty   bool
}

// newAutoSyncCoalescer wraps next so TriggerAutoSync calls are coalesced per mailing list.
func newAutoSyncCoalescer(next autoSyncTrigger) *autoSyncCoalescer {
	return &autoSyncCoalescer{
		next:  next,
		state: make(map[string]*coalescerState),
	}
}

// TriggerAutoSync runs next.TriggerAutoSync for mailingListID. If a call for that same list is
// already running (on another goroutine), this call marks it dirty and returns nil immediately
// instead of racing it -- the running call re-runs once more after it finishes, so the dirty call's
// effect is not lost. ctx is the long-lived, caller-scoped parent context (not pre-bounded by
// committeeMemberEventCallTimeout): each attempt -- the initial run and every dirty-triggered
// re-run -- derives its own fresh committeeMemberEventCallTimeout deadline from it, so a re-run is
// never starved by a deadline that was already ticking down during a previous attempt.
func (c *autoSyncCoalescer) TriggerAutoSync(ctx context.Context, mailingListID string) error {
	c.mu.Lock()
	st, ok := c.state[mailingListID]
	if !ok {
		st = &coalescerState{}
		c.state[mailingListID] = st
	}
	if st.running {
		st.dirty = true
		c.mu.Unlock()
		return nil
	}
	st.running = true
	c.mu.Unlock()

	var err error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, committeeMemberEventCallTimeout)
		err = c.next.TriggerAutoSync(attemptCtx, mailingListID)
		cancel()

		c.mu.Lock()
		if !st.dirty {
			st.running = false
			c.mu.Unlock()
			break
		}
		st.dirty = false
		c.mu.Unlock()
	}
	return err
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
		autoSyncer:        newAutoSyncCoalescer(autoSyncer),
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

// stopSubs starts draining every subscription before waiting on any of them, so the shared deadline
// is spent waiting on all subscriptions concurrently rather than sequentially. A per-subscription
// drain-then-wait loop would let one slow subscription consume the whole deadline before the next
// subscription's Drain() is even called — leaving it undrained and still accepting queue-group
// deliveries for the entire shutdown window.
func (s *CommitteeMemberSubscriber) stopSubs(deadlineCtx context.Context) {
	deadline := time.Now().Add(committeeMemberDrainTimeout)
	if d, ok := deadlineCtx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	draining := make([]*nats.Subscription, 0, len(s.subs))
	for _, sub := range s.subs {
		if err := sub.Drain(); err != nil {
			s.logger.Warn("error draining committee_member subscription", "subject", sub.Subject, "error", err)
			continue
		}
		draining = append(draining, sub)
	}

	for {
		stillDraining := false
		for _, sub := range draining {
			if sub.IsDraining() {
				stillDraining = true
				break
			}
		}
		if !stillDraining || !time.Now().Before(deadline) {
			return
		}
		time.Sleep(10 * time.Millisecond)
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
// single shared window. That per-list budget is applied by autoSyncer itself (see autoSyncCoalescer),
// not here: autoSyncer may coalesce this call into a dirty re-run of an already-in-flight call for
// the same list, and that re-run needs its own fresh budget rather than inheriting whatever remains
// of a deadline bound before this call was even made.
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
		err := autoSyncer.TriggerAutoSync(ctx, ml.UID)
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
