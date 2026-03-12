// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package proxy provides the ITX HTTP proxy client for GroupsIO operations.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/oauth"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
	"golang.org/x/oauth2"
)

const (
	tokenExpiryLeeway = 60 * time.Second
	itxScope          = "read:projects manage:groupsio"
)

// Config holds ITX proxy configuration
type Config struct {
	BaseURL     string
	ClientID    string
	PrivateKey  string // RSA private key in PEM format
	Auth0Domain string
	Audience    string
	Timeout     time.Duration
}

// Client implements domain.ITXGroupsioClient
type Client struct {
	httpClient *http.Client
	config     Config
}

// auth0TokenSource implements oauth2.TokenSource using Auth0 SDK with private key
type auth0TokenSource struct {
	ctx        context.Context
	authConfig *authentication.Authentication
	audience   string
}

// Token implements the oauth2.TokenSource interface
func (a *auth0TokenSource) Token() (*oauth2.Token, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	body := oauth.LoginWithClientCredentialsRequest{
		Audience:        a.audience,
		ExtraParameters: map[string]string{"scope": itxScope},
	}

	tokenSet, err := a.authConfig.OAuth.LoginWithClientCredentials(ctx, body, oauth.IDTokenValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Auth0: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  tokenSet.AccessToken,
		TokenType:    tokenSet.TokenType,
		RefreshToken: tokenSet.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn)*time.Second - tokenExpiryLeeway),
	}

	token = token.WithExtra(map[string]any{
		"scope": tokenSet.Scope,
	})

	return token, nil
}

// NewClient creates a new ITX proxy client with OAuth2 M2M authentication using private key
func NewClient(config Config) *Client {
	ctx := context.Background()

	if config.PrivateKey == "" {
		panic("ITX_CLIENT_PRIVATE_KEY is required but not set")
	}

	authConfig, err := authentication.New(
		ctx,
		config.Auth0Domain,
		authentication.WithClientID(config.ClientID),
		authentication.WithClientAssertion(config.PrivateKey, "RS256"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create Auth0 client: %v (ensure ITX_CLIENT_PRIVATE_KEY contains a valid RSA private key in PEM format)", err))
	}

	tokenSource := &auth0TokenSource{
		ctx:        ctx,
		authConfig: authConfig,
		audience:   config.Audience,
	}

	reuseTokenSource := oauth2.ReuseTokenSource(nil, tokenSource)
	httpClient := oauth2.NewClient(ctx, reuseTokenSource)
	httpClient.Timeout = config.Timeout

	return &Client{
		httpClient: httpClient,
		config:     config,
	}
}

// doRequest performs an HTTP request and returns the raw response body
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	slog.DebugContext(ctx, "ITX request", "method", method, "url", url, "body_bytes", len(body))

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, domain.NewInternalError("failed to create request", err)
	}

	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		slog.ErrorContext(ctx, "ITX request failed", "method", method, "url", url, "error", err)
		return nil, 0, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, domain.NewInternalError("failed to read response", err)
	}

	slog.DebugContext(ctx, "ITX response", "method", method, "url", url, "status", resp.StatusCode, "body_bytes", len(respBody))

	return respBody, resp.StatusCode, nil
}

// mapHTTPError converts HTTP status codes to domain errors
func (c *Client) mapHTTPError(statusCode int, body []byte) error {
	msg := string(body)
	switch statusCode {
	case http.StatusNotFound:
		return domain.NewNotFoundError(fmt.Sprintf("resource not found: %s", msg))
	case http.StatusBadRequest:
		return domain.NewValidationError(fmt.Sprintf("bad request: %s", msg))
	case http.StatusConflict:
		return domain.NewConflictError(fmt.Sprintf("conflict: %s", msg))
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return domain.NewUnavailableError(fmt.Sprintf("ITX service unavailable: %s", msg))
	default:
		return domain.NewInternalError(fmt.Sprintf("ITX error (status %d): %s", statusCode, msg))
	}
}

// ---- GroupsioServiceClient implementation ----

// ListServices lists GroupsIO services, optionally filtered by project_id (v1 SFID)
func (c *Client) ListServices(ctx context.Context, projectID string) (*models.GroupsioServiceListResponse, error) {
	url := fmt.Sprintf("%s/groupsio_service", c.config.BaseURL)
	if projectID != "" {
		url += fmt.Sprintf("?project_id=%s", projectID)
	}

	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioServiceListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// CreateService creates a new GroupsIO service
func (c *Client) CreateService(ctx context.Context, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_service", c.config.BaseURL)
	body, status, err := c.doRequest(ctx, http.MethodPost, url, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioService
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// GetService retrieves a GroupsIO service by ID
func (c *Client) GetService(ctx context.Context, serviceID string) (*models.GroupsioService, error) {
	url := fmt.Sprintf("%s/groupsio_service/%s", c.config.BaseURL, serviceID)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioService
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// UpdateService updates a GroupsIO service
func (c *Client) UpdateService(ctx context.Context, serviceID string, req *models.GroupsioServiceRequest) (*models.GroupsioService, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_service/%s", c.config.BaseURL, serviceID)
	body, status, err := c.doRequest(ctx, http.MethodPut, url, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioService
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// DeleteService deletes a GroupsIO service
func (c *Client) DeleteService(ctx context.Context, serviceID string) error {
	url := fmt.Sprintf("%s/groupsio_service/%s", c.config.BaseURL, serviceID)
	body, status, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return c.mapHTTPError(status, body)
	}
	return nil
}

// GetProjects returns projects that have GroupsIO services
func (c *Client) GetProjects(ctx context.Context) (*models.GroupsioServiceProjectsResponse, error) {
	url := fmt.Sprintf("%s/groupsio_service/_projects", c.config.BaseURL)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioServiceProjectsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// FindParentService finds the parent service for a project (v1 SFID)
func (c *Client) FindParentService(ctx context.Context, projectID string) (*models.GroupsioService, error) {
	url := fmt.Sprintf("%s/groupsio_service_find_parent?project_id=%s", c.config.BaseURL, projectID)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioService
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// ---- GroupsioSubgroupClient implementation ----

// ListSubgroups lists subgroups, optionally filtered by project_id and/or committee_id (v1 SFIDs)
func (c *Client) ListSubgroups(ctx context.Context, projectID, committeeID string) (*models.GroupsioSubgroupListResponse, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup", c.config.BaseURL)
	sep := "?"
	if projectID != "" {
		url += fmt.Sprintf("%sproject_id=%s", sep, projectID)
		sep = "&"
	}
	if committeeID != "" {
		url += fmt.Sprintf("%scommittee_id=%s", sep, committeeID)
	}

	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioSubgroupListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// CreateSubgroup creates a new subgroup
func (c *Client) CreateSubgroup(ctx context.Context, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup", c.config.BaseURL)
	body, status, err := c.doRequest(ctx, http.MethodPost, url, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioSubgroup
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// GetSubgroup retrieves a subgroup by ID
func (c *Client) GetSubgroup(ctx context.Context, subgroupID string) (*models.GroupsioSubgroup, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s", c.config.BaseURL, subgroupID)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioSubgroup
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// UpdateSubgroup updates a subgroup
func (c *Client) UpdateSubgroup(ctx context.Context, subgroupID string, req *models.GroupsioSubgroupRequest) (*models.GroupsioSubgroup, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s", c.config.BaseURL, subgroupID)
	body, status, err := c.doRequest(ctx, http.MethodPut, url, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioSubgroup
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// DeleteSubgroup deletes a subgroup
func (c *Client) DeleteSubgroup(ctx context.Context, subgroupID string) error {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s", c.config.BaseURL, subgroupID)
	body, status, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return c.mapHTTPError(status, body)
	}
	return nil
}

// GetSubgroupCount returns the count of subgroups for a project (v1 SFID)
func (c *Client) GetSubgroupCount(ctx context.Context, projectID string) (*models.GroupsioSubgroupCountResponse, error) {
	url := fmt.Sprintf("%s/groupsio/subgroup_count?project=%s", c.config.BaseURL, projectID)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioSubgroupCountResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// GetMemberCount returns the count of members in a subgroup
func (c *Client) GetMemberCount(ctx context.Context, subgroupID string) (*models.GroupsioMemberCountResponse, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/member_count", c.config.BaseURL, subgroupID)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioMemberCountResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// ---- GroupsioMemberClient implementation ----

// ListMembers lists members of a subgroup
func (c *Client) ListMembers(ctx context.Context, subgroupID string) (*models.GroupsioMemberListResponse, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members", c.config.BaseURL, subgroupID)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioMemberListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// AddMember adds a member to a subgroup
func (c *Client) AddMember(ctx context.Context, subgroupID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members", c.config.BaseURL, subgroupID)
	body, status, err := c.doRequest(ctx, http.MethodPost, url, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioMember
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// GetMember retrieves a member by ID
func (c *Client) GetMember(ctx context.Context, subgroupID, memberID string) (*models.GroupsioMember, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members/%s", c.config.BaseURL, subgroupID, memberID)
	body, status, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioMember
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// UpdateMember updates a member
func (c *Client) UpdateMember(ctx context.Context, subgroupID, memberID string, req *models.GroupsioMemberRequest) (*models.GroupsioMember, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members/%s", c.config.BaseURL, subgroupID, memberID)
	body, status, err := c.doRequest(ctx, http.MethodPut, url, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioMember
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}

// DeleteMember removes a member from a subgroup
func (c *Client) DeleteMember(ctx context.Context, subgroupID, memberID string) error {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members/%s", c.config.BaseURL, subgroupID, memberID)
	body, status, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return c.mapHTTPError(status, body)
	}
	return nil
}

// InviteMembers sends invitations to multiple email addresses
func (c *Client) InviteMembers(ctx context.Context, subgroupID string, req *models.GroupsioInviteMembersRequest) error {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s/invitemembers", c.config.BaseURL, subgroupID)
	body, status, err := c.doRequest(ctx, http.MethodPost, url, bodyBytes)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return c.mapHTTPError(status, body)
	}
	return nil
}

// CheckSubscriber checks if an email is subscribed to a subgroup
func (c *Client) CheckSubscriber(ctx context.Context, req *models.GroupsioCheckSubscriberRequest) (*models.GroupsioCheckSubscriberResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_checksubscriber", c.config.BaseURL)
	body, status, err := c.doRequest(ctx, http.MethodPost, url, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.mapHTTPError(status, body)
	}

	var result models.GroupsioCheckSubscriberResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return &result, nil
}
