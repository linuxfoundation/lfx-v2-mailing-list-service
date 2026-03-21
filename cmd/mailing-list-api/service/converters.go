// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

func convertService(svc *model.GroupsIOService) *mailinglist.GroupsioService {
	if svc == nil {
		return nil
	}
	createdAt := svc.CreatedAt.Format(time.RFC3339)
	updatedAt := ""
	if !svc.UpdatedAt.IsZero() {
		updatedAt = svc.UpdatedAt.Format(time.RFC3339)
	}
	return &mailinglist.GroupsioService{
		ID:         &svc.UID,
		ProjectUID: &svc.ProjectUID,
		Type:       &svc.Type,
		GroupID:    svc.GroupID,
		Domain:     &svc.Domain,
		Prefix:     &svc.Prefix,
		Status:     &svc.Status,
		CreatedAt:  &createdAt,
		UpdatedAt:  converter.NonEmptyString(updatedAt),
	}
}
