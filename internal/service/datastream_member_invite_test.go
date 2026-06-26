// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// ---------------------------------------------------------------------------
// Stub helpers
// ---------------------------------------------------------------------------

type stubInviteSender struct {
	result  *model.InviteResult
	err     error
	called  bool
	lastReq inviteapi.SendInviteRequest
}

func (s *stubInviteSender) SendInvite(_ context.Context, req inviteapi.SendInviteRequest) (*model.InviteResult, error) {
	s.called = true
	s.lastReq = req
	return s.result, s.err
}

type stubUserReader struct {
	username string
	err      error
}

func (r *stubUserReader) UsernameByEmail(_ context.Context, _ string) (string, error) {
	return r.username, r.err
}

// stubKVEntry satisfies jetstream.KeyValueEntry; only Value() is meaningful.
type stubKVEntry struct{ data []byte }

func (e *stubKVEntry) Bucket() string                  { return "test" }
func (e *stubKVEntry) Key() string                     { return "" }
func (e *stubKVEntry) Value() []byte                   { return e.data }
func (e *stubKVEntry) Revision() uint64                { return 0 }
func (e *stubKVEntry) Created() time.Time              { return time.Time{} }
func (e *stubKVEntry) Delta() uint64                   { return 0 }
func (e *stubKVEntry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }

// stubKV satisfies jetstream.KeyValue; only Get is wired.
type stubKV struct {
	entries map[string][]byte
	getErr  error
}

func newStubKV() *stubKV { return &stubKV{entries: make(map[string][]byte)} }

func (kv *stubKV) setJSON(key string, v any) {
	b, _ := json.Marshal(v)
	kv.entries[key] = b
}

func (kv *stubKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	if kv.getErr != nil {
		return nil, kv.getErr
	}
	b, ok := kv.entries[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return &stubKVEntry{data: b}, nil
}

func (kv *stubKV) GetRevision(_ context.Context, _ string, _ uint64) (jetstream.KeyValueEntry, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) Put(_ context.Context, _ string, _ []byte) (uint64, error) {
	return 0, errors.New("not implemented")
}
func (kv *stubKV) PutString(_ context.Context, _ string, _ string) (uint64, error) {
	return 0, errors.New("not implemented")
}
func (kv *stubKV) Create(_ context.Context, _ string, _ []byte, _ ...jetstream.KVCreateOpt) (uint64, error) {
	return 0, errors.New("not implemented")
}
func (kv *stubKV) Update(_ context.Context, _ string, _ []byte, _ uint64) (uint64, error) {
	return 0, errors.New("not implemented")
}
func (kv *stubKV) Delete(_ context.Context, _ string, _ ...jetstream.KVDeleteOpt) error {
	return errors.New("not implemented")
}
func (kv *stubKV) Purge(_ context.Context, _ string, _ ...jetstream.KVDeleteOpt) error {
	return errors.New("not implemented")
}
func (kv *stubKV) Watch(_ context.Context, _ string, _ ...jetstream.WatchOpt) (jetstream.KeyWatcher, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) WatchAll(_ context.Context, _ ...jetstream.WatchOpt) (jetstream.KeyWatcher, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) WatchFiltered(_ context.Context, _ []string, _ ...jetstream.WatchOpt) (jetstream.KeyWatcher, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) Keys(_ context.Context, _ ...jetstream.WatchOpt) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) ListKeys(_ context.Context, _ ...jetstream.WatchOpt) (jetstream.KeyLister, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) ListKeysFiltered(_ context.Context, _ ...string) (jetstream.KeyLister, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) History(_ context.Context, _ string, _ ...jetstream.WatchOpt) ([]jetstream.KeyValueEntry, error) {
	return nil, errors.New("not implemented")
}
func (kv *stubKV) Bucket() string                                            { return "test" }
func (kv *stubKV) PurgeDeletes(_ context.Context, _ ...jetstream.KVPurgeOpt) error {
	return errors.New("not implemented")
}
func (kv *stubKV) Status(_ context.Context) (jetstream.KeyValueStatus, error) {
	return nil, errors.New("not implemented")
}

var _ jetstream.KeyValue = (*stubKV)(nil)

// ---------------------------------------------------------------------------
// newTestHandler builds a handler wired with the given stubs.
// ---------------------------------------------------------------------------

func newTestHandler(
	sender port.InviteSender,
	reader port.UserReader,
	m port.MappingReaderWriter,
	kv jetstream.KeyValue,
) *MemberInviteHandler {
	return NewMemberInviteHandler(sender, reader, m, kv, "https://app.lfx.dev")
}

func silentLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// ---------------------------------------------------------------------------
// ShouldSendMemberInvite
// ---------------------------------------------------------------------------

func TestShouldSendMemberInvite(t *testing.T) {
	cases := []struct {
		name     string
		action   model.MessageAction
		username string
		email    string
		want     bool
	}{
		{"created no-LFID with email", model.ActionCreated, "", "user@example.com", true},
		{"created with LFID", model.ActionCreated, "jsmith", "user@example.com", false},
		{"created no email", model.ActionCreated, "", "", false},
		{"updated no-LFID with email", model.ActionUpdated, "", "user@example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldSendMemberInvite(tc.action, tc.username, tc.email)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// MaybeSendInvite
// ---------------------------------------------------------------------------

func TestMaybeSendInvite_NilHandler_DoesNotPanic(t *testing.T) {
	var h *MemberInviteHandler
	assert.NotPanics(t, func() {
		h.MaybeSendInvite(context.Background(), silentLogger(), &model.GrpsIOMember{Email: "x@example.com"})
	})
}

func TestMaybeSendInvite_NoEmail_Skips(t *testing.T) {
	sender := &stubInviteSender{result: &model.InviteResult{InviteUID: "inv-1"}}
	h := newTestHandler(sender, &stubUserReader{err: port.ErrUserNotFound}, mock.NewFakeMappingStore(), newStubKV())
	require.NotNil(t, h)

	h.MaybeSendInvite(context.Background(), silentLogger(), &model.GrpsIOMember{
		UID:   "mem-1",
		Email: "   ", // blank → skip
	})
	assert.False(t, sender.called)
}

func TestMaybeSendInvite_DedupMarkerPresent_Skips(t *testing.T) {
	sender := &stubInviteSender{result: &model.InviteResult{InviteUID: "inv-1"}}
	m := mock.NewFakeMappingStore()
	// Pre-set the dedup marker for this member.
	m.Set(memberInviteSentKey("mem-1"), "pending")

	h := newTestHandler(sender, &stubUserReader{err: port.ErrUserNotFound}, m, newStubKV())
	require.NotNil(t, h)

	h.MaybeSendInvite(context.Background(), silentLogger(), &model.GrpsIOMember{
		UID:   "mem-1",
		Email: "alice@example.com",
	})
	assert.False(t, sender.called, "should skip when dedup marker present")
}

func TestMaybeSendInvite_AlreadyHasLFID_Skips(t *testing.T) {
	sender := &stubInviteSender{result: &model.InviteResult{InviteUID: "inv-1"}}
	// userReader reports the email already has a username.
	reader := &stubUserReader{username: "jsmith"}

	h := newTestHandler(sender, reader, mock.NewFakeMappingStore(), newStubKV())
	require.NotNil(t, h)

	h.MaybeSendInvite(context.Background(), silentLogger(), &model.GrpsIOMember{
		UID:   "mem-2",
		Email: "jsmith@example.com",
	})
	assert.False(t, sender.called, "should skip when member already has LFID")
}

func TestMaybeSendInvite_NameUnresolvable_Skips(t *testing.T) {
	sender := &stubInviteSender{result: &model.InviteResult{InviteUID: "inv-1"}}
	kv := newStubKV()
	kv.getErr = errors.New("bucket unavailable")

	h := newTestHandler(sender, &stubUserReader{err: port.ErrUserNotFound}, mock.NewFakeMappingStore(), kv)
	require.NotNil(t, h)

	h.MaybeSendInvite(context.Background(), silentLogger(), &model.GrpsIOMember{
		UID:            "mem-3",
		Email:          "bob@example.com",
		MailingListUID: "sg-42",
	})
	assert.False(t, sender.called, "should skip when mailing-list name cannot be resolved")
}

func TestMaybeSendInvite_HappyPath_SendsAndStoresMarker(t *testing.T) {
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	sender := &stubInviteSender{result: &model.InviteResult{InviteUID: "inv-abc", ExpiresAt: expiresAt}}
	m := mock.NewFakeMappingStore()
	kv := newStubKV()
	kv.setJSON(kvPrefixSubgroupV1+"sg-42", map[string]any{"group_name": "Dev Mailing List"})

	h := newTestHandler(sender, &stubUserReader{err: port.ErrUserNotFound}, m, kv)
	require.NotNil(t, h)

	member := &model.GrpsIOMember{
		UID:            "mem-7",
		Email:          "dev@example.com",
		MailingListUID: "sg-42",
		FirstName:      "Dev",
		LastName:       "User",
	}
	h.MaybeSendInvite(context.Background(), silentLogger(), member)

	assert.True(t, sender.called, "SendInvite should have been called")
	assert.Equal(t, "dev@example.com", sender.lastReq.Recipient.Email)
	assert.Equal(t, constants.ResourceTypeMailingList, sender.lastReq.Resource.Type)
	assert.Equal(t, "sg-42", sender.lastReq.Resource.UID)
	assert.Equal(t, "Dev Mailing List", sender.lastReq.Resource.Name)
	assert.Equal(t, constants.InviteRoleMember, sender.lastReq.Role)
	assert.Contains(t, sender.lastReq.ReturnURL, "mailing-lists/sg-42")

	// The dedup marker should now hold the invite UID.
	markerVal, ok := m.GetMappingValue(context.Background(), memberInviteSentKey("mem-7"))
	require.True(t, ok, "dedup marker should be written after send")
	assert.Equal(t, "inv-abc", markerVal)
}

func TestMaybeSendInvite_KVFallbackToTitle(t *testing.T) {
	sender := &stubInviteSender{result: &model.InviteResult{InviteUID: "inv-2"}}
	kv := newStubKV()
	// group_name absent, title present.
	kv.setJSON(kvPrefixSubgroupV1+"sg-99", map[string]any{"title": "Dev Title List"})

	h := newTestHandler(sender, &stubUserReader{err: port.ErrUserNotFound}, mock.NewFakeMappingStore(), kv)
	require.NotNil(t, h)

	member := &model.GrpsIOMember{
		UID:            "mem-8",
		Email:          "alice@example.com",
		MailingListUID: "sg-99",
	}
	h.MaybeSendInvite(context.Background(), silentLogger(), member)

	assert.True(t, sender.called)
	assert.Equal(t, "Dev Title List", sender.lastReq.Resource.Name)
}

func TestMaybeSendInvite_UserReaderTransientError_ProceedsWithInvite(t *testing.T) {
	// Transient auth-service errors should not block the invite: the comment in the
	// production code explains that skipping would permanently lose the invite
	// opportunity (the message won't be redelivered as ActionCreated).
	sender := &stubInviteSender{result: &model.InviteResult{InviteUID: "inv-3"}}
	reader := &stubUserReader{err: errors.New("auth service timeout")}
	kv := newStubKV()
	kv.setJSON(kvPrefixSubgroupV1+"sg-1", map[string]any{"group_name": "Test List"})

	h := newTestHandler(sender, reader, mock.NewFakeMappingStore(), kv)
	require.NotNil(t, h)

	h.MaybeSendInvite(context.Background(), silentLogger(), &model.GrpsIOMember{
		UID:            "mem-9",
		Email:          "user@example.com",
		MailingListUID: "sg-1",
	})
	assert.True(t, sender.called, "transient user-reader error should not block invite")
}
