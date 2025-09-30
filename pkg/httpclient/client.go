// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package httpclient

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// RoundTripper interface for request middleware (inspired by production pattern)
type RoundTripper interface {
	RoundTrip(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error)
}

// Client represents a generic HTTP client with retry logic and middleware support
type Client struct {
	config        Config
	httpClient    *http.Client
	roundTrippers []RoundTripper
}

// Request represents an HTTP request configuration
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    io.Reader
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	StatusCode int
	Message    string
}

func (e *RetryableError) Error() string {
	return e.Message
}

// Do executes an HTTP request with retry logic
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with optional exponential backoff
			delay := c.config.RetryDelay
			if c.config.RetryBackoff {
				// Clean doubling with overflow protection
				for i := 1; i < attempt && delay < c.config.MaxDelay/2; i++ {
					delay *= 2
				}
				if delay > c.config.MaxDelay {
					delay = c.config.MaxDelay
				}

				// Add jitter (25% random variance) to prevent thundering herd
				// Use crypto/rand for unpredictable jitter across service restarts
				maxJitter := int64(delay / 4)
				if maxJitter > 0 {
					jitterBig, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
					if err == nil {
						delay = delay + time.Duration(jitterBig.Int64())
					}
				}
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		response, err := c.doRequest(ctx, req)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Don't retry on certain errors
		if !c.shouldRetry(err) {
			break
		}
	}

	slog.ErrorContext(ctx, "request failed", "error", lastErr)

	return nil, lastErr
}

// doRequest performs a single HTTP request with RoundTripper middleware
func (c *Client) doRequest(ctx context.Context, reqConfig Request) (*Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, reqConfig.Method, reqConfig.URL, reqConfig.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	httpReq.Header.Set("Accept", "application/json")

	// Set custom headers
	for key, value := range reqConfig.Headers {
		httpReq.Header.Set(key, value)
	}

	// Execute RoundTripper chain (production pattern)
	resp, err := c.executeRoundTripperChain(httpReq, 0)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}

	// Check for HTTP errors
	if resp.StatusCode >= http.StatusBadRequest {
		err := &RetryableError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
		return response, err
	}

	return response, nil
}

// shouldRetry determines if a request should be retried based on the error
func (c *Client) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a retryable error
	if retryableErr, ok := err.(*RetryableError); ok {
		// Retry on server errors and rate limiting
		return retryableErr.StatusCode >= http.StatusInternalServerError || retryableErr.StatusCode == http.StatusTooManyRequests
	}

	// Retry on network-related errors
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network")
}

// Request performs an HTTP request with the specified verb
func (c *Client) Request(ctx context.Context, verb, url string, body io.Reader, headers map[string]string) (*Response, error) {
	req := Request{
		Method:  verb,
		URL:     url,
		Headers: headers,
		Body:    body,
	}
	return c.Do(ctx, req)
}

// executeRoundTripperChain executes the RoundTripper middleware chain
func (c *Client) executeRoundTripperChain(req *http.Request, index int) (*http.Response, error) {
	if index >= len(c.roundTrippers) {
		// Base case: execute the actual HTTP request
		return c.httpClient.Do(req)
	}

	// Execute current RoundTripper with next function
	next := func(req *http.Request) (*http.Response, error) {
		return c.executeRoundTripperChain(req, index+1)
	}

	return c.roundTrippers[index].RoundTrip(req, next)
}

// AddRoundTripper adds a middleware RoundTripper to the client.
// This method is not safe for concurrent use and should only be called
// during client initialization before making any requests.
func (c *Client) AddRoundTripper(rt RoundTripper) {
	c.roundTrippers = append(c.roundTrippers, rt)
}

// NewClient creates a new HTTP client with the given configuration
func NewClient(config Config) *Client {
	// Set sensible default for MaxDelay if not configured
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}

	return &Client{
		config:        config,
		roundTrippers: make([]RoundTripper, 0),
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}
