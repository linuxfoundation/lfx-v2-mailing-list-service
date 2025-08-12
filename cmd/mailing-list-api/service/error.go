// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func wrapError(ctx context.Context, err error) error {
	f := func(err error) error {
		switch e := err.(type) {
		case errors.Validation:
			return &mailinglistservice.BadRequestError{
				Message: e.Error(),
			}
		case errors.NotFound:
			return &mailinglistservice.NotFoundError{
				Message: e.Error(),
			}
		case errors.ServiceUnavailable:
			return &mailinglistservice.ServiceUnavailableError{
				Message: e.Error(),
			}
		default:
			return &mailinglistservice.InternalServerError{
				Message: e.Error(),
			}
		}
	}

	slog.ErrorContext(ctx, "request failed", "error", err)
	return f(err)
}
