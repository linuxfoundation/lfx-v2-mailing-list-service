// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// SyncApplyOrchestrator implements port.SyncApplier by delegating the actual apply to ITX's own
// sync-apply job (itx-service-groupsio): ITX resolves and adds/removes Groups.io subscribers
// directly, so this service no longer needs its own LFX-account resolution, invite fallback, or
// cross-list removal guard — ITX's own validation (see CheckSyncModeConflict in
// groups_subgroups_sync.go) supersedes it.
type SyncApplyOrchestrator struct {
	mailingListReader port.GroupsIOMailingListReader
	itxSyncClient     port.ITXSyncClient
	accessChecker     port.AccessChecker
	translator        port.Translator
}

// SyncApplyOrchestratorOption configures a SyncApplyOrchestrator.
type SyncApplyOrchestratorOption func(*SyncApplyOrchestrator)

// WithSyncApplyMailingListReader sets the reader used to resolve the mailing list's linked
// committee UID and configured sync_mode.
func WithSyncApplyMailingListReader(r port.GroupsIOMailingListReader) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.mailingListReader = r }
}

// WithSyncApplyITXSyncClient sets the client used to start and poll ITX's sync-apply job.
func WithSyncApplyITXSyncClient(c port.ITXSyncClient) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.itxSyncClient = c }
}

// WithSyncApplyAccessChecker sets the checker used to verify the caller holds writer access on
// the mailing list's linked committee before ITX applies changes on the service's behalf — the
// ITX call itself uses service-level M2M credentials, so this is the only place that can tell a
// caller with mailing-list writer access but no committee access apart from one with both.
func WithSyncApplyAccessChecker(c port.AccessChecker) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.accessChecker = c }
}

// WithSyncApplyTranslator sets the ID translator used to compare ITX's job-reported committee ID
// (v1) against the mailing list's linked committee UID (v2) in the job-completion recheck below.
func WithSyncApplyTranslator(t port.Translator) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.translator = t }
}

// NewSyncApplyOrchestrator creates a SyncApplyOrchestrator with the given options.
func NewSyncApplyOrchestrator(opts ...SyncApplyOrchestratorOption) port.SyncApplier {
	o := &SyncApplyOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// preparedSyncApply is the outcome of validating an apply request and resolving everything needed
// to submit it to ITX, shared by ApplySyncDiff and StartSyncApply.
type preparedSyncApply struct {
	committeeUID         string
	direction            string
	approvedAddEmails    []string
	approvedRemoveEmails []string
	// addByNormalized/removeByNormalized are needed only by ApplySyncDiff, to reconstruct rich
	// per-entry results (fromSyncApplyJob) from ITX's flat email/action/status results —
	// StartSyncApply has no use for them, since GetSyncApplyStatus does not attempt that
	// reconstruction (see its doc comment).
	addByNormalized    map[string]model.SyncDiffAdd
	removeByNormalized map[string]model.SyncDiffRemove
}

// prepareSyncApply resolves mailingListID's linked committee and sync_mode, applies the
// trusted-system-caller public-list remove restriction, checks expectedCommitteeUID against the
// currently-linked committee, checks committee-writer access, validates approvedAdds/
// approvedRemoves against the resolved direction, and builds the normalized-email maps ITX's
// apply request needs. See ApplySyncDiff for why each of these checks exists.
func (o *SyncApplyOrchestrator) prepareSyncApply(
	ctx context.Context,
	mailingListID string,
	expectedCommitteeUID string,
	approvedAdds []model.SyncDiffAdd,
	approvedRemoves []model.SyncDiffRemove,
) (*preparedSyncApply, error) {
	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return nil, errs.NewValidation("mailing list is not linked to a committee")
	}
	committeeUID := ml.Committees[0].UID

	if isTrustedSystemCaller(ctx) {
		switch ml.AudienceAccess {
		case model.AudienceAccessApprovalRequired, model.AudienceAccessInviteOnly:
			// private: removes may proceed.
		default:
			approvedRemoves = nil
		}
	}

	if expectedCommitteeUID != committeeUID {
		return nil, errs.NewConflict("mailing list's linked committee no longer matches the sync-diff this apply was approved against; request a fresh diff and re-apply")
	}

	if err := checkCommitteeWriterAccess(ctx, o.accessChecker, committeeUID); err != nil {
		return nil, err
	}

	direction, err := syncJobDirectionForMode(ml.Committees[0].SyncMode)
	if err != nil {
		return nil, err
	}

	wantAddType, wantRemoveType := memberTypesForDirection(direction)
	for _, add := range approvedAdds {
		if add.MemberType != wantAddType {
			return nil, errs.NewValidation("approved add for " + add.Email + " has member_type " +
				string(add.MemberType) + ", but the mailing list's current sync_mode expects " + string(wantAddType))
		}
	}
	for _, remove := range approvedRemoves {
		if remove.MemberType != wantRemoveType {
			return nil, errs.NewValidation("approved remove for " + remove.Email + " has member_type " +
				string(remove.MemberType) + ", but the mailing list's current sync_mode expects " + string(wantRemoveType))
		}
	}

	addByNormalized := make(map[string]model.SyncDiffAdd, len(approvedAdds))
	approvedAddEmails := make([]string, 0, len(approvedAdds))
	for _, add := range approvedAdds {
		normalized := normalizeEmail(add.Email)
		if existing, ok := addByNormalized[normalized]; ok {
			return nil, errs.NewValidation("approved adds contain duplicate normalized email " + normalized +
				" (" + existing.Email + " and " + add.Email + "); submit each add at most once")
		}
		approvedAddEmails = append(approvedAddEmails, add.Email)
		addByNormalized[normalized] = add
	}

	removeByNormalized := make(map[string]model.SyncDiffRemove, len(approvedRemoves))
	approvedRemoveEmails := make([]string, 0, len(approvedRemoves))
	for _, remove := range approvedRemoves {
		normalized := normalizeEmail(remove.Email)
		if existing, ok := removeByNormalized[normalized]; ok {
			return nil, errs.NewValidation("approved removes contain duplicate normalized email " + normalized +
				" (" + existing.Email + " and " + remove.Email + "); submit each remove at most once")
		}
		// ITX's apply workers match each approved_removes entry by email, not member ID — the
		// worker looks up the row with strings.EqualFold(m.Email, email) in both directions
		// (PR #101 review). remove.MemberID is retained only for translating ITX's echoed
		// email back to a per-entry outcome in fromSyncApplyJob below.
		approvedRemoveEmails = append(approvedRemoveEmails, remove.Email)
		removeByNormalized[normalized] = remove
	}

	return &preparedSyncApply{
		committeeUID:         committeeUID,
		direction:            direction,
		approvedAddEmails:    approvedAddEmails,
		approvedRemoveEmails: approvedRemoveEmails,
		addByNormalized:      addByNormalized,
		removeByNormalized:   removeByNormalized,
	}, nil
}

// ApplySyncDiff resolves mailingListID's linked committee and sync_mode, starts an ITX
// sync-apply job applying approvedAdds (by email) and approvedRemoves (by member ID), and waits
// for it to complete.
//
// Realtime auto-sync (RealtimeAutoSyncOrchestrator.TriggerAutoSync) only approves removes when
// the mailing list is private at the time it computes the diff — removals must never
// auto-propagate on a public list. That snapshot can go stale between TriggerAutoSync computing
// approvedRemoves and this call actually submitting them (e.g. the list is made public in
// between); prepareSyncApply re-enforces the same restriction against the mailing list's
// configuration at submission time, so a stale approval can't still remove someone from a
// now-public list. isTrustedSystemCaller(ctx) is true only for TriggerAutoSync's own background
// calls (see withTrustedSystemCaller) — an HTTP caller's approvedRemoves come from a human
// reviewing the diff via a manual Sync Now, which is deliberately allowed to remove regardless of
// visibility.
//
// The caller's approved adds/removes were vetted against the diff computed for
// expectedCommitteeUID — if the list was relinked to a different committee between that diff
// being fetched and this apply call, applying them now would attribute committee A's approved
// changes to committee B. prepareSyncApply only catches a relink that already happened before the
// call; the re-check after the ITX job completes (below) catches one that happens during it.
func (o *SyncApplyOrchestrator) ApplySyncDiff(
	ctx context.Context,
	mailingListID string,
	expectedCommitteeUID string,
	approvedAdds []model.SyncDiffAdd,
	approvedRemoves []model.SyncDiffRemove,
) (*model.SyncApplyResult, error) {
	prepared, err := o.prepareSyncApply(ctx, mailingListID, expectedCommitteeUID, approvedAdds, approvedRemoves)
	if err != nil {
		return nil, err
	}
	committeeUID := prepared.committeeUID

	jobID, err := o.itxSyncClient.PostSyncApply(ctx, mailingListID, prepared.direction, prepared.approvedAddEmails, prepared.approvedRemoveEmails, "")
	if err != nil {
		return nil, err
	}

	job, err := pollSyncApplyJob(ctx, o.itxSyncClient, mailingListID, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status == port.SyncJobStatusFailed {
		return nil, errs.NewServiceUnavailable("ITX sync-apply job failed: " + job.ErrorMessage)
	}

	if err := verifyJobCommitteeMatch(ctx, o.translator, job.CommitteeID, committeeUID); err != nil {
		return nil, err
	}

	// job.Results reflect the committee that was linked when PostSyncApply was called
	// (committeeUID). verifyJobCommitteeMatch above already checked job.CommitteeID when ITX's
	// response included one; this re-check covers the mailing list being relinked after the
	// (potentially long-running) job completed, which job.CommitteeID alone cannot catch — if it
	// changed, the job's results no longer describe the currently-linked committee, so reject the
	// result instead of attributing committee A's membership changes to committee B.
	refreshedML, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(refreshedML.Committees) == 0 || refreshedML.Committees[0].UID != committeeUID {
		return nil, errs.NewConflict("mailing list's linked committee changed while the sync-apply job was running; request a fresh diff and re-apply")
	}

	return fromSyncApplyJob(mailingListID, committeeUID, job, prepared.addByNormalized, prepared.removeByNormalized)
}

// StartSyncApply performs the same validation as ApplySyncDiff (see its doc comment for why each
// check exists) and starts the underlying ITX sync-apply job, returning its job ID immediately
// instead of waiting for it to complete (see ApplySyncDiff for the blocking equivalent, and
// GetSyncApplyStatus for polling this job's progress/result).
func (o *SyncApplyOrchestrator) StartSyncApply(
	ctx context.Context,
	mailingListID string,
	expectedCommitteeUID string,
	approvedAdds []model.SyncDiffAdd,
	approvedRemoves []model.SyncDiffRemove,
) (string, error) {
	prepared, err := o.prepareSyncApply(ctx, mailingListID, expectedCommitteeUID, approvedAdds, approvedRemoves)
	if err != nil {
		return "", err
	}

	return o.itxSyncClient.PostSyncApply(ctx, mailingListID, prepared.direction, prepared.approvedAddEmails, prepared.approvedRemoveEmails, "")
}

// GetSyncApplyStatus returns the current progress/result of a sync-apply job started by
// StartSyncApply. Like GetSyncDiffStatus, this has no job-start-time state to fall back on, so it
// re-derives everything from the mailing list's CURRENT linked committee: the committee-writer
// access check is re-run against it on every call, and a job reported against a different
// committee (job.CommitteeID, translated v1->v2) than the one currently linked is rejected as
// Conflict.
//
// Unlike ApplySyncDiff/fromSyncApplyJob, this does not reconstruct MemberID/MemberType/NameHint
// enrichment for each result — that requires the request-time approved-adds/removes, which aren't
// persisted for an async caller to re-supply on every poll. Results are returned as a flat
// model.SyncJobResultEntry (a direct copy of ITX's own per-entry email/action/status/error), with
// ITX's own pre-aggregated AddedCount/RemovedCount/ErrorCount in place of recomputing them.
func (o *SyncApplyOrchestrator) GetSyncApplyStatus(ctx context.Context, mailingListID, jobID string) (*model.SyncApplyJobStatus, error) {
	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return nil, errs.NewConflict("mailing list is no longer linked to a committee; request a fresh diff and re-apply")
	}
	committeeUID := ml.Committees[0].UID

	if err := checkCommitteeWriterAccess(ctx, o.accessChecker, committeeUID); err != nil {
		return nil, err
	}

	job, err := o.itxSyncClient.GetSyncApplyJob(ctx, mailingListID, jobID)
	if err != nil {
		return nil, err
	}

	status := &model.SyncApplyJobStatus{
		JobID:          job.ID,
		Status:         job.Status,
		ProcessedCount: job.ProcessedCount,
		TotalCount:     job.TotalCount,
		AddedCount:     job.AddedCount,
		RemovedCount:   job.RemovedCount,
		ErrorCount:     job.ErrorCount,
		ErrorMessage:   job.ErrorMessage,
	}
	if job.Status != port.SyncJobStatusCompleted && job.Status != port.SyncJobStatusFailed {
		return status, nil
	}

	if err := verifyJobCommitteeMatch(ctx, o.translator, job.CommitteeID, committeeUID); err != nil {
		return nil, err
	}

	status.Results = make([]model.SyncJobResultEntry, len(job.Results))
	for i, r := range job.Results {
		status.Results[i] = model.SyncJobResultEntry{
			Email:  r.Email,
			Action: r.Action,
			Status: r.Status,
			Error:  r.Error,
		}
	}
	return status, nil
}

// fromSyncApplyJob translates a completed ITX sync-apply job's flat action/status results back
// into model.SyncApplyResult's richer per-kind outcome. ITX has no "invited" concept (it manages
// Groups.io subscriptions directly, without LFX-account resolution) and no "skipped_cross_list"
// concept (its own sync-mode conflict validation differs from this service's former bespoke
// guard) — those outcomes never appear here, only added/removed/failed. ITX's results only carry
// email, not the member ID/type that was approved, so both are recovered from the original
// request by (normalized) email. A result email that matches no approved add/remove, or a result
// with an unrecognized Action, has no valid MemberType to report — the response schema requires
// one of the two enum values, so neither can be represented as a (even failed) per-entry outcome
// without violating it; those two cases are treated as ITX protocol violations and fail the whole
// request. A short result set (ITX silently omitting a result for an approved add/remove) is
// handled differently: each missing entry DOES have a valid MemberType (from the original
// request), so it is reported as its own SyncApplyOutcomeFailed instead of discarding the results
// ITX did report — per @andrest50, failing the whole request there would turn a response where
// ITX already applied some changes into an opaque 503 with no record of what succeeded, and
// invite a caller retry that re-submits entries that already took effect.
func fromSyncApplyJob(
	mailingListID, committeeUID string,
	job *port.SyncApplyJob,
	addByNormalized map[string]model.SyncDiffAdd,
	removeByNormalized map[string]model.SyncDiffRemove,
) (*model.SyncApplyResult, error) {
	result := &model.SyncApplyResult{
		MailingListID: mailingListID,
		CommitteeUID:  committeeUID,
		Added:         make([]model.SyncApplyAddResult, 0, len(job.Results)),
		Removed:       make([]model.SyncApplyRemoveResult, 0, len(job.Results)),
	}

	seenAdds := make(map[string]bool, len(addByNormalized))
	seenRemoves := make(map[string]bool, len(removeByNormalized))

	for _, r := range job.Results {
		normalized := normalizeEmail(r.Email)
		switch r.Action {
		case "add":
			add, ok := addByNormalized[normalized]
			if !ok {
				return nil, errs.NewServiceUnavailable(
					"ITX sync-apply reported a result for " + r.Email + " that does not match any approved add by email")
			}
			if seenAdds[normalized] {
				return nil, errs.NewServiceUnavailable(
					"ITX sync-apply reported more than one result for approved add " + r.Email)
			}
			seenAdds[normalized] = true
			outcome := model.SyncApplyOutcomeFailed
			if r.Status == "success" {
				outcome = model.SyncApplyOutcomeAdded
			}
			result.Added = append(result.Added, model.SyncApplyAddResult{
				Email:      r.Email,
				MemberID:   add.MemberID,
				MemberType: add.MemberType,
				Outcome:    outcome,
				NameHint:   add.Name,
				Error:      r.Error,
			})
		case "remove":
			remove, ok := removeByNormalized[normalized]
			if !ok {
				return nil, errs.NewServiceUnavailable(
					"ITX sync-apply reported a result for " + r.Email + " that does not match any approved remove by email")
			}
			if seenRemoves[normalized] {
				return nil, errs.NewServiceUnavailable(
					"ITX sync-apply reported more than one result for approved remove " + r.Email)
			}
			seenRemoves[normalized] = true
			outcome := model.SyncApplyOutcomeFailed
			if r.Status == "success" {
				outcome = model.SyncApplyOutcomeRemoved
			}
			result.Removed = append(result.Removed, model.SyncApplyRemoveResult{
				Email:      r.Email,
				MemberID:   remove.MemberID,
				MemberType: remove.MemberType,
				Outcome:    outcome,
				Error:      r.Error,
			})
		default:
			return nil, errs.NewServiceUnavailable(
				"ITX sync-apply reported a result for " + r.Email + " with unrecognized action " + r.Action)
		}
	}

	for normalized, add := range addByNormalized {
		if seenAdds[normalized] {
			continue
		}
		result.Added = append(result.Added, model.SyncApplyAddResult{
			Email:      add.Email,
			MemberID:   add.MemberID,
			MemberType: add.MemberType,
			Outcome:    model.SyncApplyOutcomeFailed,
			NameHint:   add.Name,
			Error:      "no result reported by ITX",
		})
	}
	for normalized, remove := range removeByNormalized {
		if seenRemoves[normalized] {
			continue
		}
		result.Removed = append(result.Removed, model.SyncApplyRemoveResult{
			Email:      remove.Email,
			MemberID:   remove.MemberID,
			MemberType: remove.MemberType,
			Outcome:    model.SyncApplyOutcomeFailed,
			Error:      "no result reported by ITX",
		})
	}

	return result, nil
}
