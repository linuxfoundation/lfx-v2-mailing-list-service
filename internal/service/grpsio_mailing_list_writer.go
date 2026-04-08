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

	o.publishCommitteeMailingListChanged(ctx, committeeUID(mapped), true)
	return mapped, nil
}

// UpdateMailingList updates a mailing list, mapping project_uid (v2) -> project_id (v1)
// and committee_uid (v2) -> committee_id (v1) before forwarding.
// If the committee association changed it publishes the appropriate status events.
func (o *GroupsIOMailingListOrchestrator) UpdateMailingList(ctx context.Context, mailingListID string, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	// Fetch current state before update to detect committee association changes.
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

	newCUID := committeeUID(mapped)
	if oldCUID != newCUID {
		o.publishCommitteeMailingListChanged(ctx, oldCUID, false)
		o.publishCommitteeMailingListChanged(ctx, newCUID, true)
	}

	return mapped, nil
}

// DeleteMailingList deletes a mailing list and publishes a committee status event
// for the previously associated committee (if any).
func (o *GroupsIOMailingListOrchestrator) DeleteMailingList(ctx context.Context, mailingListID string) error {
	// Fetch current state before delete so we know which committee to notify.
	cUID := o.fetchCommitteeUID(ctx, mailingListID)

	if err := o.writer.DeleteMailingList(ctx, mailingListID); err != nil {
		return err
	}

	o.publishCommitteeMailingListChanged(ctx, cUID, false)
	return nil
}

// ---- Event publishing helpers ----

// publishCommitteeMailingListChanged best-effort publishes a CommitteeMailingListChangedEvent
// when a committee UID is present and a publisher is configured.
// It does not perform local deduplication; the committee service's UpdateHasMailingList is
// the idempotency guard and skips the KV write + re-index if the flag already matches.
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
