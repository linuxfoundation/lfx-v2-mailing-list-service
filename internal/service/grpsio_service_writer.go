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

// GroupsIOServiceOrchestrator implements port.GrpsIOServiceWriter by wrapping an inner
// GrpsIOServiceWriter and translating v2 UUIDs to v1 SFIDs before forwarding requests.
type GroupsIOServiceOrchestrator struct {
	writer     port.GroupsIOServiceWriter
	translator port.Translator
}

// ServiceOrchestratorOption configures a GroupsIOServiceOrchestrator.
type ServiceOrchestratorOption func(*GroupsIOServiceOrchestrator)

// WithServiceWriter sets the underlying writer (e.g. the ITX proxy client).
func WithServiceWriter(w port.GroupsIOServiceWriter) ServiceOrchestratorOption {
	return func(o *GroupsIOServiceOrchestrator) {
		o.writer = w
	}
}

// WithServiceTranslator sets the ID translator.
func WithServiceTranslator(t port.Translator) ServiceOrchestratorOption {
	return func(o *GroupsIOServiceOrchestrator) {
		o.translator = t
	}
}

// CreateService creates a new GroupsIO service, mapping project_uid (v2) -> project_id (v1).
func (o *GroupsIOServiceOrchestrator) CreateService(ctx context.Context, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
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

	return o.mapServiceResponse(ctx, resp)
}

// UpdateService updates a GroupsIO service, mapping project_uid (v2) -> project_id (v1).
func (o *GroupsIOServiceOrchestrator) UpdateService(ctx context.Context, serviceID string, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
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

	return o.mapServiceResponse(ctx, resp)
}

// DeleteService deletes a GroupsIO service.
func (o *GroupsIOServiceOrchestrator) DeleteService(ctx context.Context, serviceID string) error {
	return o.writer.DeleteService(ctx, serviceID)
}

// mapServiceResponse maps project_id (v1) -> project_uid (v2) in a service response.
func (o *GroupsIOServiceOrchestrator) mapServiceResponse(ctx context.Context, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	if svc == nil {
		return nil, nil
	}
	if svc.ProjectUID != "" {
		v2UID, err := o.translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV1ToV2, svc.ProjectUID)
		if err != nil {
			return nil, err
		}
		copy := *svc
		copy.ProjectUID = v2UID
		return &copy, nil
	}
	return svc, nil
}

// NewGroupsIOServiceOrchestrator creates a new orchestrator with the given options.
func NewGroupsIOServiceOrchestrator(opts ...ServiceOrchestratorOption) *GroupsIOServiceOrchestrator {
	o := &GroupsIOServiceOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
