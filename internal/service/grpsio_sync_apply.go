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
// sync-apply job applying approvedAdds (by email) and approvedRemoves (by mailing-list member
// ID), and waits for it to complete.
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

	addEmailByNormalized := make(map[string]model.SyncDiffAdd, len(approvedAdds))
	approvedAddEmails := make([]string, 0, len(approvedAdds))
	for _, add := range approvedAdds {
		approvedAddEmails = append(approvedAddEmails, add.Email)
		addEmailByNormalized[normalizeEmail(add.Email)] = add
	}

	removeByMemberID := make(map[string]model.SyncDiffRemove, len(approvedRemoves))
	approvedRemoveIDs := make([]string, 0, len(approvedRemoves))
	for _, remove := range approvedRemoves {
		approvedRemoveIDs = append(approvedRemoveIDs, remove.MailingListMemberID)
		removeByMemberID[remove.MailingListMemberID] = remove
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

	return fromSyncApplyJob(mailingListID, committeeUID, job, addEmailByNormalized, removeByMemberID), nil
}

// fromSyncApplyJob translates a completed ITX sync-apply job's flat action/status results back
// into model.SyncApplyResult's richer per-kind outcome. ITX has no "invited" concept (it manages
// Groups.io subscriptions directly, without LFX-account resolution) and no "skipped_cross_list"
// concept (its own sync-mode conflict validation differs from this service's former bespoke
// guard) — those outcomes never appear here, only added/removed/failed.
func fromSyncApplyJob(
	mailingListID, committeeUID string,
	job *port.SyncApplyJob,
	addEmailByNormalized map[string]model.SyncDiffAdd,
	removeByMemberID map[string]model.SyncDiffRemove,
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
			add := addEmailByNormalized[normalizeEmail(r.Email)]
			result.Added = append(result.Added, model.SyncApplyAddResult{
				Email:              r.Email,
				CommitteeMemberUID: add.CommitteeMemberUID,
				Outcome:            outcome,
				NameHint:           add.Name,
				Error:              r.Error,
			})
		case "remove":
			if r.Status == "success" {
				outcome = model.SyncApplyOutcomeRemoved
			}
			// ITX's sync-apply results only carry email, not the mailing-list member ID that was
			// approved for removal; recover it from the original request by email.
			var memberID string
			for id, remove := range removeByMemberID {
				if normalizeEmail(remove.Email) == normalizeEmail(r.Email) {
					memberID = id
					break
				}
			}
			result.Removed = append(result.Removed, model.SyncApplyRemoveResult{
				Email:               r.Email,
				MailingListMemberID: memberID,
				Outcome:             outcome,
				Error:               r.Error,
			})
		}
	}

	return result
}
