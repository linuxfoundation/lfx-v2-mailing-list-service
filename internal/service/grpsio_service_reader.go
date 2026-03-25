// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// GroupsIOServiceReaderOrchestrator implements port.GroupsIOServiceReader by wrapping an inner
// GroupsIOServiceReader and translating v1 SFIDs to v2 UUIDs in responses.
type GroupsIOServiceReaderOrchestrator struct {
	reader     port.GroupsIOServiceReader
	translator port.Translator
}

// ServiceReaderOrchestratorOption configures a GroupsIOServiceReaderOrchestrator.
type ServiceReaderOrchestratorOption func(*GroupsIOServiceReaderOrchestrator)

// WithServiceReader sets the underlying reader (e.g. the ITX proxy client).
func WithServiceReader(r port.GroupsIOServiceReader) ServiceReaderOrchestratorOption {
	return func(o *GroupsIOServiceReaderOrchestrator) {
		o.reader = r
	}
}

// WithServiceReaderTranslator sets the ID translator.
func WithServiceReaderTranslator(t port.Translator) ServiceReaderOrchestratorOption {
	return func(o *GroupsIOServiceReaderOrchestrator) {
		o.translator = t
	}
}

// ListServices lists GroupsIO services, mapping project_uid (v2) -> project_id (v1) in the
// request and project_id (v1) -> project_uid (v2) in each response.
func (o *GroupsIOServiceReaderOrchestrator) ListServices(ctx context.Context, projectUID string) ([]*model.GroupsIOService, int, error) {
	v1ProjectID := projectUID
	if projectUID != "" {
		v1ID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, projectUID)
		if err != nil {
			return nil, 0, err
		}
		v1ProjectID = v1ID
	}

	svcs, total, err := o.reader.ListServices(ctx, v1ProjectID)
	if err != nil {
		return nil, 0, err
	}

	for i, svc := range svcs {
		mapped, err := mapServiceResponse(ctx, o.translator, svc)
		if err != nil {
			return nil, 0, err
		}
		svcs[i] = mapped
	}

	return svcs, total, nil
}

// GetService retrieves a GroupsIO service by ID, mapping project_id (v1) -> project_uid (v2)
// in the response.
func (o *GroupsIOServiceReaderOrchestrator) GetService(ctx context.Context, serviceID string) (*model.GroupsIOService, error) {
	svc, err := o.reader.GetService(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	return mapServiceResponse(ctx, o.translator, svc)
}

// GetProjects returns v2 project UIDs that have GroupsIO services, translating
// v1 project IDs -> v2 UUIDs.
func (o *GroupsIOServiceReaderOrchestrator) GetProjects(ctx context.Context) ([]string, error) {
	v1ProjectIDs, err := o.reader.GetProjects(ctx)
	if err != nil {
		return nil, err
	}

	v2UIDs := make([]string, len(v1ProjectIDs))
	for i, v1ID := range v1ProjectIDs {
		v2UID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV1ToV2, v1ID)
		if err != nil {
			return nil, err
		}
		v2UIDs[i] = v2UID
	}

	return v2UIDs, nil
}

// FindParentService finds the parent service for a project, mapping project_uid (v2) -> project_id (v1)
// in the request and project_id (v1) -> project_uid (v2) in the response.
func (o *GroupsIOServiceReaderOrchestrator) FindParentService(ctx context.Context, projectUID string) (*model.GroupsIOService, error) {
	v1ID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, projectUID)
	if err != nil {
		return nil, err
	}

	svc, err := o.reader.FindParentService(ctx, v1ID)
	if err != nil {
		return nil, err
	}

	return mapServiceResponse(ctx, o.translator, svc)
}


// NewGroupsIOServiceReaderOrchestrator creates a new reader orchestrator with the given options.
func NewGroupsIOServiceReaderOrchestrator(opts ...ServiceReaderOrchestratorOption) *GroupsIOServiceReaderOrchestrator {
	o := &GroupsIOServiceReaderOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
