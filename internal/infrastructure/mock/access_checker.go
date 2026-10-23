// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// MockAccessChecker is an always-allow mock implementation of port.AccessChecker,
// used when ACCESS_CHECK_SOURCE=mock so local/dev environments that run without
// fga-sync (e.g. alongside AUTH_SOURCE=mock) don't need a live NATS reply subscriber.
type MockAccessChecker struct{}

// HasRelation always reports true.
func (m *MockAccessChecker) HasRelation(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

// NewMockAccessChecker creates a new mock access checker.
func NewMockAccessChecker() port.AccessChecker {
	return &MockAccessChecker{}
}
