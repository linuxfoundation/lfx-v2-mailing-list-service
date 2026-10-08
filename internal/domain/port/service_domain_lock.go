// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// ServiceDomainLock serializes domain-affecting publications across replicas.
// Subgroup handlers acquire their subgroup lock before acquiring parent service
// locks through WithServices; service handlers acquire only their service lock.
// Locks are not reentrant: never request a UID already held by the caller.
// Callbacks must stop promptly when their context expires.
type ServiceDomainLock interface {
	// WithService holds the lock for uid while fn runs. It returns fn's result
	// (true means NAK/retry, false means ACK) unless acquisition, lease renewal,
	// release, or cancellation fails, in which case it returns true. An invalid UID is
	// logged and returns false without invoking fn.
	WithService(ctx context.Context, uid string, fn func(context.Context) bool) bool
	// WithServices deduplicates and sorts uids before acquiring their locks and
	// invoking fn; callers do not need to sort them. When called from a subgroup
	// handler, the subgroup lock must already be held. It follows WithService's
	// return contract, including ACK without invoking fn for an invalid UID.
	// With no UIDs, it invokes fn directly without acquiring a lock.
	WithServices(ctx context.Context, uids []string, fn func(context.Context) bool) bool
}
