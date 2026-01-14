// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-querystring/query"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/httpclient"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
)

// groupsioBasicAuthRoundTripper implements automatic BasicAuth injection (production pattern)
type groupsioBasicAuthRoundTripper struct {
	client *Client
}

// RoundTrip ensures authentication before non-login requests to groups.io
// and adds authorization header to those requests (production pattern)
func (rt *groupsioBasicAuthRoundTripper) RoundTrip(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	// Skip auth for login requests to avoid infinite recursion
	if strings.Contains(req.URL.Path, "/v1/login") {
		return next(req)
	}

	// Get cached token for this domain
	rt.client.cacheMu.RLock()
	if cache, exists := rt.client.cache[req.Host]; exists {
		cache.mu.RLock()
		if time.Now().Before(cache.expiry) && cache.token != "" {
			// Use cached token with BasicAuth (production pattern: req.SetBasicAuth)
			req.SetBasicAuth(cache.token, "")
			cache.mu.RUnlock()
			rt.client.cacheMu.RUnlock()

			slog.DebugContext(req.Context(), "RoundTripper: using cached Groups.io token",
				"host", req.Host, "path", req.URL.Path)
			return next(req)
		}
		cache.mu.RUnlock()
	}
	rt.client.cacheMu.RUnlock()

	// No valid cached token - authenticate first (production pattern)
	if err := rt.client.WithDomain(req.Context(), req.Host); err != nil {
		return nil, fmt.Errorf("authentication failed in RoundTripper: %w", err)
	}

	// Now get the token and set BasicAuth
	rt.client.cacheMu.RLock()
	if cache, exists := rt.client.cache[req.Host]; exists {
		cache.mu.RLock()
		if cache.token != "" {
			req.SetBasicAuth(cache.token, "")
			slog.DebugContext(req.Context(), "RoundTripper: using fresh Groups.io token",
				"host", req.Host, "path", req.URL.Path)
		}
		cache.mu.RUnlock()
	}
	rt.client.cacheMu.RUnlock()

	return next(req)
}

// tokenCache holds cached authentication tokens per domain
type tokenCache struct {
	token  string
	expiry time.Time
	mu     sync.RWMutex
}

// Client handles all Groups.io API operations with smart token caching
// ClientInterface defines the contract for GroupsIO API operations
type ClientInterface interface {
	CreateGroup(ctx context.Context, domain string, options GroupCreateOptions) (*GroupObject, error)
	DeleteGroup(ctx context.Context, domain string, groupID uint64) error
	CreateSubgroup(ctx context.Context, domain string, parentGroupID uint64, options SubgroupCreateOptions) (*SubgroupObject, error)
	DeleteSubgroup(ctx context.Context, domain string, subgroupID uint64) error
	GetGroup(ctx context.Context, domain string, groupID uint64) (*GroupObject, error)
	DirectAdd(ctx context.Context, domain string, groupID uint64, emails []string, subgroupIDs []uint64) (*DirectAddResultsObject, error)
	UpdateMember(ctx context.Context, domain string, memberID uint64, updates MemberUpdateOptions) error
	UpdateGroup(ctx context.Context, domain string, groupID uint64, updates GroupUpdateOptions) error
	UpdateSubgroup(ctx context.Context, domain string, subgroupID uint64, updates SubgroupUpdateOptions) error
	RemoveMember(ctx context.Context, domain string, memberID uint64) error
	IsReady(ctx context.Context) error
}

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
		MaxDelay:     30 * time.Second, // Cap exponential backoff at 30s
	}

	client := &Client{
		config:     cfg,
		httpClient: httpclient.NewClient(httpConfig),
		cache:      make(map[string]*tokenCache),
	}

	// Register BasicAuth RoundTripper for automatic token injection (production pattern)
	authRoundTripper := &groupsioBasicAuthRoundTripper{client: client}
	client.httpClient.AddRoundTripper(authRoundTripper)

	slog.InfoContext(context.Background(), "Groups.io client initialized with RoundTripper auth pattern")

	return client, nil
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
	err = c.makeRequest(ctx, domain, http.MethodPost, "/creategroup", data, &response)
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

	return c.makeRequest(ctx, domain, http.MethodPost, "/deletegroup", data, nil)
}

// CreateSubgroup creates a subgroup (mailing list) in Groups.io
func (c *Client) CreateSubgroup(ctx context.Context, domain string, parentGroupID uint64, options SubgroupCreateOptions) (*SubgroupObject, error) {
	slog.InfoContext(ctx, "creating subgroup in Groups.io",
		"domain", domain, "parent_group_id", parentGroupID, "subgroup_name", options.GroupName)

	// Set the parent group ID in options if not already set (for backwards compatibility)
	if options.ParentGroupID == 0 {
		options.ParentGroupID = parentGroupID
	}

	data, err := query.Values(options)
	if err != nil {
		return nil, fmt.Errorf("failed to encode options: %w", err)
	}

	var response SubgroupObject
	err = c.makeRequest(ctx, domain, http.MethodPost, "/createsubgroup", data, &response)
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

	return c.makeRequest(ctx, domain, http.MethodPost, "/deletesubgroup", data, nil)
}

// GetGroup retrieves group details from Groups.io (works for both main groups and subgroups)
func (c *Client) GetGroup(ctx context.Context, domain string, groupID uint64) (*GroupObject, error) {
	slog.InfoContext(ctx, "getting group from Groups.io",
		"domain", domain, "group_id", groupID)

	data := url.Values{
		"group_id": {strconv.FormatUint(groupID, 10)},
	}

	var response GroupObject
	err := c.makeRequest(ctx, domain, http.MethodGet, "/getgroup", data, &response)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "group retrieved successfully from Groups.io",
		"group_id", response.ID,
		"subscriber_count", response.SubsCount)

	return &response, nil
}

// DirectAdd adds one or more members to a group or subgroup using the direct_add endpoint
// emails: slice of email addresses to add (comma-separated in API call)
// subgroupIDs: optional slice of subgroup IDs, one per email (comma-separated in API call)
// Returns the full DirectAddResultsObject with all added members and any errors
func (c *Client) DirectAdd(ctx context.Context, domain string, groupID uint64, emails []string, subgroupIDs []uint64) (*DirectAddResultsObject, error) {
	if len(emails) == 0 {
		return nil, fmt.Errorf("at least one email is required")
	}

	// Convert emails to comma-separated string
	emailsStr := strings.Join(emails, ",")

	// Redact emails for logging
	redactedEmails := make([]string, len(emails))
	for i, email := range emails {
		redactedEmails[i] = redaction.RedactEmail(email)
	}

	// Convert subgroup IDs to comma-separated string (if provided)
	var subgroupIDsStr string
	if len(subgroupIDs) > 0 {
		ids := make([]string, len(subgroupIDs))
		for i, id := range subgroupIDs {
			ids[i] = strconv.FormatUint(id, 10)
		}
		subgroupIDsStr = strings.Join(ids, ",")
	}

	slog.InfoContext(ctx, "adding members to Groups.io via direct_add",
		"domain", domain,
		"group_id", groupID,
		"email_count", len(emails),
		"emails", redactedEmails,
		"subgroup_ids", subgroupIDsStr)

	data := url.Values{
		"group_id": {strconv.FormatUint(groupID, 10)},
		"emails":   {emailsStr},
	}

	// Only add subgroup IDs if provided (for adding to subgroups rather than main group)
	if subgroupIDsStr != "" {
		data.Set("subgroupids", subgroupIDsStr)
	}

	var result DirectAddResultsObject
	err := c.makeRequest(ctx, domain, http.MethodPost, "/directadd", data, &result)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "direct_add completed",
		"domain", domain,
		"group_id", groupID,
		"total_emails", result.TotalEmails,
		"added_count", len(result.AddedMembers),
		"error_count", len(result.Errors))

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

	err = c.makeRequest(ctx, domain, http.MethodPost, "/updatemember", data, nil)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "member updated successfully in Groups.io",
		"member_id", memberID)

	return nil
}

// UpdateGroup updates a Groups.io group with the provided options
func (c *Client) UpdateGroup(ctx context.Context, domain string, groupID uint64, updates GroupUpdateOptions) error {
	slog.InfoContext(ctx, "updating group in Groups.io",
		"domain", domain, "group_id", groupID)

	data, err := query.Values(updates)
	if err != nil {
		return fmt.Errorf("failed to encode updates: %w", err)
	}
	data.Set("group_id", strconv.FormatUint(groupID, 10))

	err = c.makeRequest(ctx, domain, http.MethodPost, "/updategroup", data, nil)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "group updated successfully in Groups.io",
		"group_id", groupID)

	return nil
}

// UpdateSubgroup updates a Groups.io subgroup with the provided options
func (c *Client) UpdateSubgroup(ctx context.Context, domain string, subgroupID uint64, updates SubgroupUpdateOptions) error {
	slog.InfoContext(ctx, "updating subgroup in Groups.io",
		"domain", domain, "subgroup_id", subgroupID)

	data, err := query.Values(updates)
	if err != nil {
		return fmt.Errorf("failed to encode updates: %w", err)
	}
	data.Set("subgroup_id", strconv.FormatUint(subgroupID, 10))

	err = c.makeRequest(ctx, domain, http.MethodPost, "/updatesubgroup", data, nil)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "subgroup updated successfully in Groups.io",
		"subgroup_id", subgroupID)

	return nil
}

// RemoveMember removes a single member from a subgroup
func (c *Client) RemoveMember(ctx context.Context, domain string, memberID uint64) error {
	slog.InfoContext(ctx, "removing member from Groups.io",
		"domain", domain, "member_id", memberID)

	data := url.Values{
		"member_id": {strconv.FormatUint(memberID, 10)},
	}

	err := c.makeRequest(ctx, domain, http.MethodPost, "/removemember", data, nil)
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

	// NOTE: BasicAuth is now handled automatically by RoundTripper (production pattern)
	// No manual Authorization header needed - RoundTripper calls req.SetBasicAuth(token, "")

	// Make request using httpclient (with retry logic + RoundTripper auth)
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

// WithDomain ensures authentication for a domain (production pattern)
func (c *Client) WithDomain(ctx context.Context, domain string) error {
	// Check if auth token is already cached
	c.cacheMu.RLock()
	if cache, exists := c.cache[domain]; exists {
		cache.mu.RLock()
		if time.Now().Before(cache.expiry) {
			cache.mu.RUnlock()
			c.cacheMu.RUnlock()
			slog.DebugContext(ctx, "using cached Groups.io login token", "domain", domain)
			return nil
		}
		cache.mu.RUnlock()
	}
	c.cacheMu.RUnlock()

	// Need to get new token
	_, err := c.getToken(ctx, domain)
	if err != nil {
		slog.ErrorContext(ctx, "failed to authenticate with Groups.io", "error", err, "domain", domain)
		return fmt.Errorf("failed to authenticate request: %w", err)
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

	// Need to authenticate - use simplified getToken
	return c.getToken(ctx, domain)
}

// getToken authenticates a user and returns a login token (production pattern)
func (c *Client) getToken(ctx context.Context, domain string) (string, error) {
	// Login endpoint with timeout (prevents hanging on invalid domains)
	loginCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	data := url.Values{
		"email":    {c.config.Email},
		"password": {c.config.Password},
		"token":    {"true"},
	}

	// Set domain header for vhost auth (critical for multi-tenant)
	headers := map[string]string{
		"Host": domain,
	}

	// Use GET with query parameters (production pattern)
	loginURL := c.config.BaseURL + "/v1/login?" + data.Encode()
	resp, err := c.httpClient.Request(loginCtx, http.MethodGet, loginURL, nil, headers)
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

	// Parse JWT for expiry (production pattern)
	expiry := c.parseTokenExpiry(token)

	// Cache token for this domain
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
		return fmt.Errorf("groups.io API unreachable: %w", MapHTTPError(ctx, err))
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("groups.io API unhealthy (status: %d)", resp.StatusCode)
	}
	return nil
}
