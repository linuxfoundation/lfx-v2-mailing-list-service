// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// ServiceDomainLock serializes domain-affecting publications for one service
// across all service replicas. The callback must stop promptly when ctx expires.
type ServiceDomainLock interface {
	WithService(ctx context.Context, uid string, fn func(context.Context) bool) bool
	// WithServices acquires all service locks in stable order before invoking fn.
	WithServices(ctx context.Context, uids []string, fn func(context.Context) bool) bool
}
