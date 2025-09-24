// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/httpclient"
)

// MapHTTPError maps httpclient errors to domain errors with proper context logging
func MapHTTPError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// Check if it's a retryable error from httpclient
	if retryableErr, ok := err.(*httpclient.RetryableError); ok {
		slog.WarnContext(ctx, "Groups.io HTTP error occurred",
			"status_code", retryableErr.StatusCode,
			"message", retryableErr.Message,
		)

		switch retryableErr.StatusCode {
		case http.StatusNotFound:
			return errors.NewNotFound("resource not found in Groups.io", err)
		case http.StatusConflict:
			return errors.NewConflict("resource already exists in Groups.io", err)
		case http.StatusUnauthorized:
			return errors.NewUnauthorized("Groups.io authentication failed", err)
		case http.StatusForbidden:
			return errors.NewValidation("Groups.io access denied", err)
		case http.StatusTooManyRequests:
			return errors.NewServiceUnavailable("Groups.io rate limited", err)
		case http.StatusBadRequest:
			return errors.NewValidation(fmt.Sprintf("Groups.io validation error: %s", retryableErr.Message), err)
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return errors.NewServiceUnavailable("Groups.io service unavailable", err)
		default:
			slog.ErrorContext(ctx, "Unexpected Groups.io HTTP status code",
				"status_code", retryableErr.StatusCode,
				"message", retryableErr.Message,
			)
			return errors.NewUnexpected("Groups.io API error", err)
		}
	}

	// Handle other error types (network, timeout, etc.)
	slog.ErrorContext(ctx, "Groups.io request failed with non-HTTP error",
		"error", err.Error(),
	)
	return errors.NewUnexpected("Groups.io request failed", err)
}

// WrapGroupsIOError wraps a Groups.io API error response with proper context logging
func WrapGroupsIOError(ctx context.Context, errObj *ErrorObject) error {
	if errObj == nil {
		return nil
	}

	slog.WarnContext(ctx, "Groups.io API error response",
		"error_type", errObj.Type,
		"message", errObj.Message,
		"code", errObj.Code,
	)

	switch errObj.Type {
	case "validation_error":
		return errors.NewValidation(errObj.Message)
	case "not_found":
		return errors.NewNotFound(errObj.Message)
	case "conflict":
		return errors.NewConflict(errObj.Message)
	case "unauthorized":
		return errors.NewUnauthorized(errObj.Message)
	case "forbidden":
		return errors.NewValidation(errObj.Message) // Access denied, not auth failure
	case "rate_limited":
		return errors.NewServiceUnavailable(errObj.Message)
	default:
		slog.ErrorContext(ctx, "Unknown Groups.io error type",
			"error_type", errObj.Type,
			"message", errObj.Message,
		)
		return errors.NewUnexpected(errObj.Message)
	}
}