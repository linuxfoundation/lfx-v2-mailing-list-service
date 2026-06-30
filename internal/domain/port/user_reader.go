// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
	"errors"
)

// ErrUserNotFound is returned by UserReader when no registered user matches the email lookup.
var ErrUserNotFound = errors.New("user not found")

// UserReader looks up LFID user data by email.
type UserReader interface {
	UsernameByEmail(ctx context.Context, email string) (string, error)
}
