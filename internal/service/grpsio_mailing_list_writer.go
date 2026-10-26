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
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// GroupsIOMailingListOrchestrator implements port.GroupsIOMailingListWriter by wrapping an inner
// GroupsIOMailingListWriter and translating v2 UUIDs to v1 SFIDs before forwarding requests.
// It also publishes committee mailing list status events after each mutation.
type GroupsIOMailingListOrchestrator struct {
	writer                 port.GroupsIOMailingListWriter
	reader                 port.GroupsIOMailingListReader
	translator             port.Translator
	publisher              port.MessagePublisher
	serviceReader          port.GroupsIOServiceReader
	committeeProjectLookup port.CommitteeProjectLookup
	syncSettingsStore      port.CommitteeSyncSettingsStore
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

// WithMailingListServiceReader sets the service reader used to resolve a service's project.
func WithMailingListServiceReader(r port.GroupsIOServiceReader) MailingListOrchestratorOption {
	return func(o *GroupsIOMailingListOrchestrator) {
		o.serviceReader = r
	}
}

// WithMailingListCommitteeProjectLookup sets the lookup used to resolve a committee's project.
func WithMailingListCommitteeProjectLookup(l port.CommitteeProjectLookup) MailingListOrchestratorOption {
	return func(o *GroupsIOMailingListOrchestrator) {
		o.committeeProjectLookup = l
	}
}

// WithMailingListSyncSettingsStore sets the store used to persist each committee's sync_mode.
// ITX has no concept of sync_mode, so it must be stored and merged in separately.
func WithMailingListSyncSettingsStore(s port.CommitteeSyncSettingsStore) MailingListOrchestratorOption {
	return func(o *GroupsIOMailingListOrchestrator) {
		o.syncSettingsStore = s
	}
}

// validateCommitteeProject checks that the supplied committee belongs to the same project as
// the parent service. No-op when no committee is present. Returns ServiceUnavailable when
// required dependencies are not configured. On success it sets ml.ProjectUID from the
// authoritative service record.
func (o *GroupsIOMailingListOrchestrator) validateCommitteeProject(ctx context.Context, ml *model.GroupsIOMailingList) error {
	if len(ml.Committees) == 0 {
		return nil
	}
	if ml.Committees[0].UID == "" {
		if ml.Committees[0].SyncMode != "" {
			return errs.NewValidation("sync_mode requires committee_uid")
		}
		return nil
	}
	if syncMode := ml.Committees[0].SyncMode; syncMode != "" && !isValidSyncMode(syncMode) {
		return errs.NewValidation("invalid sync_mode")
	}
	if o.serviceReader == nil || o.committeeProjectLookup == nil {
		return errs.NewServiceUnavailable("committee project validation is not configured")
	}

	if ml.ServiceUID == "" {
		return errs.NewValidation("service_id is required when committee_uid is provided")
	}

	svc, err := o.serviceReader.GetService(ctx, ml.ServiceUID)
	if err != nil {
		return err
	}
	if svc == nil {
		return errs.NewServiceUnavailable("service reader returned nil service")
	}

	committeeProject, err := o.committeeProjectLookup.GetCommitteeProject(ctx, ml.Committees[0].UID)
	if err != nil {
		return err
	}

	if committeeProject != svc.ProjectUID {
		return errs.NewValidation("committee does not belong to the service's project")
	}

	ml.ProjectUID = svc.ProjectUID
	return nil
}

// CreateMailingList creates a new mailing list, mapping project_uid (v2) -> project_id (v1)
// and committee_uid (v2) -> committee_id (v1) before forwarding.
// After a successful create it publishes a committee mailing list status event.
func (o *GroupsIOMailingListOrchestrator) CreateMailingList(ctx context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	if err := o.validateCommitteeProject(ctx, ml); err != nil {
		return nil, err
	}

	requestedSyncMode := ""
	if len(ml.Committees) > 0 {
		requestedSyncMode = ml.Committees[0].SyncMode
	}

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

	cUID := committeeUID(mapped)
	persisted := o.persistSyncMode(ctx, mapped.UID, cUID, requestedSyncMode)
	if persisted && requestedSyncMode != "" && len(mapped.Committees) > 0 {
		mapped.Committees[0].SyncMode = requestedSyncMode
	}

	o.notifyCommitteeAdded(ctx, cUID)
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
	if err := o.validateCommitteeProject(ctx, ml); err != nil {
		return nil, err
	}

	// Snapshot the current committee association, and the revision of its stored sync_mode,
	// before the update runs. The revision must be captured here — not later, at cleanup time
	// — so a concurrent request that re-associates the mailing list back to oldCUID and writes
	// a newer sync_mode while this update is in flight is detected by the gated delete below
	// instead of racing past it (see LFXV2-4029 review).
	oldCUID := o.fetchCommitteeUID(ctx, mailingListID)
	oldSyncModeSnapshot := o.snapshotSyncMode(ctx, mailingListID, oldCUID)

	requestedSyncMode := ""
	requestedCUID := ""
	if len(ml.Committees) > 0 {
		requestedSyncMode = ml.Committees[0].SyncMode
		requestedCUID = ml.Committees[0].UID
	}

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
		if oldCUID != "" {
			o.cleanupStaleSyncMode(ctx, mailingListID, oldCUID, oldSyncModeSnapshot, true)
		}
	}

	persisted := true
	if requestedSyncMode != "" {
		if newCUID == requestedCUID {
			persisted = o.persistSyncMode(ctx, mailingListID, newCUID, requestedSyncMode)
		} else {
			// ITX's PUT returns 204 No Content, and the writer issues a separate GET to fetch
			// the updated resource (see itx.go UpdateMailingList). A concurrent update to this
			// same mailing list can land between that PUT and GET, so the committee reflected
			// in resp/mapped may not be the one this request validated and asked to set
			// sync_mode for. Applying requestedSyncMode to newCUID in that case would silently
			// overwrite a different, unrelated committee's stored setting (see LFXV2-4029 review).
			slog.WarnContext(ctx, "returned committee does not match requested committee; skipping sync_mode persist",
				"mailing_list_id", mailingListID, "requested_committee_uid", requestedCUID, "returned_committee_uid", newCUID)
			persisted = false
		}
	}
	if persisted && requestedSyncMode != "" && len(mapped.Committees) > 0 {
		mapped.Committees[0].SyncMode = requestedSyncMode
	} else if len(mapped.Committees) > 0 {
		// The request either omitted sync_mode (preserving whatever is stored) or the
		// persist above failed — either way, echo back the store's actual current value
		// rather than a value that may not match what's actually persisted.
		o.hydrateSyncModeFromStore(ctx, mailingListID, newCUID, mapped)
	}

	return mapped, nil
}

// syncModeSnapshot captures whether a sync_mode was stored for a (mailing list, committee)
// pair, and at what revision, at a specific point in time — so a later gated delete can tell
// whether a concurrent write has superseded it since.
type syncModeSnapshot struct {
	revision uint64
	found    bool
}

// snapshotSyncMode reads the current revision of the sync_mode stored for mailingListID+cUID,
// if any. Must be called before the mutation that may change the mailing list's committee
// association, not after — see cleanupStaleSyncMode. A lookup failure here is non-fatal to the
// caller's request: it yields a not-found snapshot, which simply skips cleanup rather than
// risking an incorrect delete.
func (o *GroupsIOMailingListOrchestrator) snapshotSyncMode(ctx context.Context, mailingListID, cUID string) syncModeSnapshot {
	if cUID == "" || o.syncSettingsStore == nil {
		return syncModeSnapshot{}
	}
	_, revision, found, err := o.syncSettingsStore.GetSyncMode(ctx, mailingListID, cUID)
	if err != nil {
		slog.WarnContext(ctx, "failed to snapshot committee sync_mode revision before mutation",
			"mailing_list_id", mailingListID, "committee_uid", cUID, "error", err)
		return syncModeSnapshot{}
	}
	return syncModeSnapshot{revision: revision, found: found}
}

// cleanupStaleSyncMode removes the sync_mode previously stored for mailingListID+oldCUID after
// the mailing list's committee association has changed. snapshot must have been captured by
// snapshotSyncMode before the mutation that changed the association ran — reading the revision
// here, after the mutation, would leave a window where a concurrent request that re-associates
// the mailing list back to oldCUID and writes a newer sync_mode in between gets its write
// silently deleted by this cleanup (see LFXV2-4029 review). Gating the delete on a
// pre-mutation revision closes that window: a concurrent write bumps the revision past what was
// snapshotted, so the delete is skipped instead of clobbering it.
//
// That revision check alone does not close every ordering: a concurrent request can move the
// association away from oldCUID and back again (B -> A -> B) without ever writing a new
// sync_mode for oldCUID, leaving the revision unchanged while oldCUID has become the *current*
// association again by the time this delayed cleanup runs. Re-checking the live association
// immediately before the delete closes that specific case — if the mailing list has already
// reverted to oldCUID, its setting is active again and must not be deleted, regardless of
// revision. This added check still has its own (narrower) TOCTOU gap between the re-check and
// the delete; closing that fully requires serializing association mutations per mailing list or
// gating the delete on an association-generation value, which is a larger change tracked
// separately (see LFXV2-4029 review, copilot-pull-request-reviewer).
//
// checkLiveAssociation gates that re-check and must be false when the mailing list itself was
// just deleted (DeleteMailingList): there is no "current association" to revert to once the
// record is gone, and a reader that still resolves the now-deleted record (a stale cache, a
// lagging index) would otherwise cause this cleanup to skip forever, leaking the orphaned entry.
func (o *GroupsIOMailingListOrchestrator) cleanupStaleSyncMode(ctx context.Context, mailingListID, oldCUID string, snapshot syncModeSnapshot, checkLiveAssociation bool) {
	if o.syncSettingsStore == nil || !snapshot.found {
		return
	}
	if checkLiveAssociation {
		if current := o.fetchCommitteeUID(ctx, mailingListID); current == oldCUID {
			slog.InfoContext(ctx, "skipped cleaning up committee sync_mode: mailing list has already reverted to this committee",
				"mailing_list_id", mailingListID, "committee_uid", oldCUID)
			return
		}
	}
	deleted, err := o.syncSettingsStore.DeleteSyncModeIfUnchanged(ctx, mailingListID, oldCUID, snapshot.revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to clean up stale committee sync_mode",
			"mailing_list_id", mailingListID, "committee_uid", oldCUID, "error", err)
		return
	}
	if !deleted {
		slog.InfoContext(ctx, "skipped cleaning up committee sync_mode: a newer value was written concurrently",
			"mailing_list_id", mailingListID, "committee_uid", oldCUID)
	}
}

// hydrateSyncModeFromStore sets mapped.Committees[0].SyncMode from the store's current value for
// mailingListID+cUID, if any. Used whenever the response field must reflect the stored state
// rather than a possibly-unapplied request value (sync_mode omitted from the request, or a
// persist failure above).
func (o *GroupsIOMailingListOrchestrator) hydrateSyncModeFromStore(ctx context.Context, mailingListID, cUID string, mapped *model.GroupsIOMailingList) {
	if cUID == "" || o.syncSettingsStore == nil {
		return
	}
	mode, _, ok, err := o.syncSettingsStore.GetSyncMode(ctx, mailingListID, cUID)
	if err != nil {
		slog.WarnContext(ctx, "failed to hydrate committee sync_mode into response",
			"mailing_list_id", mailingListID, "committee_uid", cUID, "error", err)
		return
	}
	if ok {
		mapped.Committees[0].SyncMode = mode
	}
}

// DeleteMailingList deletes a mailing list and notifies the associated committee
// that a mailing list was removed. Only publishes has_mailing_list=false if no other
// mailing lists reference the committee.
func (o *GroupsIOMailingListOrchestrator) DeleteMailingList(ctx context.Context, mailingListID string) error {
	// Fetch current state, and snapshot its stored sync_mode revision, before delete runs —
	// see cleanupStaleSyncMode for why the snapshot must be taken before the mutation.
	cUID := o.fetchCommitteeUID(ctx, mailingListID)
	syncModeSnap := o.snapshotSyncMode(ctx, mailingListID, cUID)

	if err := o.writer.DeleteMailingList(ctx, mailingListID); err != nil {
		return err
	}

	o.notifyCommitteeRemoved(ctx, cUID, mailingListID)
	if cUID != "" {
		o.cleanupStaleSyncMode(ctx, mailingListID, cUID, syncModeSnap, false)
	}
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

// isValidSyncMode reports whether syncMode is one of model.ValidSyncModes.
func isValidSyncMode(syncMode string) bool {
	for _, valid := range model.ValidSyncModes {
		if syncMode == valid {
			return true
		}
	}
	return false
}

// persistSyncMode stores the requested sync_mode for mailingListID+cUID, best-effort logged on
// failure (ITX has already committed the rest of the mutation, so we don't fail the request).
// Returns whether the value was actually persisted, so callers can avoid echoing a mode that
// isn't really stored. No-op (returns false) when no sync_mode was requested, no committee is
// attached, or the store isn't configured.
func (o *GroupsIOMailingListOrchestrator) persistSyncMode(ctx context.Context, mailingListID, cUID, syncMode string) bool {
	if cUID == "" || syncMode == "" {
		return false
	}
	if o.syncSettingsStore == nil {
		slog.WarnContext(ctx, "sync_mode requested but no sync settings store is configured; value will not persist",
			"mailing_list_id", mailingListID, "committee_uid", cUID)
		return false
	}
	if err := o.syncSettingsStore.PutSyncMode(ctx, mailingListID, cUID, syncMode); err != nil {
		slog.ErrorContext(ctx, "failed to persist committee sync_mode",
			"mailing_list_id", mailingListID, "committee_uid", cUID, "error", err)
		return false
	}
	return true
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
