// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimitMiddleware(t *testing.T) {
	const limit int64 = 1024

	// handler drains the body the way the Goa decoder would.
	newHandler := func(called *bool, readErr *error) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*called = true
			_, *readErr = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		})
	}

	t.Run("passes a body under the limit", func(t *testing.T) {
		var called bool
		var readErr error
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/groupsio/mailing-lists/x", strings.NewReader(strings.Repeat("a", 512)))

		RequestBodyLimitMiddleware(limit)(newHandler(&called, &readErr)).ServeHTTP(rec, req)

		assert.True(t, called)
		assert.NoError(t, readErr)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("rejects a declared Content-Length over the limit with 413 before the handler runs", func(t *testing.T) {
		var called bool
		var readErr error
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/groupsio/mailing-lists/x", strings.NewReader(strings.Repeat("a", int(limit)+1)))

		RequestBodyLimitMiddleware(limit)(newHandler(&called, &readErr)).ServeHTTP(rec, req)

		assert.False(t, called)
		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("stops reading a body of unknown length at the limit", func(t *testing.T) {
		var called bool
		var readErr error
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/groupsio/services", io.NopCloser(strings.NewReader(strings.Repeat("a", int(limit)*4))))
		req.ContentLength = -1

		RequestBodyLimitMiddleware(limit)(newHandler(&called, &readErr)).ServeHTTP(rec, req)

		assert.True(t, called)
		require.Error(t, readErr)
		var mbe *http.MaxBytesError
		assert.ErrorAs(t, readErr, &mbe)
	})

	t.Run("allows the larger webhook limit on the webhook route", func(t *testing.T) {
		var called bool
		var readErr error
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, groupsIOWebhookPath, strings.NewReader(strings.Repeat("a", int(limit)*2)))

		RequestBodyLimitMiddleware(limit)(newHandler(&called, &readErr)).ServeHTTP(rec, req)

		assert.True(t, called)
		assert.NoError(t, readErr)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
