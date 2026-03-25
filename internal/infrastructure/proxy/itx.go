// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package proxy provides the ITX HTTP proxy client for GroupsIO operations.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	pkgauth "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/auth"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/httpclient"
	"golang.org/x/oauth2"
)

const itxScope = "read:projects manage:groupsio"

// Config holds ITX proxy configuration.
type Config struct {
	BaseURL     string
	ClientID    string
	PrivateKey  string // RSA private key in PEM format
	Auth0Domain string
	Audience    string
	Timeout     time.Duration
}

// itx implements port.GroupsIOServiceWriter via the ITX HTTP API.
type itx struct {
	httpClient *httpclient.Client
	config     Config
}

// mapHTTPError converts HTTP status codes to domain errors.
func (c *itx) mapHTTPError(statusCode int, body []byte) error {
	msg := string(body)
	switch statusCode {
	case http.StatusNotFound:
		return errs.NewNotFound(fmt.Sprintf("resource not found: %s", msg))
	case http.StatusBadRequest:
		return errs.NewValidation(fmt.Sprintf("bad request: %s", msg))
	case http.StatusConflict:
		return errs.NewConflict(fmt.Sprintf("conflict: %s", msg))
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return errs.NewServiceUnavailable(fmt.Sprintf("ITX service unavailable: %s", msg))
	default:
		return errs.NewUnexpected(fmt.Sprintf("ITX error (status %d): %s", statusCode, msg))
	}
}

// handleRequestError converts a httpclient error into a domain error.
func (c *itx) handleRequestError(err error) error {
	var retryErr *httpclient.RetryableError
	if errors.As(err, &retryErr) {
		return c.mapHTTPError(retryErr.StatusCode, []byte(retryErr.Message))
	}
	return errs.NewServiceUnavailable("ITX service request failed", err)
}

// ---- GroupsIOServiceWriter implementation ----

// CreateService creates a new GroupsIO service.
func (c *itx) CreateService(ctx context.Context, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	bodyBytes, err := json.Marshal(toWireServiceRequest(svc))
	if err != nil {
		return nil, errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_service", c.config.BaseURL)
	resp, err := c.httpClient.Request(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire serviceWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireService(&wire), nil
}

// UpdateService updates a GroupsIO service.
func (c *itx) UpdateService(ctx context.Context, serviceID string, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	bodyBytes, err := json.Marshal(toWireServiceRequest(svc))
	if err != nil {
		return nil, errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_service/%s", c.config.BaseURL, serviceID)
	resp, err := c.httpClient.Request(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	// ITX returns 204 No Content on successful update; fetch the updated resource.
	if len(resp.Body) == 0 {
		return c.getService(ctx, serviceID)
	}

	var wire serviceWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireService(&wire), nil
}

// DeleteService deletes a GroupsIO service.
func (c *itx) DeleteService(ctx context.Context, serviceID string) error {
	url := fmt.Sprintf("%s/groupsio_service/%s", c.config.BaseURL, serviceID)
	_, err := c.httpClient.Request(ctx, http.MethodDelete, url, nil, nil)
	if err != nil {
		return c.handleRequestError(err)
	}
	return nil
}

// getService retrieves a GroupsIO service by ID (used internally by UpdateService on 204 responses).
func (c *itx) getService(ctx context.Context, serviceID string) (*model.GroupsIOService, error) {
	url := fmt.Sprintf("%s/groupsio_service/%s", c.config.BaseURL, serviceID)
	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire serviceWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireService(&wire), nil
}

// ---- GroupsIOMailingListWriter implementation ----

func (c *itx) getSubgroup(ctx context.Context, mailingListID string) (*model.GroupsIOMailingList, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s", c.config.BaseURL, mailingListID)
	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, c.handleRequestError(err)
	}
	var wire subgroupWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireSubgroup(&wire), nil
}

// CreateMailingList creates a new GroupsIO mailing list (subgroup).
func (c *itx) CreateMailingList(ctx context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	bodyBytes, err := json.Marshal(toWireSubgroupRequest(ml))
	if err != nil {
		return nil, errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup", c.config.BaseURL)
	resp, err := c.httpClient.Request(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire subgroupWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireSubgroup(&wire), nil
}

// UpdateMailingList updates an existing GroupsIO mailing list (subgroup).
func (c *itx) UpdateMailingList(ctx context.Context, mailingListID string, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	bodyBytes, err := json.Marshal(toWireSubgroupRequest(ml))
	if err != nil {
		return nil, errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s", c.config.BaseURL, mailingListID)
	resp, err := c.httpClient.Request(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	// ITX returns 204 No Content on successful update; fetch the updated resource.
	if len(resp.Body) == 0 {
		return c.getSubgroup(ctx, mailingListID)
	}

	var wire subgroupWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireSubgroup(&wire), nil
}

// DeleteMailingList deletes a GroupsIO mailing list (subgroup).
func (c *itx) DeleteMailingList(ctx context.Context, mailingListID string) error {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s", c.config.BaseURL, mailingListID)
	_, err := c.httpClient.Request(ctx, http.MethodDelete, url, nil, nil)
	if err != nil {
		return c.handleRequestError(err)
	}
	return nil
}

// ---- GroupsIOMailingListMemberReader implementation ----

// ListMembers lists all members of a GroupsIO mailing list.
func (c *itx) ListMembers(ctx context.Context, mailingListID string) ([]*model.GrpsIOMember, int, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members", c.config.BaseURL, mailingListID)
	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, 0, c.handleRequestError(err)
	}

	var wire memberListResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, 0, errs.NewUnexpected("failed to parse response", err)
	}

	items := make([]*model.GrpsIOMember, len(wire.Data))
	for i, item := range wire.Data {
		items[i] = fromWireMember(item)
	}
	return items, len(items), nil
}

// GetMember retrieves a GroupsIO member by ID.
func (c *itx) GetMember(ctx context.Context, mailingListID string, memberID string) (*model.GrpsIOMember, error) {
	return c.getMember(ctx, mailingListID, memberID)
}

// CheckSubscriber checks whether an email address is subscribed to a GroupsIO mailing list.
func (c *itx) CheckSubscriber(ctx context.Context, mailingListID string, email string) (bool, error) {
	bodyBytes, err := json.Marshal(&checkSubscriberRequestWire{Email: email, SubgroupID: mailingListID})
	if err != nil {
		return false, errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_checksubscriber", c.config.BaseURL)
	resp, err := c.httpClient.Request(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return false, c.handleRequestError(err)
	}

	var wire checkSubscriberResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return false, errs.NewUnexpected("failed to parse response", err)
	}
	return wire.Subscribed, nil
}

// ---- GroupsIOMailingListMemberWriter implementation ----

// getMember retrieves a GroupsIO member by ID (used internally by UpdateMember on 204 responses).
func (c *itx) getMember(ctx context.Context, mailingListID string, memberID string) (*model.GrpsIOMember, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members/%s", c.config.BaseURL, mailingListID, memberID)
	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, c.handleRequestError(err)
	}
	var wire memberWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireMember(&wire), nil
}

// AddMember adds a new member to a GroupsIO mailing list (subgroup).
func (c *itx) AddMember(ctx context.Context, mailingListID string, member *model.GrpsIOMember) (*model.GrpsIOMember, error) {
	bodyBytes, err := json.Marshal(toWireMemberRequest(member))
	if err != nil {
		return nil, errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members", c.config.BaseURL, mailingListID)
	resp, err := c.httpClient.Request(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire memberWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireMember(&wire), nil
}

// UpdateMember updates an existing GroupsIO member.
func (c *itx) UpdateMember(ctx context.Context, mailingListID string, memberID string, member *model.GrpsIOMember) (*model.GrpsIOMember, error) {
	bodyBytes, err := json.Marshal(toWireMemberRequest(member))
	if err != nil {
		return nil, errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members/%s", c.config.BaseURL, mailingListID, memberID)
	resp, err := c.httpClient.Request(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	// ITX returns 204 No Content on successful update; fetch the updated resource.
	if len(resp.Body) == 0 {
		return c.getMember(ctx, mailingListID, memberID)
	}

	var wire memberWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireMember(&wire), nil
}

// DeleteMember removes a member from a GroupsIO mailing list.
func (c *itx) DeleteMember(ctx context.Context, mailingListID string, memberID string) error {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/members/%s", c.config.BaseURL, mailingListID, memberID)
	_, err := c.httpClient.Request(ctx, http.MethodDelete, url, nil, nil)
	if err != nil {
		return c.handleRequestError(err)
	}
	return nil
}

// InviteMembers sends invitations to a list of emails for a GroupsIO mailing list.
func (c *itx) InviteMembers(ctx context.Context, mailingListID string, emails []string) error {
	bodyBytes, err := json.Marshal(&inviteMembersRequestWire{Emails: emails})
	if err != nil {
		return errs.NewUnexpected("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_subgroup/%s/invite_members", c.config.BaseURL, mailingListID)
	_, err = c.httpClient.Request(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return c.handleRequestError(err)
	}
	return nil
}

// ---- GroupsIOMailingListReader implementation ----

// ListMailingLists lists GroupsIO mailing lists, optionally filtered by project and/or committee v1 IDs.
func (c *itx) ListMailingLists(ctx context.Context, projectID string, committeeID string) ([]*model.GroupsIOMailingList, int, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup", c.config.BaseURL)
	sep := "?"
	if projectID != "" {
		url = fmt.Sprintf("%s%sproject_id=%s", url, sep, projectID)
		sep = "&"
	}
	if committeeID != "" {
		url = fmt.Sprintf("%s%scommittee=%s", url, sep, committeeID)
	}

	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, 0, c.handleRequestError(err)
	}

	var wire subgroupListResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, 0, errs.NewUnexpected("failed to parse response", err)
	}

	items := make([]*model.GroupsIOMailingList, len(wire.Items))
	for i, item := range wire.Items {
		items[i] = fromWireSubgroup(item)
	}
	return items, wire.Total, nil
}

// GetMailingList retrieves a GroupsIO mailing list by ID.
func (c *itx) GetMailingList(ctx context.Context, mailingListID string) (*model.GroupsIOMailingList, error) {
	return c.getSubgroup(ctx, mailingListID)
}

// GetMailingListCount returns the count of mailing lists for a given v1 project ID.
func (c *itx) GetMailingListCount(ctx context.Context, projectID string) (int, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/count?project_id=%s", c.config.BaseURL, projectID)
	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return 0, c.handleRequestError(err)
	}

	var wire countResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return 0, errs.NewUnexpected("failed to parse response", err)
	}
	return wire.Count, nil
}

// GetMailingListMemberCount returns the count of members in a given mailing list.
func (c *itx) GetMailingListMemberCount(ctx context.Context, mailingListID string) (int, error) {
	url := fmt.Sprintf("%s/groupsio_subgroup/%s/member_count", c.config.BaseURL, mailingListID)
	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return 0, c.handleRequestError(err)
	}

	var wire countResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return 0, errs.NewUnexpected("failed to parse response", err)
	}
	return wire.Count, nil
}

// ---- GroupsIOServiceReader implementation ----

// listServices retrieves a list of GroupsIO services, optionally filtered by project_id.
func (c *itx) listServices(ctx context.Context, projectID string) (*serviceListResponseWire, error) {
	url := fmt.Sprintf("%s/groupsio_service", c.config.BaseURL)
	if projectID != "" {
		url = fmt.Sprintf("%s?project_id=%s", url, projectID)
	}
	resp, err := c.httpClient.Request(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire serviceListResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return &wire, nil
}

// ListServices lists GroupsIO services, optionally filtered by project_id.
func (c *itx) ListServices(ctx context.Context, projectID string) ([]*model.GroupsIOService, int, error) {
	wire, err := c.listServices(ctx, projectID)
	if err != nil {
		return nil, 0, err
	}

	svcs := make([]*model.GroupsIOService, len(wire.Items))
	for i, item := range wire.Items {
		svcs[i] = fromWireService(item)
	}
	return svcs, wire.Total, nil
}

// GetService retrieves a GroupsIO service by ID.
func (c *itx) GetService(ctx context.Context, serviceID string) (*model.GroupsIOService, error) {
	return c.getService(ctx, serviceID)
}

// GetProjects returns the v1 project IDs that have at least one GroupsIO service.
func (c *itx) GetProjects(ctx context.Context) ([]string, error) {
	wire, err := c.listServices(ctx, "")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(wire.Items))
	for _, item := range wire.Items {
		if item.ProjectID != "" {
			seen[item.ProjectID] = struct{}{}
		}
	}

	projectIDs := make([]string, 0, len(seen))
	for id := range seen {
		projectIDs = append(projectIDs, id)
	}
	return projectIDs, nil
}

// FindParentService finds the primary service for a given v1 project ID.
func (c *itx) FindParentService(ctx context.Context, projectID string) (*model.GroupsIOService, error) {
	wire, err := c.listServices(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for _, item := range wire.Items {
		if item.Type == "v2_primary" {
			return fromWireService(item), nil
		}
	}
	return nil, errs.NewNotFound("no parent service found for project")
}

// NewProxy creates a new ITX proxy client with OAuth2 M2M authentication using private key.
func NewProxy(ctx context.Context, config Config) (port.GroupsIOReaderWriter, error) {

	if config.PrivateKey == "" {
		slog.ErrorContext(ctx, "ITX client private key is not set")
		return nil, fmt.Errorf("ITX client private key is required")
	}

	authConfig, err := authentication.New(
		ctx,
		config.Auth0Domain,
		authentication.WithClientID(config.ClientID),
		authentication.WithClientAssertion(config.PrivateKey, "RS256"),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create Auth0 authentication client", "error", err)
		return nil, fmt.Errorf("failed to create Auth0 authentication client: %w", err)
	}

	tokenSource := pkgauth.NewAuth0TokenSource(ctx, authConfig, config.Audience, itxScope)
	oauthHTTPClient := oauth2.NewClient(ctx, oauth2.ReuseTokenSource(nil, tokenSource))
	oauthHTTPClient.Timeout = config.Timeout

	return &itx{
		httpClient: httpclient.NewClientWithHTTPClient(
			httpclient.Config{
				Timeout: config.Timeout,
			},
			oauthHTTPClient),
		config: config,
	}, nil
}
