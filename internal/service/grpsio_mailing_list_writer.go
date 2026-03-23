// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service provides application service implementations.
package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// GroupsIOMailingListOrchestrator implements port.GroupsIOMailingListWriter by wrapping an inner
// GroupsIOMailingListWriter and translating v2 UUIDs to v1 SFIDs before forwarding requests.
type GroupsIOMailingListOrchestrator struct {
	writer     port.GroupsIOMailingListWriter
	translator port.Translator
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

// CreateMailingList creates a new mailing list, mapping project_uid (v2) -> project_id (v1)
// and committee_uid (v2) -> committee_id (v1) before forwarding.
func (o *GroupsIOMailingListOrchestrator) CreateMailingList(ctx context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	toSend, err := o.mapMailingListRequest(ctx, ml)
	if err != nil {
		return nil, err
	}

	resp, err := o.writer.CreateMailingList(ctx, toSend)
	if err != nil {
		return nil, err
	}

	return o.mapMailingListResponse(ctx, resp)
}

// UpdateMailingList updates a mailing list, mapping project_uid (v2) -> project_id (v1)
// and committee_uid (v2) -> committee_id (v1) before forwarding.
func (o *GroupsIOMailingListOrchestrator) UpdateMailingList(ctx context.Context, mailingListID string, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	toSend, err := o.mapMailingListRequest(ctx, ml)
	if err != nil {
		return nil, err
	}

	resp, err := o.writer.UpdateMailingList(ctx, mailingListID, toSend)
	if err != nil {
		return nil, err
	}

	return o.mapMailingListResponse(ctx, resp)
}

// DeleteMailingList deletes a mailing list.
func (o *GroupsIOMailingListOrchestrator) DeleteMailingList(ctx context.Context, mailingListID string) error {
	return o.writer.DeleteMailingList(ctx, mailingListID)
}

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
