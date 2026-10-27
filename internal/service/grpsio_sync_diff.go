// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// committeeMemberType is the GrpsIOMember.MemberType value used for members added to a mailing
// list by committee sync. Only members tagged this way are eligible for sync-driven removal —
// a member added directly (MemberType "direct") who happens to share an email with a departed
// committee member must never be silently removed.
const committeeMemberType = "committee"

// SyncDiffOrchestrator implements port.SyncDiffComputer by reconciling a mailing list's current
// membership against its linked committee's membership, per the committee's sync_mode.
type SyncDiffOrchestrator struct {
	mailingListReader port.GroupsIOMailingListReader
	memberReader      port.GroupsIOMailingListMemberReader
	committeeLookup   port.CommitteeMembershipLookup
	syncSettingsStore port.CommitteeSyncSettingsStore
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

// WithSyncDiffMemberReader sets the reader used to list the mailing list's current members.
func WithSyncDiffMemberReader(r port.GroupsIOMailingListMemberReader) SyncDiffOrchestratorOption {
	return func(o *SyncDiffOrchestrator) {
		o.memberReader = r
	}
}

// WithSyncDiffCommitteeMembershipLookup sets the lookup used to resolve the linked committee's
// current members.
func WithSyncDiffCommitteeMembershipLookup(l port.CommitteeMembershipLookup) SyncDiffOrchestratorOption {
	return func(o *SyncDiffOrchestrator) {
		o.committeeLookup = l
	}
}

// WithSyncDiffSyncSettingsStore sets the store used to resolve the linked committee's configured
// sync_mode directly, so a genuine storage failure can be reported as such instead of being
// masked by the mailing-list reader's best-effort hydration (see ComputeSyncDiff).
func WithSyncDiffSyncSettingsStore(s port.CommitteeSyncSettingsStore) SyncDiffOrchestratorOption {
	return func(o *SyncDiffOrchestrator) {
		o.syncSettingsStore = s
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

// ComputeSyncDiff resolves mailingListID's linked committee and sync_mode, then reconciles its
// current membership against the committee's current membership.
//
// Only "auto_c_to_ml" is currently supported: "auto_ml_to_c" would require writing committee
// membership, and this service has no committee-write capability (CommitteeMembershipLookup is
// read-only; committee-service exposes member writes over HTTP only, requiring org/role data
// this service does not have) — see LFXV2-4029.
//
// sync_mode is looked up directly from syncSettingsStore rather than through
// mailingListReader's GetMailingList response: that response's Committees[0].SyncMode is filled
// in by the reader's best-effort hydration, which swallows a storage error and leaves SyncMode
// empty — indistinguishable here from a genuinely manual list. A direct lookup lets a real
// storage failure surface as ServiceUnavailable instead of an incorrect Validation error.
func (o *SyncDiffOrchestrator) ComputeSyncDiff(ctx context.Context, mailingListID string) (*model.SyncDiff, error) {
	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return nil, errs.NewValidation("mailing list is not linked to a committee")
	}
	committeeUID := ml.Committees[0].UID

	// port.CommitteeMember only carries {UID, Email} — CommitteeMembershipLookup has no way to
	// report a member's voting status, so a filtered link's AllowedVotingStatuses cannot be
	// applied here. Computing the diff against the full roster would incorrectly propose
	// filtered-out members as adds and miss removals for members who no longer satisfy the
	// filter. Reject rather than silently reconcile against the wrong membership set until the
	// lookup contract carries voting status (tracked in linuxfoundation/lfx-self-serve#4339).
	if len(ml.Committees[0].AllowedVotingStatuses) > 0 {
		return nil, errs.NewValidation("sync-diff does not yet support committees with voting-status filters")
	}

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
		return nil, errs.NewValidation("mailing list sync_mode is manual; there is nothing to reconcile")
	case model.SyncModeAutoMailingListToCommittee:
		return nil, errs.NewValidation("sync_mode auto_ml_to_c is not yet supported for sync-diff")
	default:
		return nil, errs.NewValidation("mailing list has an unrecognized sync_mode: " + syncMode)
	}

	committeeMembers, err := o.committeeLookup.GetCommitteeMembers(ctx, committeeUID)
	if err != nil {
		return nil, err
	}

	mlMembers, _, err := o.memberReader.ListMembers(ctx, mailingListID)
	if err != nil {
		return nil, err
	}

	return computeCommitteeToMailingListDiff(mailingListID, committeeUID, committeeMembers, mlMembers), nil
}

// normalizeEmail returns email in the canonical form used to match a committee member against a
// mailing list member: trimmed and lowercased. Neither side's storage guarantees a consistent
// case or surrounding whitespace.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// computeCommitteeToMailingListDiff reconciles committeeMembers against mlMembers by email,
// producing the adds/removes needed to make the mailing list match the committee.
func computeCommitteeToMailingListDiff(
	mailingListID, committeeUID string,
	committeeMembers []port.CommitteeMember,
	mlMembers []*model.GrpsIOMember,
) *model.SyncDiff {
	mlEmails := make(map[string]struct{}, len(mlMembers))
	for _, m := range mlMembers {
		if key := normalizeEmail(m.Email); key != "" {
			mlEmails[key] = struct{}{}
		}
	}

	committeeEmails := make(map[string]struct{}, len(committeeMembers))
	adds := make([]model.SyncDiffAdd, 0)
	for _, cm := range committeeMembers {
		key := normalizeEmail(cm.Email)
		if key == "" {
			continue
		}
		if _, seen := committeeEmails[key]; seen {
			continue
		}
		committeeEmails[key] = struct{}{}
		if _, onList := mlEmails[key]; !onList {
			adds = append(adds, model.SyncDiffAdd{
				Email:              strings.TrimSpace(cm.Email),
				CommitteeMemberUID: cm.UID,
				Name:               cm.Name,
			})
		}
	}

	removes := make([]model.SyncDiffRemove, 0)
	for _, m := range mlMembers {
		if m.MemberType != committeeMemberType {
			continue
		}
		key := normalizeEmail(m.Email)
		if key == "" {
			continue
		}
		if _, stillOnCommittee := committeeEmails[key]; !stillOnCommittee {
			removes = append(removes, model.SyncDiffRemove{Email: strings.TrimSpace(m.Email), MailingListMemberID: m.UID})
		}
	}

	return &model.SyncDiff{
		MailingListID: mailingListID,
		CommitteeUID:  committeeUID,
		SyncMode:      model.SyncModeAutoCommitteeToMailingList,
		Adds:          adds,
		Removes:       removes,
	}
}
