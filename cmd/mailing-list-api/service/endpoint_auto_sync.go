// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
)

// TriggerGroupsioAutoSync triggers an immediate committee-membership auto-sync reconciliation for
// a GroupsIO subgroup. Called by ITX's committee-association Lambda in reaction to a Groups.io
// subscribe/unsubscribe event it observed on its own SNS topic (issue #4023) — Groups.io itself
// never calls this service directly.
//
// TriggerAutoSync is itself a no-op for a mailing list that has no linked committee or whose
// sync_mode is not auto_ml_to_c, so no additional filtering is needed here before calling it.
func (s *mailingListAPI) TriggerGroupsioAutoSync(ctx context.Context, p *mailinglist.TriggerGroupsioAutoSyncPayload) error {
	if err := s.realtimeAutoSyncer.TriggerAutoSync(ctx, p.SubgroupID); err != nil {
		return mapDomainError(err)
	}
	return nil
}
