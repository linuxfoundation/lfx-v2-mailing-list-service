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
	accessChecker     port.AccessChecker
	translator        port.Translator
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

// WithSyncDiffAccessChecker sets the checker used to verify the caller holds writer access on
// the mailing list's linked committee before computing a diff — the diff response (ToAdd/ToRemove
// /EmailConflicts) includes committee roster emails and names regardless of sync direction, so
// without this check a caller with mailing-list writer access but no committee access could read
// committee roster PII through this endpoint alone.
func WithSyncDiffAccessChecker(c port.AccessChecker) SyncDiffOrchestratorOption {
	return func(o *SyncDiffOrchestrator) {
		o.accessChecker = c
	}
}

// WithSyncDiffTranslator sets the ID translator used to compare ITX's job-reported committee ID
// (v1) against the mailing list's linked committee UID (v2) in the job-completion recheck below.
func WithSyncDiffTranslator(t port.Translator) SyncDiffOrchestratorOption {
	return func(o *SyncDiffOrchestrator) {
		o.translator = t
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
		return nil, errs.NewValidation(notLinkedToCommitteeValidationMsg, ErrNothingToReconcile)
	}
	committeeUID := ml.Committees[0].UID

	if err := checkCommitteeWriterAccess(ctx, o.accessChecker, committeeUID); err != nil {
		return nil, err
	}

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

	if err := verifyJobCommitteeMatch(ctx, o.translator, job.CommitteeID, committeeUID); err != nil {
		return nil, err
	}

	// job.ToAdd/ToRemove reflect the committee that was linked when PostSyncDiff was called
	// (committeeUID). verifyJobCommitteeMatch above already checked job.CommitteeID when ITX's
	// response included one; this re-check covers the mailing list being relinked after the
	// (potentially long-running) job completed, which job.CommitteeID alone cannot catch — if it
	// changed, the job's data no longer describes the currently-linked committee, so reject the
	// result instead of mislabeling committee A's diff as committee B's.
	refreshedML, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(refreshedML.Committees) == 0 || refreshedML.Committees[0].UID != committeeUID {
		return nil, errs.NewConflict("mailing list's linked committee changed while the sync-diff job was running; request a fresh diff")
	}

	return fromSyncDiffJob(mailingListID, committeeUID, ml.Committees[0].SyncMode, direction, job), nil
}

// StartSyncDiff resolves mailingListID's linked committee and sync_mode, checks committee-writer
// access, and starts an ITX sync-diff job for the corresponding direction — returning its job ID
// immediately instead of waiting for it to complete (see ComputeSyncDiff for the blocking
// equivalent, and GetSyncDiffStatus for polling this job's progress/result).
func (o *SyncDiffOrchestrator) StartSyncDiff(ctx context.Context, mailingListID string) (string, error) {
	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return "", err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return "", errs.NewValidation(notLinkedToCommitteeValidationMsg, ErrNothingToReconcile)
	}
	committeeUID := ml.Committees[0].UID

	if err := checkCommitteeWriterAccess(ctx, o.accessChecker, committeeUID); err != nil {
		return "", err
	}

	direction, err := syncJobDirectionForMode(ml.Committees[0].SyncMode)
	if err != nil {
		return "", err
	}

	return o.itxSyncClient.PostSyncDiff(ctx, mailingListID, direction)
}

// GetSyncDiffStatus returns the current progress/result of a sync-diff job started by
// StartSyncDiff. Unlike ComputeSyncDiff, this has no job-start-time state to fall back on, so it
// re-derives everything ComputeSyncDiff's post-poll checks rely on from the mailing list's CURRENT
// linked committee: the committee-writer access check is re-run against it on every call, and a
// job reported against a different committee (job.CommitteeID, translated v1->v2) than the one
// currently linked is rejected as Conflict — which also transitively catches the mailing list
// having been relinked or unlinked since the job started, without needing to persist the
// committee UID StartSyncDiff saw.
func (o *SyncDiffOrchestrator) GetSyncDiffStatus(ctx context.Context, mailingListID, jobID string) (*model.SyncDiffJobStatus, error) {
	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return nil, errs.NewConflict("mailing list is no longer linked to a committee; request a fresh diff")
	}
	committeeUID := ml.Committees[0].UID

	if err := checkCommitteeWriterAccess(ctx, o.accessChecker, committeeUID); err != nil {
		return nil, err
	}

	job, err := o.itxSyncClient.GetSyncDiffJob(ctx, mailingListID, jobID)
	if err != nil {
		return nil, err
	}

	status := &model.SyncDiffJobStatus{
		JobID:          job.ID,
		Status:         job.Status,
		ProcessedCount: job.ProcessedCount,
		TotalCount:     job.TotalCount,
		ErrorMessage:   job.ErrorMessage,
	}
	if job.Status != port.SyncJobStatusCompleted {
		return status, nil
	}

	if err := verifyJobCommitteeMatch(ctx, o.translator, job.CommitteeID, committeeUID); err != nil {
		return nil, err
	}

	// job.Direction reflects the sync_mode in effect when the job was started; if sync_mode has
	// since changed (including to manual), labeling the job's diff with the CURRENT sync_mode
	// below would mismatch the direction the data was actually computed for — reject instead of
	// mixing a stale direction's data with today's mode.
	currentDirection, err := syncJobDirectionForMode(ml.Committees[0].SyncMode)
	if err != nil || currentDirection != job.Direction {
		return nil, errs.NewConflict("mailing list's sync mode changed while the sync-diff job was running; request a fresh diff")
	}

	status.Diff = fromSyncDiffJob(mailingListID, committeeUID, ml.Committees[0].SyncMode, job.Direction, job)
	return status, nil
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
