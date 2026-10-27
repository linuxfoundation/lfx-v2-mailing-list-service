// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// SyncApplyOrchestrator implements port.SyncApplier by applying an admin-approved subset of a
// mailing list's sync-diff: adding resolved committee members and removing members who have
// dropped off the linked committee, guarded against removing someone still reachable through
// another mailing list linked to the same committee.
type SyncApplyOrchestrator struct {
	mailingListReader port.GroupsIOMailingListReader
	memberReader      port.GroupsIOMailingListMemberReader
	memberWriter      port.GroupsIOMailingListMemberWriter
	syncSettingsStore port.CommitteeSyncSettingsStore
	userReader        port.UserReader
}

// SyncApplyOrchestratorOption configures a SyncApplyOrchestrator.
type SyncApplyOrchestratorOption func(*SyncApplyOrchestrator)

// WithSyncApplyMailingListReader sets the reader used to resolve the mailing list's linked
// committee UID and to enumerate sibling mailing lists for the cross-list removal guard.
func WithSyncApplyMailingListReader(r port.GroupsIOMailingListReader) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.mailingListReader = r }
}

// WithSyncApplyMemberReader sets the reader used to list members of the target mailing list and
// its siblings, for the cross-list removal guard.
func WithSyncApplyMemberReader(r port.GroupsIOMailingListMemberReader) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.memberReader = r }
}

// WithSyncApplyMemberWriter sets the writer used to add, remove, and invite mailing list members.
func WithSyncApplyMemberWriter(w port.GroupsIOMailingListMemberWriter) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.memberWriter = w }
}

// WithSyncApplySyncSettingsStore sets the store used to confirm the committee's configured
// sync_mode supports apply, for the same reason ComputeSyncDiff looks it up directly rather
// than through the mailing list reader's best-effort hydration (see grpsio_sync_diff.go).
func WithSyncApplySyncSettingsStore(s port.CommitteeSyncSettingsStore) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.syncSettingsStore = s }
}

// WithSyncApplyUserReader sets the reader used to resolve an LFX account for each add's email.
func WithSyncApplyUserReader(r port.UserReader) SyncApplyOrchestratorOption {
	return func(o *SyncApplyOrchestrator) { o.userReader = r }
}

// NewSyncApplyOrchestrator creates a SyncApplyOrchestrator with the given options.
func NewSyncApplyOrchestrator(opts ...SyncApplyOrchestratorOption) port.SyncApplier {
	o := &SyncApplyOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// ApplySyncDiff resolves mailingListID's linked committee and sync_mode, then applies
// approvedAdds and approvedRemoves against it.
//
// Validation mirrors SyncDiffOrchestrator.ComputeSyncDiff: only "auto_c_to_ml" is supported, and
// sync_mode is read directly from syncSettingsStore rather than through the mailing list
// reader's best-effort hydration, for the same reason (see grpsio_sync_diff.go).
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

	syncMode, _, found, err := o.syncSettingsStore.GetSyncMode(ctx, ml.UID, committeeUID)
	if err != nil {
		return nil, errs.NewServiceUnavailable("failed to load committee sync_mode", err)
	}
	if !found {
		syncMode = model.SyncModeManual
	}
	switch syncMode {
	case model.SyncModeAutoCommitteeToMailingList:
		// supported, proceed below
	case "", model.SyncModeManual:
		return nil, errs.NewValidation("mailing list sync_mode is manual; there is nothing to apply")
	case model.SyncModeAutoMailingListToCommittee:
		return nil, errs.NewValidation("sync_mode auto_ml_to_c is not yet supported for sync-apply")
	default:
		return nil, errs.NewValidation("mailing list has an unrecognized sync_mode: " + syncMode)
	}

	result := &model.SyncApplyResult{
		MailingListID: mailingListID,
		CommitteeUID:  committeeUID,
		Added:         make([]model.SyncApplyAddResult, 0, len(approvedAdds)),
		Removed:       make([]model.SyncApplyRemoveResult, 0, len(approvedRemoves)),
	}

	for _, add := range approvedAdds {
		result.Added = append(result.Added, o.applyAdd(ctx, mailingListID, add))
	}

	if len(approvedRemoves) > 0 {
		siblingEmails, err := o.siblingLinkedListEmails(ctx, committeeUID, mailingListID)
		if err != nil {
			return nil, err
		}
		for _, remove := range approvedRemoves {
			result.Removed = append(result.Removed, o.applyRemove(ctx, mailingListID, remove, siblingEmails))
		}
	}

	return result, nil
}

// applyAdd resolves add's email to an LFX account and adds the member directly, falling back to
// an invite when no account is found. A hard failure from either path is reported, never
// swallowed — an admin reviewing SyncApplyResult must see every add that did not succeed.
func (o *SyncApplyOrchestrator) applyAdd(ctx context.Context, mailingListID string, add model.SyncDiffAdd) model.SyncApplyAddResult {
	res := model.SyncApplyAddResult{Email: add.Email, CommitteeMemberUID: add.CommitteeMemberUID}

	username, err := o.userReader.UsernameByEmail(ctx, add.Email)
	switch {
	case err == nil:
		member := &model.GrpsIOMember{
			Email:      add.Email,
			Username:   username,
			MemberType: committeeMemberType,
		}
		if _, addErr := o.memberWriter.AddMember(ctx, mailingListID, member); addErr != nil {
			res.Outcome = model.SyncApplyOutcomeFailed
			res.Error = addErr.Error()
			return res
		}
		res.Outcome = model.SyncApplyOutcomeAdded
		return res

	case errors.Is(err, port.ErrUserNotFound):
		if inviteErr := o.memberWriter.InviteMembers(ctx, mailingListID, []string{add.Email}); inviteErr != nil {
			res.Outcome = model.SyncApplyOutcomeFailed
			res.Error = inviteErr.Error()
			return res
		}
		res.Outcome = model.SyncApplyOutcomeInvited
		res.NameHint = add.Name
		return res

	default:
		res.Outcome = model.SyncApplyOutcomeFailed
		res.Error = err.Error()
		return res
	}
}

// applyRemove removes remove's member from mailingListID, unless they are still reachable via
// another mailing list linked to the same committee (siblingEmails) — removing them here would
// just orphan the committee link on the sibling list while still disconnecting them from this
// one's distribution, which is not what a committee-to-mailing-list sync should do.
func (o *SyncApplyOrchestrator) applyRemove(
	ctx context.Context,
	mailingListID string,
	remove model.SyncDiffRemove,
	siblingEmails map[string]struct{},
) model.SyncApplyRemoveResult {
	res := model.SyncApplyRemoveResult{Email: remove.Email, MailingListMemberID: remove.MailingListMemberID}

	if _, stillReachable := siblingEmails[normalizeEmail(remove.Email)]; stillReachable {
		res.Outcome = model.SyncApplyOutcomeSkippedCrossList
		return res
	}

	if err := o.memberWriter.DeleteMember(ctx, mailingListID, remove.MailingListMemberID); err != nil {
		res.Outcome = model.SyncApplyOutcomeFailed
		res.Error = err.Error()
		return res
	}
	res.Outcome = model.SyncApplyOutcomeRemoved
	return res
}

// siblingLinkedListEmails returns the set of normalized member emails present on any other
// mailing list linked to committeeUID besides excludeMailingListID.
func (o *SyncApplyOrchestrator) siblingLinkedListEmails(ctx context.Context, committeeUID, excludeMailingListID string) (map[string]struct{}, error) {
	siblings, _, err := o.mailingListReader.ListMailingLists(ctx, "", committeeUID)
	if err != nil {
		return nil, errs.NewServiceUnavailable("failed to list mailing lists linked to committee for cross-list removal guard", err)
	}

	emails := make(map[string]struct{})
	for _, sibling := range siblings {
		if sibling.UID == excludeMailingListID {
			continue
		}
		members, _, err := o.memberReader.ListMembers(ctx, sibling.UID)
		if err != nil {
			return nil, errs.NewServiceUnavailable("failed to list members of sibling mailing list "+sibling.UID+" for cross-list removal guard", err)
		}
		for _, m := range members {
			if key := normalizeEmail(m.Email); key != "" {
				emails[key] = struct{}{}
			}
		}
	}
	return emails, nil
}
