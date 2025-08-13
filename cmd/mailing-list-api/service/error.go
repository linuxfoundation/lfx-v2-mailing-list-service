// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	lfxerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func wrapError(ctx context.Context, err error) error {
	slog.ErrorContext(ctx, "request failed", "error", err)
	
	// Unwrap to get the underlying error for direct type assertion
	unwrappedErr := errors.Unwrap(err)
	if unwrappedErr == nil {
		unwrappedErr = err
	}
	
	switch unwrappedErr.(type) {
	case lfxerrors.Validation:
		return &mailinglistservice.BadRequestError{Message: err.Error()}
	case lfxerrors.NotFound:
		return &mailinglistservice.NotFoundError{Message: err.Error()}
	case lfxerrors.ServiceUnavailable:
		return &mailinglistservice.ServiceUnavailableError{Message: err.Error()}
	default:
		return &mailinglistservice.InternalServerError{Message: err.Error()}
	}
}
