// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// SyncDiffOrchestrator implements port.SyncDiffComputer by delegating the actual reconciliation
// to ITX's own sync-diff job (itx-service-groupsio): ITX already knows a mailing list's
// committee link and sync_mode (proxied straight through, see proxy.subgroupWire.SyncMode), and
// computes the diff against its own committee-member and Groups.io-subscriber data, including
// voting-status filtering and email-conflict detection this service does not need to reimplement.
type SyncDiffOrchestrator struct {
	mailingListReader port.GroupsIOMailingListReader
	itxSyncClient     port.ITXSyncClient
}

// SyncDiffOrchestratorOption configures a SyncDiffOrchestrator.
type SyncDiffOrchestratorOption func(*SyncDiffOrchestrator)

// WithSyncDiffMailingListReader sets the reader used to resolve the mailing list's linked
// committee UID and configured sync_mode.
func WithSyncDiffMailingListReader(r port.GroupsIOMailingListReader) SyncDiffOrchestratorOption {
	return func(o *SyncDiffOrchestrator) {
		o.mailingListReader = r
	}
}

// WithSyncDiffITXSyncClient sets the client used to start and poll ITX's sync-diff job.
func WithSyncDiffITXSyncClient(c port.ITXSyncClient) SyncDiffOrchestratorOption {
	return func(o *SyncDiffOrchestrator) {
		o.itxSyncClient = c
	}
}

// NewSyncDiffOrchestrator creates a SyncDiffOrchestrator with the given options.
func NewSyncDiffOrchestrator(opts ...SyncDiffOrchestratorOption) port.SyncDiffComputer {
	o := &SyncDiffOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// ComputeSyncDiff resolves mailingListID's linked committee and sync_mode, starts an ITX
// sync-diff job for the corresponding direction, and waits for it to complete.
func (o *SyncDiffOrchestrator) ComputeSyncDiff(ctx context.Context, mailingListID string) (*model.SyncDiff, error) {
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

	jobID, err := o.itxSyncClient.PostSyncDiff(ctx, mailingListID, direction)
	if err != nil {
		return nil, err
	}

	job, err := pollSyncDiffJob(ctx, o.itxSyncClient, mailingListID, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status == port.SyncJobStatusFailed {
		return nil, errs.NewServiceUnavailable("ITX sync-diff job failed: " + job.ErrorMessage)
	}

	// Re-resolve the mailing list's linked committee after the (potentially long-running) ITX job
	// completes, rather than trusting the committeeUID resolved before the job started: narrows
	// the window in which the result is mislabeled with a stale committee if the mailing list gets
	// reassociated to a different committee while the job was in flight.
	if refreshedML, refreshErr := o.mailingListReader.GetMailingList(ctx, mailingListID); refreshErr == nil &&
		len(refreshedML.Committees) > 0 && refreshedML.Committees[0].UID != "" {
		committeeUID = refreshedML.Committees[0].UID
	}

	return fromSyncDiffJob(mailingListID, committeeUID, ml.Committees[0].SyncMode, direction, job), nil
}

// fromSyncDiffJob translates a completed ITX sync-diff job into model.SyncDiff. direction decides
// which roster each of ToAdd/ToRemove's IDs belongs to: for auto_c_to_ml, ToAdd holds committee
// members and ToRemove holds mailing-list members; for auto_ml_to_c it is the reverse.
func fromSyncDiffJob(mailingListID, committeeUID, syncMode, direction string, job *port.SyncDiffJob) *model.SyncDiff {
	addMemberType, removeMemberType := memberTypesForDirection(direction)

	adds := make([]model.SyncDiffAdd, len(job.ToAdd))
	for i, m := range job.ToAdd {
		adds[i] = model.SyncDiffAdd{
			Email:      m.Email,
			MemberID:   m.MemberID,
			MemberType: addMemberType,
			Name:       m.FullName,
		}
	}
	removes := make([]model.SyncDiffRemove, len(job.ToRemove))
	for i, m := range job.ToRemove {
		removes[i] = model.SyncDiffRemove{
			Email:      m.Email,
			MemberID:   m.MemberID,
			MemberType: removeMemberType,
		}
	}
	conflicts := make([]model.SyncDiffConflict, len(job.EmailConflicts))
	for i, c := range job.EmailConflicts {
		conflicts[i] = model.SyncDiffConflict{
			CommitteeEmail:   c.CommitteeEmail,
			MailingListEmail: c.MLEmail,
			Name:             c.FullName,
		}
	}
	return &model.SyncDiff{
		MailingListID: mailingListID,
		CommitteeUID:  committeeUID,
		SyncMode:      syncMode,
		Adds:          adds,
		Removes:       removes,
		Conflicts:     conflicts,
	}
}
