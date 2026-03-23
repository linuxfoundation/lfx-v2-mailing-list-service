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

// GroupsIOServiceWriterOrchestrator implements port.GrpsIOServiceWriter by wrapping an inner
// GrpsIOServiceWriter and translating v2 UUIDs to v1 SFIDs before forwarding requests.
type GroupsIOServiceWriterOrchestrator struct {
	writer     port.GroupsIOServiceWriter
	translator port.Translator
}

// ServiceWriterOrchestratorOption configures a GroupsIOServiceWriterOrchestrator.
type ServiceWriterOrchestratorOption func(*GroupsIOServiceWriterOrchestrator)

// WithServiceWriter sets the underlying writer (e.g. the ITX proxy client).
func WithServiceWriter(w port.GroupsIOServiceWriter) ServiceWriterOrchestratorOption {
	return func(o *GroupsIOServiceWriterOrchestrator) {
		o.writer = w
	}
}

// WithServiceTranslator sets the ID translator.
func WithServiceTranslator(t port.Translator) ServiceWriterOrchestratorOption {
	return func(o *GroupsIOServiceWriterOrchestrator) {
		o.translator = t
	}
}

// CreateService creates a new GroupsIO service, mapping project_uid (v2) -> project_id (v1).
func (o *GroupsIOServiceWriterOrchestrator) CreateService(ctx context.Context, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	toSend := *svc
	if svc.ProjectUID != "" {
		v1ID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, svc.ProjectUID)
		if err != nil {
			return nil, err
		}
		toSend.ProjectUID = v1ID
	}

	resp, err := o.writer.CreateService(ctx, &toSend)
	if err != nil {
		return nil, err
	}

	return mapServiceResponse(ctx, o.translator, resp)
}

// UpdateService updates a GroupsIO service, mapping project_uid (v2) -> project_id (v1).
func (o *GroupsIOServiceWriterOrchestrator) UpdateService(ctx context.Context, serviceID string, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	toSend := *svc
	if svc.ProjectUID != "" {
		v1ID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, svc.ProjectUID)
		if err != nil {
			return nil, err
		}
		toSend.ProjectUID = v1ID
	}

	resp, err := o.writer.UpdateService(ctx, serviceID, &toSend)
	if err != nil {
		return nil, err
	}

	return mapServiceResponse(ctx, o.translator, resp)
}

// DeleteService deletes a GroupsIO service.
func (o *GroupsIOServiceWriterOrchestrator) DeleteService(ctx context.Context, serviceID string) error {
	return o.writer.DeleteService(ctx, serviceID)
}

// mapServiceResponse maps project_id (v1) -> project_uid (v2) in a service response.
func mapServiceResponse(ctx context.Context, translator port.Translator, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	if svc == nil {
		return nil, nil
	}
	if svc.ProjectUID != "" {
		v2UID, err := translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV1ToV2, svc.ProjectUID)
		if err != nil {
			return nil, err
		}
		copy := *svc
		copy.ProjectUID = v2UID
		return &copy, nil
	}
	return svc, nil
}

// NewGroupsIOServiceWriterOrchestrator creates a new orchestrator with the given options.
func NewGroupsIOServiceWriterOrchestrator(opts ...ServiceWriterOrchestratorOption) *GroupsIOServiceWriterOrchestrator {
	o := &GroupsIOServiceWriterOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
