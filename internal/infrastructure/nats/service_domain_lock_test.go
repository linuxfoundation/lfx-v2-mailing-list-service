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
	mu        sync.Mutex
	entries   map[string]fakeLeaseEntry
	next      uint64
	updates   int
	updateErr error
	deleteRev uint64
	deleteErr error
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
	return kv.next, nil
}

func (kv *fakeLeaseKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	entry, ok := kv.entries[key]
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
	return kv.next, nil
}

func (kv *fakeLeaseKV) Delete(_ context.Context, key string, _ ...jetstream.KVDeleteOpt) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.deleteRev = kv.entries[key].rev
	if kv.deleteErr != nil {
		return kv.deleteErr
	}
	delete(kv.entries, key)
	return nil
}

func TestServiceDomainLockSerializesSameService(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lockA := NewServiceDomainLock(kv)
	lockB := NewServiceDomainLock(kv)
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
	assert.True(t, NewServiceDomainLock(kv).WithService(context.Background(), "svc-1", func(context.Context) bool { called = true; return false }))
	assert.False(t, called)
}

func TestServiceDomainLockFailedRenewalKeepsLastRevision(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 100 * time.Millisecond, renewInterval: time.Millisecond}
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

func TestServiceDomainLockReleaseFailureDoesNotAllowTakeover(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry), deleteErr: errors.New("connection unavailable")}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 20 * time.Millisecond, renewInterval: 5 * time.Millisecond}
	assert.False(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool { return false }))
	time.Sleep(25 * time.Millisecond)
	called := false
	assert.True(t, lock.WithService(context.Background(), "svc-1", func(context.Context) bool { called = true; return false }))
	assert.False(t, called, "an unreleased lease must not be stolen after expiry")
}

func TestServiceDomainLockWaitsForSameService(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lockA, lockB := NewServiceDomainLock(kv), NewServiceDomainLock(kv)
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

func TestServiceDomainLockRenewsBeyondLeaseWindow(t *testing.T) {
	kv := &fakeLeaseKV{entries: make(map[string]fakeLeaseEntry)}
	lock := &serviceDomainLock{kv: kv, leaseDuration: 20 * time.Millisecond, renewInterval: 5 * time.Millisecond}
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
