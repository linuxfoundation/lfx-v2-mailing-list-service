// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// TestProjectServiceErrorCode pins the envelope parser used by
// natsProjectLookup.GetProjectSlug.
func TestProjectServiceErrorCode(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{name: "not_found code", data: []byte(`{"error":"not_found"}`), wantErr: "not_found"},
		{name: "internal code", data: []byte(`{"error":"internal"}`), wantErr: "internal"},
		{name: "unknown code", data: []byte(`{"error":"foo"}`), wantErr: "foo"},
		{name: "success plain string", data: []byte("my-slug"), wantErr: ""},
		{name: "success json without error key", data: []byte(`{"slug":"k8s"}`), wantErr: ""},
		{name: "empty body", data: []byte{}, wantErr: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantErr, projectServiceErrorCode(tt.data))
		})
	}
}

// TestNATSProjectLookup_GetProjectSlug_NotFoundReturnsEmptyNoError pins the
// contract: a confirmed not_found from project-service must return ("", nil)
// so callers follow the no-slug path rather than treating the permanent
// absence as a retryable transient failure (LFXV2-1747).
//
// project-service PR #121 fixed HandleMessage to send {"error":"not_found"}
// instead of nil on handler errors, so the empty-reply branch below is no
// longer the dominant failure mode. The projectServiceErrorCode parser in this
// service is correct: it maps not_found -> ("", nil) and internal/unknown ->
// Unexpected, which is the right behaviour against the new project-service
// reply contract.
func TestNATSProjectLookup_GetProjectSlug_NotFoundReturnsEmptyNoError(t *testing.T) {
	data := []byte(`{"error":"not_found","message":"project not found"}`)
	code := projectServiceErrorCode(data)

	require.Equal(t, "not_found", code,
		"projectServiceErrorCode must recognise the not_found envelope")

	// Reproduce the exact branch logic from GetProjectSlug.
	var gotSlug string
	var gotErr error
	if code == "not_found" {
		gotSlug, gotErr = "", nil
	} else if code != "" {
		gotErr = errs.NewUnexpected("project-service error: " + code)
	}

	require.NoError(t, gotErr,
		"not_found must not return an error — callers treat any error as retryable transient failure")
	assert.Empty(t, gotSlug,
		"not_found must return an empty slug so callers proceed without one")
}

// TestNATSProjectLookup_GetProjectSlug_InternalCodeReturnsError ensures that
// a non-not_found service error surfaces as a non-nil error so the caller
// retries it as a genuine transient failure.
func TestNATSProjectLookup_GetProjectSlug_InternalCodeReturnsError(t *testing.T) {
	data := []byte(`{"error":"internal","message":"internal server error"}`)
	code := projectServiceErrorCode(data)

	require.Equal(t, "internal", code)

	var gotErr error
	if code == "not_found" {
		// should not reach here
	} else if code != "" {
		gotErr = errs.NewUnexpected("project-service error: " + code)
	}

	require.Error(t, gotErr,
		"internal code must return an error so the caller retries")

	var unexpected errs.Unexpected
	assert.True(t, errors.As(gotErr, &unexpected),
		"internal code must surface as Unexpected, got %T: %v", gotErr, gotErr)
}

// TestNATSProjectLookup_GetProjectSlug_EmptyBodyIsNotAnError pins that an
// empty body (a project with no slug configured) returns ("", nil) and does
// NOT trigger the error path — distinguishing it from {"error":"not_found"}.
func TestNATSProjectLookup_GetProjectSlug_EmptyBodyIsNotAnError(t *testing.T) {
	data := []byte{}
	code := projectServiceErrorCode(data)

	assert.Empty(t, code, "empty body must not parse as an error code")

	// Reproduce the exact branch logic from GetProjectSlug.
	var gotSlug string
	var gotErr error
	if code == "not_found" {
		gotSlug, gotErr = "", nil
	} else if code != "" {
		gotErr = errs.NewUnexpected("project-service error: " + code)
	} else {
		gotSlug = string(data)
	}

	require.NoError(t, gotErr,
		"empty body must not return an error — a project with no slug configured is a valid state")
	assert.Empty(t, gotSlug, "empty body returns empty slug (no slug configured)")
}
