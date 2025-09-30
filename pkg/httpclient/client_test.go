// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	config := Config{
		Timeout:      10 * time.Second,
		MaxRetries:   2,
		RetryDelay:   500 * time.Millisecond,
		RetryBackoff: true,
	}

	client := NewClient(config)

	if client.config.Timeout != config.Timeout {
		t.Errorf("Expected timeout %v, got %v", config.Timeout, client.config.Timeout)
	}
	if client.config.MaxRetries != config.MaxRetries {
		t.Errorf("Expected max retries %d, got %d", config.MaxRetries, client.config.MaxRetries)
	}
	if client.httpClient.Timeout != config.Timeout {
		t.Errorf("Expected HTTP client timeout %v, got %v", config.Timeout, client.httpClient.Timeout)
	}
}

func TestClient_Get_Success(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"message": "success"}`))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}))
	defer server.Close()

	config := Config{
		Timeout:      5 * time.Second,
		MaxRetries:   1,
		RetryDelay:   100 * time.Millisecond,
		RetryBackoff: false,
	}

	client := NewClient(config)
	ctx := context.Background()

	headers := map[string]string{
		"Custom-Header": "custom-value",
	}

	resp, err := client.Request(ctx, "GET", server.URL, nil, headers)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	expectedBody := `{"message": "success"}`
	if string(resp.Body) != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, string(resp.Body))
	}
}

func TestClient_Get_NotFound(t *testing.T) {
	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`{"error": "not found"}`))
		if err != nil {
			t.Errorf("Expected no error writing response, got %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	client := NewClient(config)
	ctx := context.Background()

	_, err := client.Request(ctx, "GET", server.URL, nil, nil)

	// Error contract: Non-2xx responses MUST return *RetryableError
	// See client.go lines 142-146 where StatusCode >= 400 creates RetryableError
	require.Error(t, err, "Expected error for 404 status")

	var retryableErr *RetryableError
	require.ErrorAs(t, err, &retryableErr, "Expected *RetryableError for non-2xx response, got %T", err)
	assert.Equal(t, http.StatusNotFound, retryableErr.StatusCode, "Expected status code 404")
	assert.Contains(t, retryableErr.Message, "not found", "Expected error message to contain 'not found'")
}

func TestClient_Retry_ServerError(t *testing.T) {
	callCount := 0

	// Create a test server that fails twice then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"error": "server error"}`))
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"message": "success"}`))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}))
	defer server.Close()

	config := Config{
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryDelay:   10 * time.Millisecond, // Short delay for testing
		RetryBackoff: false,
	}

	client := NewClient(config)
	ctx := context.Background()

	resp, err := client.Request(ctx, "GET", server.URL, nil, nil)
	if err != nil {
		t.Fatalf("Expected no error after retries, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls (2 failures + 1 success), got %d", callCount)
	}
}

func TestClient_Post(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		expectedBody := `{"test": "data"}`
		if string(body) != expectedBody {
			t.Errorf("Expected body '%s', got '%s'", expectedBody, string(body))
		}

		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{"created": true}`))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	client := NewClient(config)
	ctx := context.Background()

	body := strings.NewReader(`{"test": "data"}`)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := client.Request(ctx, "POST", server.URL, body, headers)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status code 201, got %d", resp.StatusCode)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}
	if config.MaxRetries != 2 {
		t.Errorf("Expected default max retries 2, got %d", config.MaxRetries)
	}
	if config.RetryDelay != 1*time.Second {
		t.Errorf("Expected default retry delay 1s, got %v", config.RetryDelay)
	}
	if !config.RetryBackoff {
		t.Error("Expected default retry backoff to be true")
	}
	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected default max delay 30s, got %v", config.MaxDelay)
	}
}

func TestClient_ExponentialBackoff_DelayCapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := Config{
		Timeout:      5 * time.Second,
		MaxRetries:   5,
		RetryDelay:   100 * time.Millisecond,
		RetryBackoff: true,
		MaxDelay:     500 * time.Millisecond,
	}

	client := NewClient(config)
	start := time.Now()

	_, err := client.Request(context.Background(), "GET", server.URL, nil, nil)

	elapsed := time.Since(start)
	// With exponential backoff: 100ms, 200ms, 400ms, 500ms (capped), 500ms (capped)
	// Total expected: ~1.7s + jitter. Allow reasonable range.
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if elapsed >= 4*time.Second {
		t.Errorf("Request took too long: %v", elapsed)
	}
	if elapsed <= 1*time.Second {
		t.Errorf("Request finished too quickly: %v", elapsed)
	}
}

func TestClient_ExponentialBackoff_JitterVariance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := Config{
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryDelay:   200 * time.Millisecond,
		RetryBackoff: true,
		MaxDelay:     1 * time.Second,
	}

	// Run multiple attempts to verify jitter adds randomness
	var durations []time.Duration
	for i := 0; i < 3; i++ {
		client := NewClient(config)
		start := time.Now()

		_, err := client.Request(context.Background(), "GET", server.URL, nil, nil)

		elapsed := time.Since(start)
		durations = append(durations, elapsed)
		if err == nil {
			t.Error("Expected error, got nil")
		}
	}

	// Verify durations are different (jitter working)
	if durations[0] == durations[1] {
		t.Error("Jitter should make durations different")
	}
	if durations[1] == durations[2] {
		t.Error("Jitter should make durations different")
	}
}

func TestNewClient_DefaultMaxDelay(t *testing.T) {
	config := Config{
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryDelay:   100 * time.Millisecond,
		RetryBackoff: true,
		// MaxDelay not set
	}

	client := NewClient(config)

	// Verify default MaxDelay was set
	if client.config.MaxDelay != 30*time.Second {
		t.Errorf("Expected default MaxDelay 30s, got %v", client.config.MaxDelay)
	}
}

// mockRoundTripper for testing RoundTripper functionality
type mockRoundTripper struct {
	called     bool
	headerSet  string
	nextCalled bool
}

func (m *mockRoundTripper) RoundTrip(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	m.called = true

	// Add a test header to verify RoundTripper was executed
	req.Header.Set("X-Mock-RoundTripper", "executed")
	m.headerSet = req.Header.Get("X-Mock-RoundTripper")

	// Call next in chain
	resp, err := next(req)
	if err == nil {
		m.nextCalled = true
	}

	return resp, err
}

// authRoundTripper simulates BasicAuth injection (Groups.io pattern)
type authRoundTripper struct {
	username string
	password string
	called   bool
}

func (a *authRoundTripper) RoundTrip(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	a.called = true

	// Inject BasicAuth like Groups.io pattern
	if a.username != "" {
		req.SetBasicAuth(a.username, a.password)
	}

	return next(req)
}

func TestClient_AddRoundTripper(t *testing.T) {
	config := DefaultConfig()
	client := NewClient(config)

	mock := &mockRoundTripper{}
	client.AddRoundTripper(mock)

	if len(client.roundTrippers) != 1 {
		t.Errorf("Expected 1 RoundTripper, got %d", len(client.roundTrippers))
	}
}

func TestClient_RoundTripper_BasicAuth_Production_Pattern(t *testing.T) {
	// Create test server that validates BasicAuth (Groups.io pattern)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()

		if !ok {
			t.Error("Expected BasicAuth to be present")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Simulate Groups.io token pattern (token as username, empty password)
		expectedToken := "jwt.token.here"
		if username != expectedToken || password != "" {
			t.Errorf("Expected token '%s' with empty password, got '%s':'%s'", expectedToken, username, password)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "authenticated"}`))
	}))
	defer server.Close()

	config := DefaultConfig()
	client := NewClient(config)

	// Add auth RoundTripper that simulates Groups.io pattern
	auth := &authRoundTripper{username: "jwt.token.here", password: ""}
	client.AddRoundTripper(auth)

	ctx := context.Background()
	resp, err := client.Request(ctx, "GET", server.URL, nil, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	if !auth.called {
		t.Error("Expected auth RoundTripper to be called")
	}
}

func TestClient_RoundTripper_MultipleMiddleware(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify both RoundTrippers executed
		if r.Header.Get("X-Mock-RoundTripper") != "executed" {
			t.Errorf("Expected mock RoundTripper header")
		}

		// Verify BasicAuth was set
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("Expected BasicAuth to be set")
		}
		if username != "testuser" || password != "testpass" {
			t.Errorf("Expected BasicAuth testuser:testpass, got %s:%s", username, password)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authenticated": true}`))
	}))
	defer server.Close()

	config := DefaultConfig()
	client := NewClient(config)

	// Add multiple RoundTrippers
	mock := &mockRoundTripper{}
	auth := &authRoundTripper{username: "testuser", password: "testpass"}

	client.AddRoundTripper(mock)
	client.AddRoundTripper(auth)

	ctx := context.Background()
	_, err := client.Request(ctx, "GET", server.URL, nil, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !mock.called {
		t.Error("Expected mock RoundTripper to be called")
	}

	if !auth.called {
		t.Error("Expected auth RoundTripper to be called")
	}
}

func TestClient_RoundTripper_WithRetry(t *testing.T) {
	attempts := 0

	// Create test server that fails first time, succeeds second time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++

		// Verify RoundTripper header on every attempt
		if r.Header.Get("X-Mock-RoundTripper") != "executed" {
			t.Errorf("Expected RoundTripper header on attempt %d", attempts)
		}

		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "server error"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	config := Config{
		Timeout:      5 * time.Second,
		MaxRetries:   2,
		RetryDelay:   100 * time.Millisecond,
		RetryBackoff: false,
	}
	client := NewClient(config)

	mock := &mockRoundTripper{}
	client.AddRoundTripper(mock)

	ctx := context.Background()
	resp, err := client.Request(ctx, "GET", server.URL, nil, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}

	if !mock.called {
		t.Error("Expected RoundTripper to be called")
	}
}
