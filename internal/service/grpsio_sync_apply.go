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

// ApplySyncDiff resolves mailingListID's linked committee and sync_mode, starts an ITX
// sync-apply job applying approvedAdds (by email) and approvedRemoves (by member ID), and waits
// for it to complete.
func (o *SyncApplyOrchestrator) ApplySyncDiff(
	ctx context.Context,
	mailingListID string,
	expectedCommitteeUID string,
	approvedAdds []model.SyncDiffAdd,
	approvedRemoves []model.SyncDiffRemove,
) (*model.SyncApplyResult, error) {
	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return nil, errs.NewValidation("mailing list is not linked to a committee")
	}
	committeeUID := ml.Committees[0].UID

	// The caller's approved adds/removes were vetted against the diff computed for
	// expectedCommitteeUID — if the list was relinked to a different committee between that diff
	// being fetched and this apply call, applying them now would attribute committee A's approved
	// changes to committee B. This only catches a relink that already happened before the call;
	// the re-check after the ITX job completes (below) catches one that happens during it.
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

	addEmailByNormalized := make(map[string]model.SyncDiffAdd, len(approvedAdds))
	approvedAddEmails := make([]string, 0, len(approvedAdds))
	for _, add := range approvedAdds {
		normalized := normalizeEmail(add.Email)
		if existing, ok := addEmailByNormalized[normalized]; ok {
			return nil, errs.NewValidation("approved adds contain duplicate normalized email " + normalized +
				" (" + existing.Email + " and " + add.Email + "); submit each add at most once")
		}
		approvedAddEmails = append(approvedAddEmails, add.Email)
		addEmailByNormalized[normalized] = add
	}

	removeByNormalized := make(map[string]model.SyncDiffRemove, len(approvedRemoves))
	approvedRemoveIDs := make([]string, 0, len(approvedRemoves))
	for _, remove := range approvedRemoves {
		normalized := normalizeEmail(remove.Email)
		if existing, ok := removeByNormalized[normalized]; ok {
			return nil, errs.NewValidation("approved removes contain duplicate normalized email " + normalized +
				" (" + existing.Email + " and " + remove.Email + "); submit each remove at most once")
		}
		approvedRemoveIDs = append(approvedRemoveIDs, remove.MemberID)
		removeByNormalized[normalized] = remove
	}

	jobID, err := o.itxSyncClient.PostSyncApply(ctx, mailingListID, direction, approvedAddEmails, approvedRemoveIDs, "")
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
	// (committeeUID) — ITX's job response carries no committee identifier of its own to confirm
	// this against. Re-check the link after the (potentially long-running) job completes: if it
	// changed, the job's results no longer describe the currently-linked committee, so reject the
	// result instead of attributing committee A's membership changes to committee B.
	refreshedML, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(refreshedML.Committees) == 0 || refreshedML.Committees[0].UID != committeeUID {
		return nil, errs.NewConflict("mailing list's linked committee changed while the sync-apply job was running; request a fresh diff and re-apply")
	}

	return fromSyncApplyJob(mailingListID, committeeUID, job, addEmailByNormalized, removeByNormalized)
}

// fromSyncApplyJob translates a completed ITX sync-apply job's flat action/status results back
// into model.SyncApplyResult's richer per-kind outcome. ITX has no "invited" concept (it manages
// Groups.io subscriptions directly, without LFX-account resolution) and no "skipped_cross_list"
// concept (its own sync-mode conflict validation differs from this service's former bespoke
// guard) — those outcomes never appear here, only added/removed/failed. ITX's results only carry
// email, not the member ID/type that was approved, so both are recovered from the original
// request by (normalized) email. A result email that matches no approved add/remove has no valid
// MemberType to report — the response schema requires one of the two enum values, so such a
// result can't be represented as a (even failed) per-entry outcome without violating it. That
// case, an unrecognized Action, and a short result set (ITX silently dropping an approved entry)
// are all treated as ITX protocol violations and fail the whole request — the API promises exactly
// one outcome per approved add/remove, so a partial result set must not be reported as success.
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

	if len(seenAdds) != len(addByNormalized) || len(seenRemoves) != len(removeByNormalized) {
		return nil, errs.NewServiceUnavailable(
			"ITX sync-apply job completed without reporting a result for every approved add/remove")
	}

	return result, nil
}
