// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLeaseKV struct {
	jetstream.KeyValue
	mu              sync.Mutex
	entries         map[string]fakeLeaseEntry
	next            uint64
	updates         int
	updateErr       error
	createCommitErr error
	updateCommitErr error
	deleteRev       uint64
	deleteOpts      int
	deleteErr       error
	disappearOnGet  bool
	getFailures     int // -1 means every Get fails
	getCalls        int
}

type fakeLeaseEntry struct {
	jetstream.KeyValueEntry
	value []byte
	rev   uint64
}

func (e fakeLeaseEntry) Revision() uint64 { return e.rev }
func (e fakeLeaseEntry) Value() []byte    { return e.value }

func (kv *fakeLeaseKV) Create(_ context.Context, key string, value []byte, _ ...jetstream.KVCreateOpt) (uint64, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if _, ok := kv.entries[key]; ok {
		return 0, jetstream.ErrKeyExists
	}
	kv.next++
	kv.entries[key] = fakeLeaseEntry{value: value, rev: kv.next}
	if kv.createCommitErr != nil {
		return 0, kv.createCommitErr
	}
	return kv.next, nil
}

func (kv *fakeLeaseKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.getCalls++
	if kv.getFailures != 0 {
		if kv.getFailures > 0 {
			kv.getFailures--
		}
		return nil, errors.New("transient lock read failure")
	}
	entry, ok := kv.entries[key]
	if kv.disappearOnGet && ok {
		delete(kv.entries, key)
		kv.disappearOnGet = false
		return nil, jetstream.ErrKeyDeleted
	}
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return entry, nil
}

func (kv *fakeLeaseKV) Update(_ context.Context, key string, value []byte, rev uint64) (uint64, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if kv.entries[key].rev != rev {
		return 0, jetstream.ErrKeyRevisionMismatch
	}
	if kv.updateErr != nil {
		return 0, kv.updateErr
	}
	kv.next++
	kv.updates++
	kv.entries[key] = fakeLeaseEntry{value: value, rev: kv.next}
	if kv.updateCommitErr != nil {
		return 0, kv.updateCommitErr
	}
	return kv.next, nil
}

func (kv *fakeLeaseKV) Delete(_ context.Context, key string, opts ...jetstream.KVDeleteOpt) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.deleteOpts = len(opts)
	kv.deleteRev = kv.entries[key].rev
	if kv.deleteErr != nil {
		return kv.deleteErr
	}
	delete(kv.entries, key)
	return nil
}

// JetStream's KVDeleteOpt hides its revision behind an unexported method, so
// tests inject a revision-aware release rather than pretending to parse it.
func (kv *fakeLeaseKV) deleteRevision(_ context.Context, key string, revision uint64) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.deleteRev = revision
	entry, exists := kv.entries[key]
	if !exists || entry.rev != revision {
		return jetstream.ErrKeyRevisionMismatch
	}
	if kv.deleteErr != nil {
		return kv.deleteErr
	}
	delete(kv.entries, key)
	return nil
}

func newFakeServiceDomainLock(kv *fakeLeaseKV) *serviceDomainLock {
	lock := NewServiceDomainLock(kv).(*serviceDomainLock)
	lock.releaseRevision = kv.deleteRevision
	return lock
}

func TestServiceDomainLockSerializesSameService(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lockA := newFakeServiceDomainLock(kv)
	lockB := newFakeServiceDomainLock(kv)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		lockA.WithService(context.Background(), "svc-1", func(context.Context) bool {
			close(entered)
			<-release
			return false
		})
	}()
	<-entered
	called := false
	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	assert.True(t, lockB.WithService(waitCtx, "svc-1", func(context.Context) bool { called = true; return false }))
	assert.False(t, called)
	assert.False(t, lockB.WithService(context.Background(), "svc-2", func(context.Context) bool { called = true; return false }))
	assert.True(t, called)
	close(release)
	<-done
	assert.False(t, lockB.WithService(context.Background(), "svc-1", func(context.Context) bool { return false }))
}

func TestServiceDomainLockDoesNotStealExpiredLease(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	value, err := json.Marshal(lockValue{Owner: "orphaned", Expires: time.Now().Add(-time.Minute)})
	require.NoError(t, err)
	kv.entries["service.svc-1"] = fakeLeaseEntry{value: value, rev: 1}
	called := false
	assert.True(t, newFakeServiceDomainLock(kv).WithService(context.Background(), "svc-1", func(context.Context) bool { called = true; return false }))
	assert.False(t, called)
}

func TestServiceDomainLockFailedRenewalKeepsLastRevision(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 100 * time.Millisecond, renewInterval: time.Millisecond, releaseRevision: kv.deleteRevision}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := lock.WithService(ctx, "svc-1", func(workCtx context.Context) bool {
		kv.mu.Lock()
		kv.updateErr = errors.New("connection unavailable")
		kv.mu.Unlock()
		<-workCtx.Done()
		return false
	})
	assert.True(t, result)
	assert.Equal(t, uint64(1), kv.deleteRev, "failed Update must not replace the last owned revision with zero")
}

func TestServiceDomainLockCreateCommitThenTimeoutReleasesOwnedLock(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), createCommitErr: errors.New("create response timed out")}
	lock := newFakeServiceDomainLock(kv)
	called := false
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool {
		called = true
		return false
	}))
	assert.False(t, called, "an ambiguous acquisition must not run the callback")
	assert.Equal(t, uint64(1), kv.deleteRev)
	assert.NotContains(t, kv.entries, "service.svc-1", "the committed create must be cleaned up")
	kv.createCommitErr = nil
	assert.False(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool {
		called = true
		return false
	}))
	assert.True(t, called, "the next delivery must be able to acquire the lock")
}

func TestServiceDomainLockCreateCommitThenKeyExistsReleasesOwnToken(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), createCommitErr: jetstream.ErrKeyExists}
	called := false
	assert.True(t, newFakeServiceDomainLock(kv).WithService(context.Background(), "svc-1", func(context.Context) bool {
		called = true
		return false
	}))
	assert.False(t, called)
	assert.Equal(t, uint64(1), kv.deleteRev)
	assert.NotContains(t, kv.entries, "service.svc-1")
}

func TestServiceDomainLockAmbiguousCreateRetriesTransientReads(t *testing.T) {
	for _, createErr := range []error{errors.New("create response timed out"), jetstream.ErrKeyExists} {
		t.Run(createErr.Error(), func(t *testing.T) {
			kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), createCommitErr: createErr, getFailures: 2}
			called := false
			assert.True(t, newFakeServiceDomainLock(kv).WithService(context.Background(), "svc-1", func(context.Context) bool {
				called = true
				return false
			}))
			assert.False(t, called)
			assert.GreaterOrEqual(t, kv.getCalls, 3)
			assert.Equal(t, uint64(1), kv.deleteRev)
			assert.NotContains(t, kv.entries, "service.svc-1")
		})
	}
}

func TestServiceDomainLockRenewalCommitThenTimeoutReleasesCurrentRevision(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), updateCommitErr: errors.New("renewal response timed out")}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 100 * time.Millisecond, renewInterval: time.Millisecond, releaseRevision: kv.deleteRevision}
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(ctx context.Context) bool {
		<-ctx.Done()
		return false
	}))
	assert.Equal(t, uint64(2), kv.deleteRev, "release must use the committed renewal revision")
	assert.NotContains(t, kv.entries, "service.svc-1")
	kv.updateCommitErr = nil
	assert.False(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool { return false }))
}

func TestServiceDomainLockAmbiguousRenewalRetriesTransientReads(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), updateCommitErr: errors.New("renewal response timed out"), getFailures: 2}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 100 * time.Millisecond, renewInterval: time.Millisecond, releaseRevision: kv.deleteRevision}
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(ctx context.Context) bool {
		<-ctx.Done()
		return false
	}))
	assert.GreaterOrEqual(t, kv.getCalls, 3)
	assert.Equal(t, uint64(2), kv.deleteRev)
	assert.NotContains(t, kv.entries, "service.svc-1")
}

func TestServiceDomainLockReadRetryStopsAtDeadline(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), getFailures: -1}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := newFakeServiceDomainLock(kv).readLockWithRetry(ctx, "service.svc-1")
	require.ErrorContains(t, err, "transient lock read failure")
	assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	assert.Equal(t, uint64(0), kv.deleteRev, "an unverified lock must not be deleted")
}

func TestServiceDomainLockAmbiguousRenewalDoesNotDeleteAnotherOwner(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), updateCommitErr: errors.New("renewal response timed out")}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 100 * time.Millisecond, renewInterval: time.Millisecond, releaseRevision: kv.deleteRevision}
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(ctx context.Context) bool {
		<-ctx.Done()
		kv.mu.Lock()
		kv.next++
		kv.entries["service.svc-1"] = fakeLeaseEntry{value: []byte(`{"owner":"replacement"}`), rev: kv.next}
		kv.mu.Unlock()
		return false
	}))
	assert.Equal(t, uint64(0), kv.deleteRev, "never attempt to delete a replacement worker's lock")
	assert.Equal(t, []byte(`{"owner":"replacement"}`), kv.entries["service.svc-1"].value)
}

func TestServiceDomainLockReleaseFailureDoesNotAllowTakeover(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), deleteErr: errors.New("connection unavailable")}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 20 * time.Millisecond, renewInterval: 5 * time.Millisecond, releaseRevision: kv.deleteRevision}
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool { return false }))
	time.Sleep(25 * time.Millisecond)
	called := false
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool { called = true; return false }))
	assert.False(t, called, "an unreleased lease must not be stolen after expiry")
}

func TestServiceDomainLockReleaseRevisionMismatchPreservesReplacement(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lock := newFakeServiceDomainLock(kv)
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool {
		kv.mu.Lock()
		kv.next++
		kv.entries["service.svc-1"] = fakeLeaseEntry{value: []byte(`{"owner":"replacement"}`), rev: kv.next}
		kv.mu.Unlock()
		return false
	}), "lost ownership must NAK rather than ACK")
	kv.mu.Lock()
	entry, present := kv.entries["service.svc-1"]
	kv.mu.Unlock()
	assert.True(t, present)
	assert.Equal(t, []byte(`{"owner":"replacement"}`), entry.value)
	assert.Equal(t, uint64(1), kv.deleteRev, "release must use the original worker's revision")
}

func TestServiceDomainLockProductionReleaseUsesRevisionOption(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	assert.False(t, NewServiceDomainLock(kv).WithService(context.Background(), "svc-1", func(context.Context) bool {
		return false
	}))
	assert.Equal(t, 1, kv.deleteOpts, "JetStream release must specify LastRevision")
}

func TestServiceDomainLockWaitsForSameService(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lockA, lockB := newFakeServiceDomainLock(kv), newFakeServiceDomainLock(kv)
	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		lockA.WithService(context.Background(), "svc-1", func(context.Context) bool {
			close(entered)
			<-release
			return false
		})
	}()
	<-entered
	secondDone := make(chan bool, 1)
	go func() {
		secondDone <- lockB.WithService(context.Background(), "svc-1", func(context.Context) bool { return false })
	}()
	select {
	case <-secondDone:
		t.Fatal("second worker ran while the first still held the lock")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	<-firstDone
	select {
	case result := <-secondDone:
		assert.False(t, result)
	case <-time.After(time.Second):
		t.Fatal("second worker did not proceed after the first released the lock")
	}
}

func TestServiceDomainLockRetriesWhenOwnerReleasesBetweenCreateAndGet(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), disappearOnGet: true}
	kv.entries["service.svc-1"] = fakeLeaseEntry{value: []byte(`{"owner":"previous"}`), rev: 1}
	called := false
	assert.False(t, newFakeServiceDomainLock(kv).WithService(context.Background(), "svc-1", func(context.Context) bool {
		called = true
		return false
	}))
	assert.True(t, called)
}

func TestServiceDomainLockRenewsBeyondLeaseWindow(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 20 * time.Millisecond, renewInterval: 5 * time.Millisecond, releaseRevision: kv.deleteRevision}
	assert.False(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool {
		time.Sleep(65 * time.Millisecond)
		kv.mu.Lock()
		entry := kv.entries["service.svc-1"]
		kv.mu.Unlock()
		var value lockValue
		require.NoError(t, json.Unmarshal(entry.value, &value))
		assert.True(t, time.Now().Before(value.Expires), "lease must remain live during long fan-out")
		return false
	}))
	assert.Greater(t, kv.updates, 0)
}
