// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"strconv"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
)

// ---- wire ↔ domain translation helpers ----

func fromWireService(w *serviceWire) *model.GroupsIOService {
	if w == nil {
		return nil
	}
	return &model.GroupsIOService{
		UID:        w.ID,
		ProjectUID: w.ProjectID,
		Type:       w.Type,
		GroupID:    converter.NonZeroInt64(w.GroupID),
		Domain:     w.Domain,
		Prefix:     w.Prefix,
		Status:     w.Status,
	}
}

func toWireServiceRequest(svc *model.GroupsIOService) *serviceRequestWire {
	return &serviceRequestWire{
		ProjectID: svc.ProjectUID,
		Type:      svc.Type,
		GroupID:   converter.Int64Val(svc.GroupID),
		Domain:    svc.Domain,
		Prefix:    svc.Prefix,
		Status:    svc.Status,
	}
}

func fromWireSubgroup(w *subgroupWire) *model.GroupsIOMailingList {
	if w == nil {
		return nil
	}
	// ITX identifies mailing lists by their numeric group_id, not a UUID.
	uid := strconv.FormatInt(w.GroupID, 10)
	createdAt, _ := converter.ParseRFC3339(w.CreatedAt)
	updatedAt, _ := converter.ParseRFC3339(w.UpdatedAt)
	ml := &model.GroupsIOMailingList{
		UID:            uid,
		ProjectUID:     w.ProjectID,
		ServiceUID:     w.ParentID,
		GroupID:        converter.NonZeroInt64(w.GroupID),
		GroupName:      w.Name,
		Description:    w.Description,
		Type:           w.Type,
		AudienceAccess: w.AudienceAccess,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
	if w.CommitteeID != "" {
		ml.Committees = []model.Committee{{UID: w.CommitteeID}}
	}
	return ml
}

func toWireSubgroupRequest(ml *model.GroupsIOMailingList) *subgroupRequestWire {
	req := &subgroupRequestWire{
		ProjectID:      ml.ProjectUID,
		ParentID:       ml.ServiceUID,
		Name:           ml.GroupName,
		Description:    ml.Description,
		Type:           ml.Type,
		AudienceAccess: ml.AudienceAccess,
	}
	if len(ml.Committees) > 0 {
		req.CommitteeID = ml.Committees[0].UID
	}
	return req
}