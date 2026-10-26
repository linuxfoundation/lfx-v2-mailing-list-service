// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/stretchr/testify/require"
)

// mustLookup reads key through the port and fails the test on a read error.
func mustLookup(t *testing.T, m port.MappingReader, key string) port.MappingEntry {
	t.Helper()
	entry, err := m.Lookup(context.Background(), key)
	require.NoError(t, err)
	return entry
}
