// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// TestDecodeCommitteeMembers pins the response-decoding contract that
// natsCommitteeMembershipLookup.GetCommitteeMembers relies on: telling the
// {"error":"<msg>"} envelope apart from a success array, classifying the
// envelope message into the right port error type, and handling malformed
// bodies.
func TestDecodeCommitteeMembers(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		wantMembers []port.CommitteeMember
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "member array",
			data: []byte(`[{"uid":"m1","email":"a@example.com"},{"uid":"m2","email":"b@example.com"}]`),
			wantMembers: []port.CommitteeMember{
				{UID: "m1", Email: "a@example.com"},
				{UID: "m2", Email: "b@example.com"},
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:        "empty array",
			data:        []byte(`[]`),
			wantMembers: []port.CommitteeMember{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "not found error",
			data: []byte(`{"error":"committee not found"}`),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var notFound errs.NotFound
				assert.True(t, errors.As(err, &notFound), "expected NotFound, got %T: %v", err, err)
			},
		},
		{
			// The actual wire message produced by lfx-v2-committee-service's GetBase ->
			// rewrapWithUID for a missing committee (committee_reader.go wrapping
			// storage.go's "committee not found" NotFound).
			name: "wire-accurate committee not found message",
			data: []byte(`{"error":"committee \"abc-123\": committee not found: committee UID: abc-123"}`),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var notFound errs.NotFound
				assert.True(t, errors.As(err, &notFound), "expected NotFound, got %T: %v", err, err)
			},
		},
		{
			// A different not-found-shaped failure that does not contain the literal
			// "committee not found" phrase must not be misclassified as a terminal
			// NotFound — it is a distinct failure mode upstream.
			name: "committee settings not found does not classify as committee NotFound",
			data: []byte(`{"error":"committee settings not found: committee UID: abc-123"}`),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var unavailable errs.ServiceUnavailable
				assert.True(t, errors.As(err, &unavailable), "expected ServiceUnavailable, got %T: %v", err, err)
			},
		},
		{
			// lfx-v2-committee-service's ListMembersByCommittee returns a nil slice for a
			// committee with zero members, which marshals to JSON `null` over the wire — a
			// legitimately empty roster, not a malformed reply. Must normalize to [].
			name:        "null reply is normalized to an empty roster",
			data:        []byte(`null`),
			wantMembers: []port.CommitteeMember{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "invalid committee uid error",
			data: []byte(`{"error":"invalid committee UID in list_members request: invalid UUID length: 3"}`),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var validation errs.Validation
				assert.True(t, errors.As(err, &validation), "expected Validation, got %T: %v", err, err)
			},
		},
		{
			name: "unknown upstream error defaults to service unavailable",
			data: []byte(`{"error":"boom: storage index failure"}`),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var unavailable errs.ServiceUnavailable
				assert.True(t, errors.As(err, &unavailable), "expected ServiceUnavailable, got %T: %v", err, err)
			},
		},
		{
			name: "malformed JSON",
			data: []byte(`not-json`),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var unavailable errs.ServiceUnavailable
				assert.True(t, errors.As(err, &unavailable), "expected ServiceUnavailable, got %T: %v", err, err)
			},
		},
		{
			name: "empty body",
			data: []byte{},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var unavailable errs.ServiceUnavailable
				assert.True(t, errors.As(err, &unavailable), "expected ServiceUnavailable, got %T: %v", err, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			members, err := decodeCommitteeMembers(tt.data)
			tt.assertErr(t, err)
			if tt.wantMembers != nil {
				assert.Equal(t, tt.wantMembers, members)
			}
		})
	}
}

// TestGetCommitteeMembers_EmptyUID pins the input-validation guard that runs
// before any NATS round trip.
func TestGetCommitteeMembers_EmptyUID(t *testing.T) {
	c := &natsCommitteeMembershipLookup{}
	members, err := c.GetCommitteeMembers(context.Background(), "")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation), "expected Validation, got %T: %v", err, err)
	assert.Nil(t, members)
}
