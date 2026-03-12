// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpMapper_PassThrough(t *testing.T) {
	m := NewNoOpMapper()
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func(string) (string, error)
		id   string
	}{
		{"MapProjectV2ToV1", func(id string) (string, error) { return m.MapProjectV2ToV1(ctx, id) }, "proj-uuid-123"},
		{"MapProjectV1ToV2", func(id string) (string, error) { return m.MapProjectV1ToV2(ctx, id) }, "a0000000000001"},
		{"MapCommitteeV2ToV1", func(id string) (string, error) { return m.MapCommitteeV2ToV1(ctx, id) }, "comm-uuid-456"},
		{"MapCommitteeV1ToV2", func(id string) (string, error) { return m.MapCommitteeV1ToV2(ctx, id) }, "b0000000000002"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.fn(tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.id, result, "NoOpMapper should return input unchanged")
		})
	}
}

func TestNoOpMapper_EmptyInput(t *testing.T) {
	m := NewNoOpMapper()
	ctx := context.Background()

	result, err := m.MapProjectV2ToV1(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestNoOpMapper_ImplementsInterface(t *testing.T) {
	// Verifies the NoOpMapper satisfies the domain.IDMapper interface at compile time
	// via the type assertion in providers.go — this test documents the contract
	m := NewNoOpMapper()
	assert.NotNil(t, m)
}
