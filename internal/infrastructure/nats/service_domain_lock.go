// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	serviceLockTTL     = 2 * time.Minute
	serviceLockRenewal = serviceLockTTL / 4
)

type serviceDomainLock struct {
	kv            jetstream.KeyValue
	leaseDuration time.Duration
	renewInterval time.Duration
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
			entry, readErr := l.kv.Get(ctx, key)
			if readErr != nil {
				return true
			}
			// Never steal a lease, even after its timestamp expires. KV revision
			// CAS cannot fence a NATS publish already in progress by the prior
			// owner. An orphaned lock requires explicit operational repair.
			var existing lockValue
			if json.Unmarshal(entry.Value(), &existing) != nil || !time.Now().Before(existing.Expires) {
				slog.ErrorContext(ctx, "service domain lock expired or corrupt; manual repair required", "uid", uid, "mapping_key", key)
				return true
			}
			select {
			case <-ctx.Done():
				return true
			case <-time.After(250 * time.Millisecond):
				continue
			}
		}
		break
	}
	if err != nil {
		slog.WarnContext(ctx, "failed to acquire service domain lock", "uid", uid, "error", err)
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
	func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := l.kv.Delete(cleanupCtx, key, jetstream.LastRevision(finalRevision)); err != nil &&
			!errors.Is(err, jetstream.ErrKeyRevisionMismatch) {
			slog.WarnContext(ctx, "failed to release service domain lock", "uid", uid, "error", err)
		}
	}()
	if failed != nil || ctx.Err() != nil {
		slog.WarnContext(ctx, "service domain lock lost or processing cancelled", "uid", uid, "error", failed)
		return true
	}
	return result
}

func (l *serviceDomainLock) WithServices(ctx context.Context, uids []string, fn func(context.Context) bool) bool {
	unique := make(map[string]struct{}, len(uids))
	for _, uid := range uids {
		if !constants.ValidKVKeySegment(uid) {
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
