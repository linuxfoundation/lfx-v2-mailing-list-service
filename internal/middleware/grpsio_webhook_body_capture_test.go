// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrpsIOWebhookBodyCaptureMiddleware(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		method             string
		body               string
		expectBodyCaptured bool
		expectContextKey   bool
		expectStatusCode   int
	}{
		{
			name:               "captures body for GroupsIO webhook endpoint",
			path:               "/webhooks/groupsio",
			method:             http.MethodPost,
			body:               `{"action":"subgroup.created","data":{"name":"test"}}`,
			expectBodyCaptured: true,
			expectContextKey:   true,
			expectStatusCode:   http.StatusOK,
		},
		{
			name:               "does not capture body for other endpoints",
			path:               "/api/services",
			method:             http.MethodGet,
			body:               `{"some":"data"}`,
			expectBodyCaptured: false,
			expectContextKey:   false,
			expectStatusCode:   http.StatusOK,
		},
		{
			name:               "handles empty body on webhook endpoint",
			path:               "/webhooks/groupsio",
			method:             http.MethodPost,
			body:               "",
			expectBodyCaptured: true,
			expectContextKey:   true,
			expectStatusCode:   http.StatusOK,
		},
		{
			name:               "does not capture for similar paths",
			path:               "/webhooks/groupsio/other",
			method:             http.MethodPost,
			body:               `{"test":"data"}`,
			expectBodyCaptured: false,
			expectContextKey:   false,
			expectStatusCode:   http.StatusOK,
		},
		{
			name:               "handles GET requests to webhook endpoint",
			path:               "/webhooks/groupsio",
			method:             http.MethodGet,
			body:               "",
			expectBodyCaptured: true,
			expectContextKey:   true,
			expectStatusCode:   http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedBody []byte
			var capturedContext context.Context
			var bodyReadableInHandler bool

			// Test handler that verifies body capture and context
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedContext = r.Context()

				// Try to read body in handler (should work because of NopCloser)
				handlerBody, err := io.ReadAll(r.Body)
				if err == nil && len(handlerBody) > 0 {
					bodyReadableInHandler = true
				}

				// Get body from context
				if bodyBytes, ok := r.Context().Value(constants.GrpsIOWebhookBodyContextKey).([]byte); ok {
					capturedBody = bodyBytes
				}

				w.WriteHeader(http.StatusOK)
			})

			// Wrap handler with middleware
			middleware := GrpsIOWebhookBodyCaptureMiddleware()
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)

			// Verify status code
			assert.Equal(t, tc.expectStatusCode, rec.Code)

			// Verify body capture
			if tc.expectBodyCaptured {
				assert.Equal(t, tc.body, string(capturedBody), "captured body should match original")
				if len(tc.body) > 0 {
					assert.True(t, bodyReadableInHandler, "body should still be readable in handler")
				}
			} else {
				assert.Empty(t, capturedBody, "body should not be captured for non-webhook endpoints")
			}

			// Verify context key
			if tc.expectContextKey {
				contextBody, ok := capturedContext.Value(constants.GrpsIOWebhookBodyContextKey).([]byte)
				assert.True(t, ok, "context should contain body key")
				assert.Equal(t, tc.body, string(contextBody), "context body should match original")
			} else {
				contextBody := capturedContext.Value(constants.GrpsIOWebhookBodyContextKey)
				assert.Nil(t, contextBody, "context should not contain body key for non-webhook endpoints")
			}
		})
	}
}

func TestGrpsIOWebhookBodyCaptureMiddleware_LargeBody(t *testing.T) {
	// Create a body larger than 10MB limit
	largeBody := strings.Repeat("x", 11*1024*1024) // 11MB

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for oversized body")
	})

	middleware := GrpsIOWebhookBodyCaptureMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", strings.NewReader(largeBody))
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "max 10MB allowed")
}

func TestGrpsIOWebhookBodyCaptureMiddleware_BodyPreservation(t *testing.T) {
	testBody := `{"action":"subgroup.created","data":{"id":123,"name":"test-group"}}`

	var firstRead string
	var secondRead string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First read from context
		if bodyBytes, ok := r.Context().Value(constants.GrpsIOWebhookBodyContextKey).([]byte); ok {
			firstRead = string(bodyBytes)
		}

		// Second read from request body
		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		secondRead = string(bodyBytes)

		w.WriteHeader(http.StatusOK)
	})

	middleware := GrpsIOWebhookBodyCaptureMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", strings.NewReader(testBody))
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Both reads should get the same body
	assert.Equal(t, testBody, firstRead, "body from context should match original")
	assert.Equal(t, testBody, secondRead, "body from request should match original")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGrpsIOWebhookBodyCaptureMiddleware_MultipleReads(t *testing.T) {
	testBody := `{"test":"data"}`
	readCount := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body multiple times
		for i := 0; i < 3; i++ {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				readCount++
			}
			// Reset for next read (in real scenario, you'd need to reset manually)
			if i < 2 {
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := GrpsIOWebhookBodyCaptureMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", strings.NewReader(testBody))
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, 3, readCount, "should be able to read body multiple times with reset")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGrpsIOWebhookBodyCaptureMiddleware_SpecialCharacters(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{
			name: "unicode characters",
			body: `{"name":"test-グループ-😀","action":"created"}`,
		},
		{
			name: "escaped characters",
			body: `{"message":"Line1\nLine2\tTabbed"}`,
		},
		{
			name: "special JSON characters",
			body: `{"data":"{\"nested\":\"value\"}"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedBody []byte

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if bodyBytes, ok := r.Context().Value(constants.GrpsIOWebhookBodyContextKey).([]byte); ok {
					capturedBody = bodyBytes
				}
				w.WriteHeader(http.StatusOK)
			})

			middleware := GrpsIOWebhookBodyCaptureMiddleware()
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			assert.Equal(t, tc.body, string(capturedBody), "special characters should be preserved exactly")
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestGrpsIOWebhookBodyCaptureMiddleware_ConcurrentRequests(t *testing.T) {
	const numRequests = 10

	middleware := GrpsIOWebhookBodyCaptureMiddleware()

	results := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			testBody := string(rune('A' + index)) // Different body for each request

			var capturedBody string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if bodyBytes, ok := r.Context().Value(constants.GrpsIOWebhookBodyContextKey).([]byte); ok {
					capturedBody = string(bodyBytes)
				}
				w.WriteHeader(http.StatusOK)
			})

			wrappedHandler := middleware(handler)
			req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", strings.NewReader(testBody))
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			// Verify each request got its own body
			results <- (capturedBody == testBody && rec.Code == http.StatusOK)
		}(i)
	}

	// Verify all requests succeeded
	for i := 0; i < numRequests; i++ {
		success := <-results
		assert.True(t, success, "concurrent request %d should succeed", i)
	}
}

func TestGrpsIOWebhookBodyCaptureMiddleware_ErrorHandling(t *testing.T) {
	tests := []struct {
		name             string
		setupRequest     func() *http.Request
		expectError      bool
		expectStatusCode int
	}{
		{
			name: "handles nil body",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", nil)
				return req
			},
			expectError:      false,
			expectStatusCode: http.StatusOK,
		},
		{
			name: "handles already read body",
			setupRequest: func() *http.Request {
				body := strings.NewReader(`{"test":"data"}`)
				req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", body)
				// Pre-read the body
				_, _ = io.ReadAll(req.Body)
				return req
			},
			expectError:      false,
			expectStatusCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			middleware := GrpsIOWebhookBodyCaptureMiddleware()
			wrappedHandler := middleware(handler)

			req := tc.setupRequest()
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectStatusCode, rec.Code)
			if !tc.expectError {
				assert.True(t, handlerCalled, "handler should be called when no error expected")
			}
		})
	}
}

func TestGrpsIOWebhookBodyCaptureMiddleware_ChainedMiddleware(t *testing.T) {
	testBody := `{"action":"test"}`
	
	// Simulate another middleware that modifies context
	outerMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "outer-key", "outer-value")
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}

	var capturedBody []byte
	var outerValue string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check both middleware values are present
		if bodyBytes, ok := r.Context().Value(constants.GrpsIOWebhookBodyContextKey).([]byte); ok {
			capturedBody = bodyBytes
		}
		if val, ok := r.Context().Value("outer-key").(string); ok {
			outerValue = val
		}
		w.WriteHeader(http.StatusOK)
	})

	// Chain middlewares
	webhookMiddleware := GrpsIOWebhookBodyCaptureMiddleware()
	wrappedHandler := outerMiddleware(webhookMiddleware(handler))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/groupsio", strings.NewReader(testBody))
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Verify both middleware effects are present
	assert.Equal(t, testBody, string(capturedBody), "webhook body should be captured")
	assert.Equal(t, "outer-value", outerValue, "outer middleware value should be present")
	assert.Equal(t, http.StatusOK, rec.Code)
}
