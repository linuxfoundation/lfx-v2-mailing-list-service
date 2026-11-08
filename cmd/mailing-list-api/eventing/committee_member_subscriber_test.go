// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// stubAutoSyncer records TriggerAutoSync calls and returns a configured error, optionally keyed
// per mailing list ID.
type stubAutoSyncer struct {
	calls     []string
	err       error
	errByList map[string]error
}

func (s *stubAutoSyncer) TriggerAutoSync(_ context.Context, mailingListID string) error {
	s.calls = append(s.calls, mailingListID)
	if s.errByList != nil {
		if err, ok := s.errByList[mailingListID]; ok {
			return err
		}
	}
	return s.err
}

// stubCommitteeMailingListReader is a minimal port.GroupsIOMailingListReader for the committee
// subscriber tests: only ListMailingLists is exercised.
type stubCommitteeMailingListReader struct {
	lists []*model.GroupsIOMailingList
	err   error

	gotCtxHadDeadline bool
}

func (r *stubCommitteeMailingListReader) ListMailingLists(ctx context.Context, _, _ string) ([]*model.GroupsIOMailingList, int, error) {
	_, r.gotCtxHadDeadline = ctx.Deadline()
	return r.lists, len(r.lists), r.err
}
func (r *stubCommitteeMailingListReader) GetMailingList(_ context.Context, _ string) (*model.GroupsIOMailingList, error) {
	return nil, nil
}
func (r *stubCommitteeMailingListReader) GetMailingListCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (r *stubCommitteeMailingListReader) GetMailingListMemberCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func autoC2MList(uid string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		UID:        uid,
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeAutoCommitteeToMailingList}},
	}
}

func TestCommitteeUIDFromEvent(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{name: "created event", data: `{"data":{"member_uid":"m1","committee_uid":"c1","project_uid":"p1"}}`, want: "c1"},
		{name: "deleted event", data: `{"data":{"member_uid":"m1","committee_uid":"c2","email":"a@example.com"}}`, want: "c2"},
		{name: "updated event", data: `{"data":{"member_uid":"m1","old_member":{},"member":{"committee_uid":"c3"}}}`, want: "c3"},
		{name: "missing committee_uid", data: `{"data":{"member_uid":"m1"}}`, want: ""},
		{name: "malformed json", data: `not json`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := committeeUIDFromEvent([]byte(tt.data))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProcessCommitteeMemberEvent_TriggersAutoSyncForAutoC2MLinkedLists(t *testing.T) {
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{autoC2MList("ml-1")}}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.Equal(t, []string{"ml-1"}, syncer.calls)
}

func TestProcessCommitteeMemberEvent_SkipsListsNotConfiguredForAutoC2ML(t *testing.T) {
	manualList := &model.GroupsIOMailingList{
		UID:        "ml-manual",
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeManual}},
	}
	ml2cList := &model.GroupsIOMailingList{
		UID:        "ml-ml2c",
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeAutoMailingListToCommittee}},
	}
	noCommitteeList := &model.GroupsIOMailingList{UID: "ml-none"}
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{manualList, ml2cList, noCommitteeList}}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, syncer.calls, "only auto_c_to_ml lists should trigger auto-sync from a committee event")
}

func TestProcessCommitteeMemberEvent_MultipleLinkedLists_AllTriggered(t *testing.T) {
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{
		autoC2MList("ml-1"),
		autoC2MList("ml-2"),
	}}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ml-1", "ml-2"}, syncer.calls)
}

func TestProcessCommitteeMemberEvent_ListMailingListsError_Propagates(t *testing.T) {
	reader := &stubCommitteeMailingListReader{err: errors.New("itx unavailable")}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.Error(t, err)
	assert.Empty(t, syncer.calls)
}

func TestProcessCommitteeMemberEvent_OneListFails_OthersStillAttempted(t *testing.T) {
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{
		autoC2MList("ml-1"),
		autoC2MList("ml-2"),
	}}
	syncer := &stubAutoSyncer{errByList: map[string]error{"ml-1": errors.New("itx job failed")}}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.Error(t, err, "a failure should be reported so the caller can log it")
	assert.ElementsMatch(t, []string{"ml-1", "ml-2"}, syncer.calls, "a failure on one list must not block attempting the others")
}

func TestProcessCommitteeMemberEvent_ListMailingListsLookup_HasBoundedDeadline(t *testing.T) {
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{autoC2MList("ml-1")}}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.True(t, reader.gotCtxHadDeadline, "the ListMailingLists lookup must run under its own bounded timeout, not the subscriber's unbounded background context")
}

func TestProcessCommitteeMemberEvent_NoLinkedLists_NoOp(t *testing.T) {
	reader := &stubCommitteeMailingListReader{}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, syncer.calls)
}

// blockingAutoSyncer's TriggerAutoSync blocks on release until it is closed, letting a test hold
// one call "in flight" while issuing a second concurrent call for the same mailing list.
type blockingAutoSyncer struct {
	mu      sync.Mutex
	calls   int
	release chan struct{}
}

func (s *blockingAutoSyncer) TriggerAutoSync(ctx context.Context, _ string) error {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	select {
	case <-s.release:
	case <-ctx.Done():
	}
	return nil
}

func (s *blockingAutoSyncer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// TestAutoSyncCoalescer_ConcurrentCallsForSameList_Coalesced pins the fix for the Major finding
// (per @andrest50): two concurrent TriggerAutoSync calls for the same mailing list must not both
// reach the wrapped autoSyncTrigger at once — the second must be coalesced into a dirty re-run of
// the first instead of racing it (which would otherwise hit ITX's "a sync job is already in
// progress for this mailing list" Validation error and be silently dropped by handle's
// best-effort/no-retry logging).
func TestAutoSyncCoalescer_ConcurrentCallsForSameList_Coalesced(t *testing.T) {
	syncer := &blockingAutoSyncer{release: make(chan struct{})}
	c := newAutoSyncCoalescer(syncer)

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- c.TriggerAutoSync(context.Background(), "ml-1")
	}()

	require.Eventually(t, func() bool { return syncer.callCount() == 1 }, time.Second, time.Millisecond,
		"first call should reach the wrapped syncer")

	// Second call arrives while the first is still running: it must be coalesced (return
	// immediately, nil error) rather than block or race the in-flight call.
	err := c.TriggerAutoSync(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Equal(t, 1, syncer.callCount(), "a coalesced call must not reach the wrapped syncer immediately")

	close(syncer.release)
	require.NoError(t, <-firstDone)

	// The in-flight call, finding itself marked dirty, must re-run once more on the coalesced
	// call's behalf.
	require.Eventually(t, func() bool { return syncer.callCount() == 2 }, time.Second, time.Millisecond,
		"the dirty flag must trigger exactly one re-run after the in-flight call finishes")
}

// TestAutoSyncCoalescer_DifferentLists_NotCoalesced pins that coalescing is scoped per mailing
// list: a concurrent call for a different list must run independently, not be swallowed by another
// list's in-flight run.
func TestAutoSyncCoalescer_DifferentLists_NotCoalesced(t *testing.T) {
	syncer := &stubAutoSyncer{}
	c := newAutoSyncCoalescer(syncer)

	require.NoError(t, c.TriggerAutoSync(context.Background(), "ml-1"))
	require.NoError(t, c.TriggerAutoSync(context.Background(), "ml-2"))

	assert.ElementsMatch(t, []string{"ml-1", "ml-2"}, syncer.calls)
}

// TestAutoSyncCoalescer_SequentialCalls_BothRun pins that coalescing only short-circuits a call
// that arrives while another is running for the same list — two calls that do not overlap must
// both reach the wrapped syncer.
func TestAutoSyncCoalescer_SequentialCalls_BothRun(t *testing.T) {
	syncer := &stubAutoSyncer{}
	c := newAutoSyncCoalescer(syncer)

	require.NoError(t, c.TriggerAutoSync(context.Background(), "ml-1"))
	require.NoError(t, c.TriggerAutoSync(context.Background(), "ml-1"))

	assert.Equal(t, []string{"ml-1", "ml-1"}, syncer.calls)
}

// deadlineRecordingAutoSyncer records the remaining time-until-deadline on the ctx it is called
// with for every call, so a test can assert each attempt got a fresh budget.
type deadlineRecordingAutoSyncer struct {
	mu         sync.Mutex
	remainings []time.Duration
	block      chan struct{}
}

func (s *deadlineRecordingAutoSyncer) TriggerAutoSync(ctx context.Context, _ string) error {
	deadline, ok := ctx.Deadline()
	remaining := time.Duration(0)
	if ok {
		remaining = time.Until(deadline)
	}

	s.mu.Lock()
	s.remainings = append(s.remainings, remaining)
	n := len(s.remainings)
	s.mu.Unlock()

	if s.block != nil && n == 1 {
		<-s.block
	}
	return nil
}

// TestAutoSyncCoalescer_DirtyRerun_GetsFreshBudget pins that a dirty-triggered re-run derives its
// own fresh committeeMemberEventCallTimeout deadline from the caller's ctx, rather than continuing
// to run under whatever remains of the first attempt's deadline. Without this, a re-run triggered
// close to the first attempt's deadline could itself be starved mid-poll, silently dropping the
// very update the coalescer exists to rescue (see autoSyncCoalescer's doc comment).
func TestAutoSyncCoalescer_DirtyRerun_GetsFreshBudget(t *testing.T) {
	syncer := &deadlineRecordingAutoSyncer{block: make(chan struct{})}
	c := newAutoSyncCoalescer(syncer)

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- c.TriggerAutoSync(context.Background(), "ml-1")
	}()

	require.Eventually(t, func() bool {
		syncer.mu.Lock()
		defer syncer.mu.Unlock()
		return len(syncer.remainings) == 1
	}, time.Second, time.Millisecond, "first attempt should reach the wrapped syncer")

	// Simulate some of the first attempt's budget elapsing before the dirty call arrives.
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, c.TriggerAutoSync(context.Background(), "ml-1"))

	close(syncer.block)
	require.NoError(t, <-firstDone)

	require.Eventually(t, func() bool {
		syncer.mu.Lock()
		defer syncer.mu.Unlock()
		return len(syncer.remainings) == 2
	}, time.Second, time.Millisecond, "the dirty flag must trigger exactly one re-run")

	syncer.mu.Lock()
	defer syncer.mu.Unlock()
	require.Len(t, syncer.remainings, 2)
	// The re-run's remaining budget must not be meaningfully smaller than the first attempt's — it
	// must be a fresh committeeMemberEventCallTimeout window, not the first attempt's deadline minus
	// however long the first attempt had already been running.
	assert.InDelta(t, syncer.remainings[0].Seconds(), syncer.remainings[1].Seconds(), 2,
		"re-run should get a fresh budget, not the first attempt's already-elapsed deadline")
}
