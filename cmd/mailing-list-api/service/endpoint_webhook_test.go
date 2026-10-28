// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// stubWebhookMailingListReader is a minimal port.GroupsIOMailingListReader for the webhook
// handler tests: only GetMailingList is exercised.
type stubWebhookMailingListReader struct {
	ml  *model.GroupsIOMailingList
	err error
}

func (r *stubWebhookMailingListReader) ListMailingLists(_ context.Context, _, _ string) ([]*model.GroupsIOMailingList, int, error) {
	return nil, 0, nil
}
func (r *stubWebhookMailingListReader) GetMailingList(_ context.Context, mailingListID string) (*model.GroupsIOMailingList, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.ml, nil
}
func (r *stubWebhookMailingListReader) GetMailingListCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (r *stubWebhookMailingListReader) GetMailingListMemberCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// stubWebhookAutoSyncer records TriggerAutoSync calls and returns a configured error.
type stubWebhookAutoSyncer struct {
	calls []string
	err   error
}

func (s *stubWebhookAutoSyncer) TriggerAutoSync(_ context.Context, mailingListID string) error {
	s.calls = append(s.calls, mailingListID)
	return s.err
}

func autoML2CList() *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		UID:        "12345",
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeAutoMailingListToCommittee}},
	}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyGroupsIOWebhookSignature(t *testing.T) {
	body := []byte(`{"action":"added_member"}`)
	secret := "shhh"
	validSig := sign(secret, body)

	tests := []struct {
		name    string
		body    []byte
		sig     *string
		secret  string
		wantErr bool
	}{
		{name: "valid signature", body: body, sig: &validSig, secret: secret},
		{name: "missing secret", body: body, sig: &validSig, secret: "", wantErr: true},
		{name: "nil signature", body: body, sig: nil, secret: secret, wantErr: true},
		{name: "empty signature", body: body, sig: strPtr(""), secret: secret, wantErr: true},
		{name: "mismatched signature", body: body, sig: strPtr("deadbeef"), secret: secret, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyGroupsIOWebhookSignature(tt.body, tt.sig, tt.secret)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func strPtr(s string) *string { return &s }

func TestProcessGroupsIOWebhookEvent_IgnoresNonMembershipActions(t *testing.T) {
	mlReader := &stubWebhookMailingListReader{}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupCreatedEvent, MemberInfo: &model.MemberInfo{GroupID: 12345}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_MissingMemberInfo_Discarded(t *testing.T) {
	mlReader := &stubWebhookMailingListReader{}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberAddedEvent}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_MissingGroupID_Discarded(t *testing.T) {
	mlReader := &stubWebhookMailingListReader{}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberAddedEvent, MemberInfo: &model.MemberInfo{GroupID: 0}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_GetMailingListError_Propagates(t *testing.T) {
	mlReader := &stubWebhookMailingListReader{err: errors.New("not found")}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberAddedEvent, MemberInfo: &model.MemberInfo{GroupID: 12345}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.Error(t, err)
	assert.Empty(t, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_NotConfiguredForAutoML2C_Ignored(t *testing.T) {
	ml := &model.GroupsIOMailingList{UID: "12345", Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeManual}}}
	mlReader := &stubWebhookMailingListReader{ml: ml}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberAddedEvent, MemberInfo: &model.MemberInfo{GroupID: 12345}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_NoLinkedCommittee_Ignored(t *testing.T) {
	ml := &model.GroupsIOMailingList{UID: "12345"}
	mlReader := &stubWebhookMailingListReader{ml: ml}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberRemovedEvent, MemberInfo: &model.MemberInfo{GroupID: 12345}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_ConfiguredForAutoML2C_TriggersAutoSync(t *testing.T) {
	ml := autoML2CList()
	mlReader := &stubWebhookMailingListReader{ml: ml}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberAddedEvent, MemberInfo: &model.MemberInfo{GroupID: 12345}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.NoError(t, err)
	require.Equal(t, []string{"12345"}, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_RemovedMember_TriggersAutoSync(t *testing.T) {
	ml := autoML2CList()
	mlReader := &stubWebhookMailingListReader{ml: ml}
	autoSyncer := &stubWebhookAutoSyncer{}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberRemovedEvent, MemberInfo: &model.MemberInfo{GroupID: 12345}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.NoError(t, err)
	require.Equal(t, []string{"12345"}, autoSyncer.calls)
}

func TestProcessGroupsIOWebhookEvent_TriggerAutoSyncError_Propagates(t *testing.T) {
	ml := autoML2CList()
	mlReader := &stubWebhookMailingListReader{ml: ml}
	autoSyncer := &stubWebhookAutoSyncer{err: errors.New("itx unavailable")}
	evt := model.GrpsIOWebhookEvent{Action: constants.SubGroupMemberAddedEvent, MemberInfo: &model.MemberInfo{GroupID: 12345}}

	err := processGroupsIOWebhookEvent(context.Background(), evt, mlReader, autoSyncer, silentLog())

	require.Error(t, err)
}

func silentLog() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
