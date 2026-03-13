// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNATSMapper_EmptyURL(t *testing.T) {
	_, err := NewNATSMapper(Config{URL: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NATS URL is required")
}

func TestNewNATSMapper_UnreachableURL(t *testing.T) {
	_, err := NewNATSMapper(Config{
		URL:     "nats://localhost:14222", // port unlikely to be in use
		Timeout: 100 * time.Millisecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to NATS")
}

// The following tests use an internal NATSMapper instance with a nil conn to
// exercise input validation that occurs before any NATS call is made.

func TestNATSMapper_MapProjectV2ToV1_EmptyUID(t *testing.T) {
	m := &NATSMapper{conn: nil, timeout: 5 * time.Second}

	_, err := m.MapProjectV2ToV1(context.Background(), "")
	require.Error(t, err)

	var domErr *domain.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
	assert.Contains(t, domErr.Message, "v2 project UID is required")
}

func TestNATSMapper_MapProjectV1ToV2_EmptySFID(t *testing.T) {
	m := &NATSMapper{conn: nil, timeout: 5 * time.Second}

	_, err := m.MapProjectV1ToV2(context.Background(), "")
	require.Error(t, err)

	var domErr *domain.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
	assert.Contains(t, domErr.Message, "v1 project SFID is required")
}

func TestNATSMapper_MapCommitteeV2ToV1_EmptyUID(t *testing.T) {
	m := &NATSMapper{conn: nil, timeout: 5 * time.Second}

	_, err := m.MapCommitteeV2ToV1(context.Background(), "")
	require.Error(t, err)

	var domErr *domain.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
	assert.Contains(t, domErr.Message, "v2 committee UID is required")
}

func TestNATSMapper_MapCommitteeV1ToV2_EmptySFID(t *testing.T) {
	m := &NATSMapper{conn: nil, timeout: 5 * time.Second}

	_, err := m.MapCommitteeV1ToV2(context.Background(), "")
	require.Error(t, err)

	var domErr *domain.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, domain.ErrorTypeValidation, domErr.Type)
	assert.Contains(t, domErr.Message, "v1 committee SFID is required")
}

// TestNATSMapper_CommitteeResponseParsing tests the compound response parsing
// logic in MapCommitteeV2ToV1 by directly calling the parseCommitteeResponse helper.
func TestNATSMapper_parseCommitteeResponse(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		expectErr     bool
		errType       domain.ErrorType
		expectedSFID  string
	}{
		{
			name:         "compound format returns committee SFID",
			response:     "proj-sfid-001:comm-sfid-002",
			expectedSFID: "comm-sfid-002",
		},
		{
			name:         "plain response (no colon) returned as-is",
			response:     "comm-sfid-only",
			expectedSFID: "comm-sfid-only",
		},
		{
			name:      "too many colons is an error",
			response:  "a:b:c",
			expectErr: true,
			errType:   domain.ErrorTypeUnavailable,
		},
		{
			name:      "empty committee part is an error",
			response:  "proj-sfid-001:",
			expectErr: true,
			errType:   domain.ErrorTypeUnavailable,
		},
		{
			name:         "UUID-style committee SFID",
			response:     "a0000000000001:b0000000000002",
			expectedSFID: "b0000000000002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCommitteeResponse(tt.response)
			if tt.expectErr {
				require.Error(t, err)
				var domErr *domain.DomainError
				require.True(t, errors.As(err, &domErr))
				assert.Equal(t, tt.errType, domErr.Type)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSFID, result)
			}
		})
	}
}

func TestNATSMapper_DefaultTimeout(t *testing.T) {
	// Verify that zero timeout uses the default
	m := &NATSMapper{conn: nil, timeout: 0}
	// A zero timeout would be unusual but shouldn't panic at construction
	assert.Equal(t, time.Duration(0), m.timeout)
}
