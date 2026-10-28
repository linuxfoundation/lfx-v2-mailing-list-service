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
		approvedAddEmails = append(approvedAddEmails, add.Email)
		addEmailByNormalized[normalizeEmail(add.Email)] = add
	}

	removeByNormalized := make(map[string]model.SyncDiffRemove, len(approvedRemoves))
	approvedRemoveIDs := make([]string, 0, len(approvedRemoves))
	for _, remove := range approvedRemoves {
		approvedRemoveIDs = append(approvedRemoveIDs, remove.MemberID)
		removeByNormalized[normalizeEmail(remove.Email)] = remove
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

	// Re-resolve the mailing list's linked committee after the (potentially long-running) ITX job
	// completes, rather than trusting the committeeUID resolved before the job started: narrows
	// the window in which the result is mislabeled with a stale committee if the mailing list gets
	// reassociated to a different committee while the job was in flight.
	if refreshedML, refreshErr := o.mailingListReader.GetMailingList(ctx, mailingListID); refreshErr == nil &&
		len(refreshedML.Committees) > 0 && refreshedML.Committees[0].UID != "" {
		committeeUID = refreshedML.Committees[0].UID
	}

	return fromSyncApplyJob(mailingListID, committeeUID, job, addEmailByNormalized, removeByNormalized), nil
}

// fromSyncApplyJob translates a completed ITX sync-apply job's flat action/status results back
// into model.SyncApplyResult's richer per-kind outcome. ITX has no "invited" concept (it manages
// Groups.io subscriptions directly, without LFX-account resolution) and no "skipped_cross_list"
// concept (its own sync-mode conflict validation differs from this service's former bespoke
// guard) — those outcomes never appear here, only added/removed/failed. ITX's results only carry
// email, not the member ID/type that was approved, so both are recovered from the original
// request by (normalized) email.
func fromSyncApplyJob(
	mailingListID, committeeUID string,
	job *port.SyncApplyJob,
	addByNormalized map[string]model.SyncDiffAdd,
	removeByNormalized map[string]model.SyncDiffRemove,
) *model.SyncApplyResult {
	result := &model.SyncApplyResult{
		MailingListID: mailingListID,
		CommitteeUID:  committeeUID,
		Added:         make([]model.SyncApplyAddResult, 0, len(job.Results)),
		Removed:       make([]model.SyncApplyRemoveResult, 0, len(job.Results)),
	}

	for _, r := range job.Results {
		outcome := model.SyncApplyOutcomeFailed
		switch r.Action {
		case "add":
			if r.Status == "success" {
				outcome = model.SyncApplyOutcomeAdded
			}
			add, ok := addByNormalized[normalizeEmail(r.Email)]
			errMsg := r.Error
			if !ok {
				outcome = model.SyncApplyOutcomeFailed
				errMsg = "ITX reported this email but it does not match any approved add by email: " + r.Email
			}
			result.Added = append(result.Added, model.SyncApplyAddResult{
				Email:      r.Email,
				MemberID:   add.MemberID,
				MemberType: add.MemberType,
				Outcome:    outcome,
				NameHint:   add.Name,
				Error:      errMsg,
			})
		case "remove":
			if r.Status == "success" {
				outcome = model.SyncApplyOutcomeRemoved
			}
			remove, ok := removeByNormalized[normalizeEmail(r.Email)]
			errMsg := r.Error
			if !ok {
				outcome = model.SyncApplyOutcomeFailed
				errMsg = "ITX reported this email but it does not match any approved remove by email: " + r.Email
			}
			result.Removed = append(result.Removed, model.SyncApplyRemoveResult{
				Email:      r.Email,
				MemberID:   remove.MemberID,
				MemberType: remove.MemberType,
				Outcome:    outcome,
				Error:      errMsg,
			})
		}
	}

	return result
}
