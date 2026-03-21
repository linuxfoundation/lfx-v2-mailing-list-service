// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// GroupsIOMailingListReaderOrchestrator implements port.GroupsIOMailingListReader by wrapping an inner
// GroupsIOMailingListReader and translating v2 UUIDs to v1 SFIDs before forwarding requests.
type GroupsIOMailingListReaderOrchestrator struct {
	reader     port.GroupsIOMailingListReader
	translator port.Translator
}

// MailingListReaderOrchestratorOption configures a GroupsIOMailingListReaderOrchestrator.
type MailingListReaderOrchestratorOption func(*GroupsIOMailingListReaderOrchestrator)

// WithMailingListReader sets the underlying reader (e.g. the ITX proxy client).
func WithMailingListReader(r port.GroupsIOMailingListReader) MailingListReaderOrchestratorOption {
	return func(o *GroupsIOMailingListReaderOrchestrator) {
		o.reader = r
	}
}

// WithMailingListReaderTranslator sets the ID translator.
func WithMailingListReaderTranslator(t port.Translator) MailingListReaderOrchestratorOption {
	return func(o *GroupsIOMailingListReaderOrchestrator) {
		o.translator = t
	}
}

// ListMailingLists lists mailing lists, translating v2 projectUID and committeeUID to v1 before forwarding,
// then translating v1 IDs back to v2 in each response item.
func (o *GroupsIOMailingListReaderOrchestrator) ListMailingLists(ctx context.Context, projectUID string, committeeUID string) ([]*model.GroupsIOMailingList, int, error) {
	v1ProjectID := projectUID
	if projectUID != "" {
		id, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, projectUID)
		if err != nil {
			return nil, 0, err
		}
		v1ProjectID = id
	}

	v1CommitteeID := committeeUID
	if committeeUID != "" {
		id, err := o.translator.MapID(ctx, constants.TranslationSubjectCommittee, constants.TranslationDirectionV2ToV1, committeeUID)
		if err != nil {
			return nil, 0, err
		}
		v1CommitteeID = id
	}

	items, total, err := o.reader.ListMailingLists(ctx, v1ProjectID, v1CommitteeID)
	if err != nil {
		return nil, 0, err
	}

	for _, ml := range items {
		if err := o.translateMailingListResponse(ctx, ml); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

// GetMailingList retrieves a mailing list by ID and translates v1 IDs to v2 in the response.
func (o *GroupsIOMailingListReaderOrchestrator) GetMailingList(ctx context.Context, mailingListID string) (*model.GroupsIOMailingList, error) {
	ml, err := o.reader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return nil, err
	}
	if err := o.translateMailingListResponse(ctx, ml); err != nil {
		return nil, err
	}
	return ml, nil
}

// GetMailingListCount returns the count of mailing lists for a given v2 projectUID.
func (o *GroupsIOMailingListReaderOrchestrator) GetMailingListCount(ctx context.Context, projectUID string) (int, error) {
	v1ProjectID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, projectUID)
	if err != nil {
		return 0, err
	}
	return o.reader.GetMailingListCount(ctx, v1ProjectID)
}

// GetMailingListMemberCount returns the count of members in a given mailing list.
func (o *GroupsIOMailingListReaderOrchestrator) GetMailingListMemberCount(ctx context.Context, mailingListID string) (int, error) {
	return o.reader.GetMailingListMemberCount(ctx, mailingListID)
}

// translateMailingListResponse translates v1 IDs to v2 in-place on a mailing list response from ITX.
func (o *GroupsIOMailingListReaderOrchestrator) translateMailingListResponse(ctx context.Context, ml *model.GroupsIOMailingList) error {
	if ml.ProjectUID != "" {
		v2UID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV1ToV2, ml.ProjectUID)
		if err != nil {
			return err
		}
		ml.ProjectUID = v2UID
	}

	if len(ml.Committees) > 0 && ml.Committees[0].UID != "" {
		v2UID, err := o.translator.MapID(ctx, constants.TranslationSubjectCommittee, constants.TranslationDirectionV1ToV2, ml.Committees[0].UID)
		if err != nil {
			return err
		}
		ml.Committees[0].UID = v2UID
	}
	return nil
}

// NewGroupsIOMailingListReaderOrchestrator creates a new reader orchestrator with the given options.
func NewGroupsIOMailingListReaderOrchestrator(opts ...MailingListReaderOrchestratorOption) port.GroupsIOMailingListReader {
	o := &GroupsIOMailingListReaderOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
