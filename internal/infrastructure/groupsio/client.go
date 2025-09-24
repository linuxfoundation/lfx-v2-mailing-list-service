// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-querystring/query"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/httpclient"
)

// tokenCache holds cached authentication tokens per domain
type tokenCache struct {
	token  string
	expiry time.Time
	mu     sync.RWMutex
}

// Client handles all Groups.io API operations with smart token caching
type Client struct {
	config     Config
	httpClient *httpclient.Client
	cache      map[string]*tokenCache // domain -> token
	cacheMu    sync.RWMutex
}

// NewClient creates a new GroupsIO client with the given configuration
func NewClient(cfg Config) (*Client, error) {
	if cfg.MockMode {
		return nil, nil // Return nil for mock mode - orchestrator will handle this
	}

	// Validate required configuration
	if cfg.Email == "" || cfg.Password == "" {
		return nil, fmt.Errorf("email and password are required for Groups.io client")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.groups.io"
	}

	// Use reusable httpclient from pkg
	httpConfig := httpclient.Config{
		Timeout:      cfg.Timeout,
		MaxRetries:   cfg.MaxRetries,
		RetryDelay:   cfg.RetryDelay,
		RetryBackoff: true,
	}

	return &Client{
		config:     cfg,
		httpClient: httpclient.NewClient(httpConfig),
		cache:      make(map[string]*tokenCache),
	}, nil
}

// CreateGroup creates a main group in Groups.io
func (c *Client) CreateGroup(ctx context.Context, domain string, options GroupCreateOptions) (*GroupObject, error) {
	slog.InfoContext(ctx, "creating group in Groups.io",
		"domain", domain, "group_name", options.GroupName)

	data, err := query.Values(options)
	if err != nil {
		return nil, fmt.Errorf("failed to encode options: %w", err)
	}

	var response GroupObject
	err = c.makeRequest(ctx, domain, "POST", "/creategroup", data, &response)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "group created successfully in Groups.io",
		"group_id", response.ID, "domain", domain)

	return &response, nil
}

// DeleteGroup removes a group from Groups.io
func (c *Client) DeleteGroup(ctx context.Context, domain string, groupID uint64) error {
	slog.InfoContext(ctx, "deleting group from Groups.io",
		"domain", domain, "group_id", groupID)

	data := url.Values{
		"group_id": {strconv.FormatUint(groupID, 10)},
	}

	return c.makeRequest(ctx, domain, "POST", "/deletegroup", data, nil)
}

// CreateSubgroup creates a subgroup (mailing list) in Groups.io
func (c *Client) CreateSubgroup(ctx context.Context, domain string, parentGroupID uint64, options SubgroupCreateOptions) (*SubgroupObject, error) {
	slog.InfoContext(ctx, "creating subgroup in Groups.io",
		"domain", domain, "parent_group_id", parentGroupID, "subgroup_name", options.SubgroupName)

	data, err := query.Values(options)
	if err != nil {
		return nil, fmt.Errorf("failed to encode options: %w", err)
	}
	data.Set("group_id", strconv.FormatUint(parentGroupID, 10))

	var response SubgroupObject
	err = c.makeRequest(ctx, domain, "POST", "/createsubgroup", data, &response)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "subgroup created successfully in Groups.io",
		"subgroup_id", response.ID, "parent_group_id", parentGroupID)

	return &response, nil
}

// DeleteSubgroup removes a subgroup from Groups.io
func (c *Client) DeleteSubgroup(ctx context.Context, domain string, subgroupID uint64) error {
	slog.InfoContext(ctx, "deleting subgroup from Groups.io",
		"domain", domain, "subgroup_id", subgroupID)

	data := url.Values{
		"subgroup_id": {strconv.FormatUint(subgroupID, 10)},
	}

	return c.makeRequest(ctx, domain, "POST", "/deletesubgroup", data, nil)
}

// AddMember adds a single member to a subgroup
func (c *Client) AddMember(ctx context.Context, domain string, subgroupID uint64, email, name string) (*MemberObject, error) {
	slog.InfoContext(ctx, "adding member to Groups.io",
		"domain", domain, "subgroup_id", subgroupID, "email", email)

	data := url.Values{
		"subgroup_id": {strconv.FormatUint(subgroupID, 10)},
		"email":       {email},
		"name":        {name},
	}

	var result MemberObject
	err := c.makeRequest(ctx, domain, "POST", "/addmember", data, &result)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "member added successfully to Groups.io",
		"member_id", result.ID, "subgroup_id", subgroupID, "email", email)

	return &result, nil
}

// UpdateMember updates member properties (e.g., mod_status)
func (c *Client) UpdateMember(ctx context.Context, domain string, memberID uint64, updates MemberUpdateOptions) error {
	slog.InfoContext(ctx, "updating member in Groups.io",
		"domain", domain, "member_id", memberID)

	data, err := query.Values(updates)
	if err != nil {
		return fmt.Errorf("failed to encode updates: %w", err)
	}
	data.Set("member_id", strconv.FormatUint(memberID, 10))

	err = c.makeRequest(ctx, domain, "POST", "/updatemember", data, nil)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "member updated successfully in Groups.io",
		"member_id", memberID)

	return nil
}

// RemoveMember removes a single member from a subgroup
func (c *Client) RemoveMember(ctx context.Context, domain string, memberID uint64) error {
	slog.InfoContext(ctx, "removing member from Groups.io",
		"domain", domain, "member_id", memberID)

	data := url.Values{
		"member_id": {strconv.FormatUint(memberID, 10)},
	}

	err := c.makeRequest(ctx, domain, "POST", "/removemember", data, nil)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "member removed successfully from Groups.io",
		"member_id", memberID)

	return nil
}

// makeRequest centralizes all API calls with authentication and error handling
func (c *Client) makeRequest(ctx context.Context, domain string, method string, path string, data url.Values, result interface{}) error {
	// Get or refresh token for domain
	token, err := c.getOrRefreshToken(ctx, domain)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Build request URL
	reqURL := c.config.BaseURL + "/v1" + path

	var body io.Reader
	headers := map[string]string{}

	if method == "POST" && data != nil {
		// Add cached token to POST data (Groups.io pattern)
		data.Set("csrf", token)
		body = strings.NewReader(data.Encode())
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	} else if method == "GET" && data != nil {
		reqURL += "?" + data.Encode()
	}

	// Set domain header for vhost auth (critical for multi-tenant)
	headers["Host"] = domain

	// Make request using httpclient (with retry logic)
	resp, err := c.httpClient.Request(ctx, method, reqURL, body, headers)
	if err != nil {
		return MapHTTPError(ctx, err)
	}

	// Parse response
	if result != nil {
		if err := json.Unmarshal(resp.Body, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// getOrRefreshToken implements smart token caching with JWT expiry
func (c *Client) getOrRefreshToken(ctx context.Context, domain string) (string, error) {
	c.cacheMu.RLock()
	if cache, exists := c.cache[domain]; exists {
		cache.mu.RLock()
		if time.Now().Before(cache.expiry) {
			token := cache.token
			cache.mu.RUnlock()
			c.cacheMu.RUnlock()
			return token, nil
		}
		cache.mu.RUnlock()
	}
	c.cacheMu.RUnlock()

	// Need to authenticate
	return c.authenticate(ctx, domain)
}

// authenticate implements Groups.io login with JWT parsing (from go-groupsio pattern)
func (c *Client) authenticate(ctx context.Context, domain string) (string, error) {
	// Login endpoint with timeout (prevents hanging on invalid domains)
	loginCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data := url.Values{
		"email":    {c.config.Email},
		"password": {c.config.Password},
		"token":    {"true"},
	}

	// Set domain header for vhost auth (critical for multi-tenant)
	headers := map[string]string{
		"Host":         domain,
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, err := c.httpClient.Request(loginCtx, "POST", c.config.BaseURL+"/v1/login",
		strings.NewReader(data.Encode()), headers)
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", MapHTTPError(loginCtx, err))
	}

	var loginResp LoginObject
	if err := json.Unmarshal(resp.Body, &loginResp); err != nil {
		return "", fmt.Errorf("login response parse failed: %w", err)
	}

	token := loginResp.Token
	if token == "" {
		return "", fmt.Errorf("no token in login response")
	}

	// Parse JWT for expiry (from go-groupsio pattern)
	expiry := c.parseTokenExpiry(token)

	// Cache token
	c.cacheMu.Lock()
	if c.cache[domain] == nil {
		c.cache[domain] = &tokenCache{}
	}
	cache := c.cache[domain]
	c.cacheMu.Unlock()

	cache.mu.Lock()
	cache.token = token
	cache.expiry = expiry
	cache.mu.Unlock()

	slog.InfoContext(ctx, "Groups.io authentication successful",
		"domain", domain, "expires_at", expiry.Format(time.RFC3339))

	return token, nil
}

// parseTokenExpiry extracts expiry from JWT (reused from go-groupsio)
func (c *Client) parseTokenExpiry(token string) time.Time {
	parser := jwt.Parser{}
	claims := jwt.MapClaims{}

	_, _, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		slog.Warn("failed to parse JWT token", "error", err)
		return time.Now().Add(10 * time.Minute) // Default TTL
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		slog.Warn("no expiry in JWT token", "error", err)
		return time.Now().Add(10 * time.Minute) // Default TTL
	}

	// Cache until 1 minute before expiry
	return exp.Time.Add(-1 * time.Minute)
}

// IsReady checks if Groups.io API is accessible
func (c *Client) IsReady(ctx context.Context) error {
	resp, err := c.httpClient.Request(ctx, "GET", c.config.BaseURL, nil, nil)
	if err != nil {
		return fmt.Errorf("Groups.io API unreachable: %w", MapHTTPError(ctx, err))
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Groups.io API unhealthy (status: %d)", resp.StatusCode)
	}
	return nil
}