// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRealtimeAutoSyncer is a test double for port.RealtimeAutoSyncer.
type fakeRealtimeAutoSyncer struct {
	gotMailingListID string
	err              error
}

func (f *fakeRealtimeAutoSyncer) TriggerAutoSync(_ context.Context, mailingListID string) error {
	f.gotMailingListID = mailingListID
	return f.err
}

func TestTriggerGroupsioAutoSync(t *testing.T) {
	tests := []struct {
		name        string
		syncerErr   error
		expectErrAs any
	}{
		{
			name: "success forwards subgroup id and returns no error",
		},
		{
			name:        "not found error is mapped to NotFoundError",
			syncerErr:   errs.NewNotFound("mailing list not found"),
			expectErrAs: &mailinglist.NotFoundError{},
		},
		{
			name:        "unexpected error is mapped to InternalServerError",
			syncerErr:   errors.New("boom"),
			expectErrAs: &mailinglist.InternalServerError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncer := &fakeRealtimeAutoSyncer{err: tt.syncerErr}
			s := &mailingListAPI{realtimeAutoSyncer: syncer}

			err := s.TriggerGroupsioAutoSync(context.Background(), &mailinglist.TriggerGroupsioAutoSyncPayload{
				SubgroupID: "subgroup-123",
			})

			assert.Equal(t, "subgroup-123", syncer.gotMailingListID)

			if tt.expectErrAs == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorAs(t, err, &tt.expectErrAs)
		})
	}
}
