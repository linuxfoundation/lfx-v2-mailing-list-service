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
func (o *SyncDiffOrchestrator) ComputeSyncDiff(ctx context.Context, mailingListID string) (*model.SyncDiff, error) {
	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return nil, errs.NewValidation("mailing list is not linked to a committee")
	}
	committeeUID := ml.Committees[0].UID

	switch ml.Committees[0].SyncMode {
	case model.SyncModeAutoCommitteeToMailingList:
		// supported, proceed below
	case "", model.SyncModeManual:
		return nil, errs.NewValidation("mailing list sync_mode is manual; there is nothing to reconcile")
	case model.SyncModeAutoMailingListToCommittee:
		return nil, errs.NewValidation("sync_mode auto_ml_to_c is not yet supported for sync-diff")
	default:
		return nil, errs.NewValidation("mailing list has an unrecognized sync_mode: " + ml.Committees[0].SyncMode)
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
		committeeEmails[key] = struct{}{}
		if _, onList := mlEmails[key]; !onList {
			adds = append(adds, model.SyncDiffAdd{Email: cm.Email, CommitteeMemberUID: cm.UID})
		}
	}

	removes := make([]model.SyncDiffRemove, 0)
	for _, m := range mlMembers {
		if m.MemberType != committeeMemberType {
			continue
		}
		if _, stillOnCommittee := committeeEmails[normalizeEmail(m.Email)]; !stillOnCommittee {
			removes = append(removes, model.SyncDiffRemove{Email: m.Email, MailingListMemberID: m.UID})
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
