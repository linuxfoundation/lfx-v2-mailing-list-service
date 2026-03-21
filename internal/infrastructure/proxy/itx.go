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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	pkgauth "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/auth"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/converter"
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

// handleRequestError converts a httpclient error into a domain error.
func (c *itx) handleRequestError(err error) error {
	var retryErr *httpclient.RetryableError
	if errors.As(err, &retryErr) {
		return c.mapHTTPError(retryErr.StatusCode, []byte(retryErr.Message))
	}
	return domain.NewUnavailableError("ITX service request failed", err)
}

// ---- wire ↔ domain translation helpers ----

func fromWireService(w *serviceWire) *model.GroupsIOService {
	if w == nil {
		return nil
	}
	createdAt, _ := converter.ParseRFC3339(w.CreatedAt)
	updatedAt, _ := converter.ParseRFC3339(w.UpdatedAt)
	return &model.GroupsIOService{
		UID:        w.ID,
		ProjectUID: w.ProjectID,
		Type:       w.Type,
		GroupID:    converter.NonZeroInt64(w.GroupID),
		Domain:     w.Domain,
		Prefix:     w.Prefix,
		Status:     w.Status,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

func toWireServiceRequest(svc *model.GroupsIOService) *serviceRequestWire {
	return &serviceRequestWire{
		ProjectID: svc.ProjectUID,
		Type:      svc.Type,
		GroupID:   converter.Int64Val(svc.GroupID),
		Domain:    svc.Domain,
		Prefix:    svc.Prefix,
		Status:    svc.Status,
	}
}

// ---- GroupsIOServiceWriter implementation ----

// CreateService creates a new GroupsIO service.
func (c *itx) CreateService(ctx context.Context, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	bodyBytes, err := json.Marshal(toWireServiceRequest(svc))
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/groupsio_service", c.config.BaseURL)
	resp, err := c.httpClient.Request(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire serviceWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return fromWireService(&wire), nil
}

// UpdateService updates a GroupsIO service.
func (c *itx) UpdateService(ctx context.Context, serviceID string, svc *model.GroupsIOService) (*model.GroupsIOService, error) {
	bodyBytes, err := json.Marshal(toWireServiceRequest(svc))
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
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
		return nil, domain.NewInternalError("failed to parse response", err)
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
		return nil, domain.NewInternalError("failed to parse response", err)
	}
	return fromWireService(&wire), nil
}

// NewProxy creates a new ITX proxy client with OAuth2 M2M authentication using private key.
func NewProxy(ctx context.Context, config Config) (port.GroupsIOServiceWriter, error) {

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
