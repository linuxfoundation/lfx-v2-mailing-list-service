// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service provides application service implementations.
package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// GroupsIOMailingListOrchestrator implements port.GroupsIOMailingListWriter by wrapping an inner
// GroupsIOMailingListWriter and translating v2 UUIDs to v1 SFIDs before forwarding requests.
// It also publishes committee mailing list status events after each mutation.
type GroupsIOMailingListOrchestrator struct {
	writer     port.GroupsIOMailingListWriter
	reader     port.GroupsIOMailingListReader
	translator port.Translator
	publisher  port.MessagePublisher
}

// MailingListOrchestratorOption configures a GroupsIOMailingListOrchestrator.
type MailingListOrchestratorOption func(*GroupsIOMailingListOrchestrator)

// WithMailingListWriter sets the underlying writer (e.g. the ITX proxy client).
func WithMailingListWriter(w port.GroupsIOMailingListWriter) MailingListOrchestratorOption {
	return func(o *GroupsIOMailingListOrchestrator) {
		o.writer = w
	}
}

// WithMailingListTranslator sets the ID translator.
func WithMailingListTranslator(t port.Translator) MailingListOrchestratorOption {
	return func(o *GroupsIOMailingListOrchestrator) {
		o.translator = t
	}
}

// WithMailingListEventReader sets the reader used to fetch current state before
// update/delete operations so committee status events can be published correctly.
func WithMailingListEventReader(r port.GroupsIOMailingListReader) MailingListOrchestratorOption {
	return func(o *GroupsIOMailingListOrchestrator) {
		o.reader = r
	}
}

// WithMailingListPublisher sets the message publisher for inter-service events.
func WithMailingListPublisher(p port.MessagePublisher) MailingListOrchestratorOption {
	return func(o *GroupsIOMailingListOrchestrator) {
		o.publisher = p
	}
}

// CreateMailingList creates a new mailing list, mapping project_uid (v2) -> project_id (v1)
// and committee_uid (v2) -> committee_id (v1) before forwarding.
// After a successful create it publishes a committee mailing list status event.
func (o *GroupsIOMailingListOrchestrator) CreateMailingList(ctx context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	toSend, err := o.mapMailingListRequest(ctx, ml)
	if err != nil {
		return nil, err
	}

	resp, err := o.writer.CreateMailingList(ctx, toSend)
	if err != nil {
		return nil, err
	}

	mapped, err := o.mapMailingListResponse(ctx, resp)
	if err != nil {
		return nil, err
	}

	o.notifyCommitteeAdded(ctx, committeeUID(mapped))
	return mapped, nil
}

// UpdateMailingList updates a mailing list, mapping project_uid (v2) -> project_id (v1)
// and committee_uid (v2) -> committee_id (v1) before forwarding.
//
// Committee event logic:
//   - Fetches the committee UID before the update (oldCUID) and compares it with the
//     committee UID after the update (newCUID).
//   - If they match, no committee-related change occurred — skip event publishing.
//   - If they differ, three scenarios are possible:
//     1. Committee swapped (A -> B): notify A removed, notify B added.
//     2. Committee removed (A -> ""): notify A removed; add is a no-op (empty UID guard).
//     3. Committee added ("" -> B): remove is a no-op (empty UID guard); notify B added.
//   - notifyCommitteeRemoved checks whether the old committee still has other mailing lists
//     before publishing has_mailing_list=false, preventing incorrect flag clearing when a
//     committee is shared across multiple mailing lists.
//   - notifyCommitteeAdded always publishes has_mailing_list=true unconditionally.
func (o *GroupsIOMailingListOrchestrator) UpdateMailingList(ctx context.Context, mailingListID string, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	// Snapshot the current committee association before the update so we can detect changes.
	oldCUID := o.fetchCommitteeUID(ctx, mailingListID)

	toSend, err := o.mapMailingListRequest(ctx, ml)
	if err != nil {
		return nil, err
	}

	resp, err := o.writer.UpdateMailingList(ctx, mailingListID, toSend)
	if err != nil {
		return nil, err
	}

	mapped, err := o.mapMailingListResponse(ctx, resp)
	if err != nil {
		return nil, err
	}

	// Compare pre- and post-update committee UIDs to detect association changes.
	newCUID := committeeUID(mapped)
	if oldCUID != newCUID {
		o.notifyCommitteeRemoved(ctx, oldCUID, mailingListID)
		o.notifyCommitteeAdded(ctx, newCUID)
	}

	return mapped, nil
}

// DeleteMailingList deletes a mailing list and notifies the associated committee
// that a mailing list was removed. Only publishes has_mailing_list=false if no other
// mailing lists reference the committee.
func (o *GroupsIOMailingListOrchestrator) DeleteMailingList(ctx context.Context, mailingListID string) error {
	// Fetch current state before delete so we know which committee to notify.
	cUID := o.fetchCommitteeUID(ctx, mailingListID)

	if err := o.writer.DeleteMailingList(ctx, mailingListID); err != nil {
		return err
	}

	o.notifyCommitteeRemoved(ctx, cUID, mailingListID)
	return nil
}

// ---- Event publishing helpers ----

// notifyCommitteeAdded unconditionally publishes has_mailing_list=true for the given committee.
// No-op when cUID is empty or publisher is not configured.
func (o *GroupsIOMailingListOrchestrator) notifyCommitteeAdded(ctx context.Context, cUID string) {
	o.publishCommitteeMailingListChanged(ctx, cUID, true)
}

// notifyCommitteeRemoved publishes has_mailing_list=false for the given committee only if
// no other mailing lists still reference it. Checks remaining count and excludes the
// mailing list identified by excludeMLID (the one being deleted/modified) to guard against
// stale reads from ITX. No-op when cUID is empty.
func (o *GroupsIOMailingListOrchestrator) notifyCommitteeRemoved(ctx context.Context, cUID string, excludeMLID string) {
	if cUID == "" {
		return
	}
	if o.committeeHasRemainingMailingLists(ctx, cUID, excludeMLID) {
		return
	}
	o.publishCommitteeMailingListChanged(ctx, cUID, false)
}

// publishCommitteeMailingListChanged best-effort publishes a CommitteeMailingListChangedEvent.
// No-op when cUID is empty or publisher is not configured. The committee service's
// UpdateHasMailingList is the idempotency guard and skips the KV write + re-index if the
// flag already matches.
func (o *GroupsIOMailingListOrchestrator) publishCommitteeMailingListChanged(ctx context.Context, cUID string, hasMailingList bool) {
	if cUID == "" || o.publisher == nil {
		return
	}
	event := &model.CommitteeMailingListChangedEvent{
		CommitteeUID:   cUID,
		HasMailingList: hasMailingList,
	}
	if err := o.publisher.Internal(ctx, constants.CommitteeMailingListChangedSubject, event); err != nil {
		slog.ErrorContext(ctx, "failed to publish committee mailing list changed event",
			"committee_uid", cUID,
			"has_mailing_list", hasMailingList,
			"error", err)
	}
}

// fetchCommitteeUID reads the current committee UID for a mailing list.
// Returns "" if the reader is not configured or the fetch fails (non-fatal).
func (o *GroupsIOMailingListOrchestrator) fetchCommitteeUID(ctx context.Context, mailingListID string) string {
	if o.reader == nil {
		return ""
	}
	ml, err := o.reader.GetMailingList(ctx, mailingListID)
	if err != nil {
		slog.WarnContext(ctx, "failed to fetch mailing list before mutation — committee event may be skipped",
			"mailing_list_id", mailingListID, "error", err)
		return ""
	}
	return committeeUID(ml)
}

// committeeHasRemainingMailingLists checks whether the committee still has other mailing lists
// besides the one identified by excludeMLID. Returns true (assume others exist) on any error
// to avoid publishing a spurious has_mailing_list=false that would overwrite correct state.
func (o *GroupsIOMailingListOrchestrator) committeeHasRemainingMailingLists(ctx context.Context, cUID, excludeMLID string) bool {
	if o.reader == nil {
		return false
	}
	items, _, err := o.reader.ListMailingLists(ctx, "", cUID)
	if err != nil {
		slog.WarnContext(ctx, "failed to check remaining mailing lists for committee — skipping false event",
			"committee_uid", cUID, "error", err)
		return true // assume others exist when uncertain
	}
	for _, ml := range items {
		if ml.UID != excludeMLID {
			return true
		}
	}
	return false
}

// committeeUID extracts the first committee UID from a mailing list, or "".
func committeeUID(ml *model.GroupsIOMailingList) string {
	if ml != nil && len(ml.Committees) > 0 {
		return ml.Committees[0].UID
	}
	return ""
}

// ---- ID mapping helpers ----

// mapMailingListRequest copies the mailing list and translates v2 IDs to v1 before sending to ITX.
func (o *GroupsIOMailingListOrchestrator) mapMailingListRequest(ctx context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	toSend := *ml

	if ml.ProjectUID != "" {
		v1ID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, ml.ProjectUID)
		if err != nil {
			return nil, err
		}
		toSend.ProjectUID = v1ID
	}

	if len(ml.Committees) > 0 && ml.Committees[0].UID != "" {
		v1ID, err := o.translator.MapID(ctx, constants.TranslationSubjectCommittee, constants.TranslationDirectionV2ToV1, ml.Committees[0].UID)
		if err != nil {
			return nil, err
		}
		committees := make([]model.Committee, len(ml.Committees))
		copy(committees, ml.Committees)
		committees[0].UID = v1ID
		toSend.Committees = committees
	}

	return &toSend, nil
}

// mapMailingListResponse translates v1 IDs to v2 in a mailing list response from ITX.
func (o *GroupsIOMailingListOrchestrator) mapMailingListResponse(ctx context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	if ml == nil {
		return nil, nil
	}

	if ml.ProjectUID != "" {
		v2UID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV1ToV2, ml.ProjectUID)
		if err != nil {
			return nil, err
		}
		ml.ProjectUID = v2UID
	}

	if len(ml.Committees) > 0 && ml.Committees[0].UID != "" {
		v2UID, err := o.translator.MapID(ctx, constants.TranslationSubjectCommittee, constants.TranslationDirectionV1ToV2, ml.Committees[0].UID)
		if err != nil {
			return nil, err
		}
		ml.Committees[0].UID = v2UID
	}

	return ml, nil
}

// NewGroupsIOMailingListOrchestrator creates a new orchestrator with the given options.
func NewGroupsIOMailingListOrchestrator(opts ...MailingListOrchestratorOption) port.GroupsIOMailingListWriter {
	o := &GroupsIOMailingListOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
