// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	serviceLockTTL       = 2 * time.Minute
	serviceLockRenewal   = serviceLockTTL / 4
	serviceLockReadRetry = 250 * time.Millisecond
)

type serviceDomainLock struct {
	kv            jetstream.KeyValue
	leaseDuration time.Duration
	renewInterval time.Duration
	// releaseRevision is overridden by tests to exercise revision-aware deletion.
	// Production uses the JetStream LastRevision option in release.
	releaseRevision func(context.Context, string, uint64) error
}

type lockValue struct {
	Owner   string    `json:"owner"`
	Expires time.Time `json:"expires"`
}

// NewServiceDomainLock uses a short-lived KV lease to coordinate replicas.
func NewServiceDomainLock(kv jetstream.KeyValue) port.ServiceDomainLock {
	return &serviceDomainLock{kv: kv, leaseDuration: serviceLockTTL, renewInterval: serviceLockRenewal}
}

func (l *serviceDomainLock) WithService(ctx context.Context, uid string, fn func(context.Context) bool) bool {
	if !constants.ValidKVKeySegment(uid) {
		slog.ErrorContext(ctx, "invalid service UID for domain lock", "uid", uid)
		return false
	}
	key := "service." + uid
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		slog.ErrorContext(ctx, "failed to generate service lock token", "uid", uid, "error", err)
		return true
	}
	owner := hex.EncodeToString(token[:])
	var value []byte
	var err error
	var revision uint64
	for {
		if err := ctx.Err(); err != nil {
			return true
		}
		value, err = json.Marshal(lockValue{Owner: owner, Expires: time.Now().Add(l.leaseDuration)})
		if err != nil {
			return true
		}
		revision, err = l.kv.Create(ctx, key, value)
		if errors.Is(err, jetstream.ErrKeyExists) {
			// A timed-out Create may have committed and been retried internally,
			// returning KeyExists for our own token. Read with a bounded cleanup
			// context even if the acquisition context has since been cancelled.
			readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
			entry, readErr := l.readLockWithRetry(readCtx, key)
			readCancel()
			if errors.Is(readErr, jetstream.ErrKeyNotFound) || errors.Is(readErr, jetstream.ErrKeyDeleted) {
				continue // owner released between Create and Get
			}
			if readErr != nil {
				return true
			}
			// Never steal a lease, even after its timestamp expires. KV revision
			// CAS cannot fence a NATS publish already in progress by the prior
			// owner. An orphaned lock requires explicit operational repair.
			var existing lockValue
			if json.Unmarshal(entry.Value(), &existing) != nil {
				slog.ErrorContext(ctx, "service domain lock expired or corrupt; manual repair required", "uid", uid, "mapping_key", key)
				return true
			}
			if existing.Owner == owner {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cleanupCancel()
				if cleanupErr := l.releaseIfOwner(cleanupCtx, key, owner); cleanupErr != nil {
					slog.WarnContext(ctx, "could not reconcile ambiguous service domain lock acquisition", "uid", uid, "error", cleanupErr)
				}
				return true
			}
			if !time.Now().Before(existing.Expires) {
				slog.ErrorContext(ctx, "service domain lock expired or corrupt; manual repair required", "uid", uid, "mapping_key", key)
				return true
			}
			select {
			case <-ctx.Done():
				return true
			case <-time.After(serviceLockReadRetry):
				continue
			}
		}
		break
	}
	if err != nil {
		slog.WarnContext(ctx, "failed to acquire service domain lock", "uid", uid, "error", err)
		// Create may have committed before its response timed out. No work has
		// started, so release only if the stored token still belongs to us.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if cleanupErr := l.releaseIfOwner(cleanupCtx, key, owner); cleanupErr != nil {
			slog.WarnContext(ctx, "could not reconcile ambiguous service domain lock acquisition", "uid", uid, "error", cleanupErr)
		}
		return true
	}
	// Renew the lease with revision CAS while work runs. An expired lock is
	// never stolen automatically, because KV cannot fence a publish already
	// started by its former owner; an orphaned key needs operational repair.
	workCtx, cancel := context.WithCancel(ctx)
	var mu sync.Mutex
	var leaseErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(l.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				renewed, marshalErr := json.Marshal(lockValue{Owner: owner, Expires: time.Now().Add(l.leaseDuration)})
				if marshalErr == nil {
					var nextRevision uint64
					nextRevision, marshalErr = l.kv.Update(workCtx, key, renewed, revision)
					if marshalErr == nil {
						revision = nextRevision
					}
				}
				if marshalErr != nil {
					leaseErr = marshalErr
					cancel()
				}
				mu.Unlock()
				if marshalErr != nil {
					return
				}
			}
		}
	}()
	result := fn(workCtx)
	// Stop renewal and wait for its last CAS to finish before releasing the
	// final revision. A failed CAS must never release another owner's lock.
	mu.Lock()
	cancel()
	mu.Unlock()
	<-done
	mu.Lock()
	finalRevision, failed := revision, leaseErr
	mu.Unlock()
	var releaseErr error
	func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if failed != nil {
			// Update may have committed even though it returned an error. The
			// callback and renewal are stopped; re-read the revision, but never
			// release a lock with another worker's token.
			releaseErr = l.releaseIfOwner(cleanupCtx, key, owner)
		} else {
			releaseErr = l.release(cleanupCtx, key, finalRevision)
		}
	}()
	if releaseErr != nil {
		slog.WarnContext(ctx, "failed to release service domain lock, will retry", "uid", uid, "error", releaseErr)
		return true
	}
	if failed != nil || ctx.Err() != nil {
		slog.WarnContext(ctx, "service domain lock lost or processing cancelled", "uid", uid, "error", failed)
		return true
	}
	return result
}

func (l *serviceDomainLock) release(ctx context.Context, key string, revision uint64) error {
	if l.releaseRevision != nil {
		return l.releaseRevision(ctx, key, revision)
	}
	return l.kv.Delete(ctx, key, jetstream.LastRevision(revision))
}

func (l *serviceDomainLock) releaseIfOwner(ctx context.Context, key, owner string) error {
	entry, err := l.readLockWithRetry(ctx, key)
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return nil // the ambiguous write did not leave a live lock
	}
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("service domain lock %s has no readable entry", key)
	}
	var current lockValue
	if err := json.Unmarshal(entry.Value(), &current); err != nil {
		return fmt.Errorf("decode service domain lock %s: %w", key, err)
	}
	if current.Owner != owner {
		return fmt.Errorf("service domain lock %s belongs to another owner", key)
	}
	return l.release(ctx, key, entry.Revision())
}

// readLockWithRetry gives transient KV read failures a bounded chance to
// recover while retaining the caller's owner-token and revision checks.
func (l *serviceDomainLock) readLockWithRetry(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	for {
		entry, err := l.kv.Get(ctx, key)
		if err == nil || errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) || errors.Is(err, jetstream.ErrInvalidKey) {
			return entry, err
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("read service domain lock %s: %w (retry deadline: %v)", key, err, ctx.Err())
		case <-time.After(serviceLockReadRetry):
		}
	}
}

func (l *serviceDomainLock) WithServices(ctx context.Context, uids []string, fn func(context.Context) bool) bool {
	unique := make(map[string]struct{}, len(uids))
	for _, uid := range uids {
		if !constants.ValidKVKeySegment(uid) {
			slog.ErrorContext(ctx, "invalid service UID for domain lock", "uid", uid)
			return false
		}
		unique[uid] = struct{}{}
	}
	ordered := make([]string, 0, len(unique))
	for uid := range unique {
		ordered = append(ordered, uid)
	}
	slices.Sort(ordered)
	var acquire func(int, context.Context) bool
	acquire = func(i int, ctx context.Context) bool {
		if i == len(ordered) {
			return fn(ctx)
		}
		return l.WithService(ctx, ordered[i], func(lockedCtx context.Context) bool {
			return acquire(i+1, lockedCtx)
		})
	}
	return acquire(0, ctx)
}
