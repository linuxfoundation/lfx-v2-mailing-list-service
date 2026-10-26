// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package errors

import (
	"errors"
	"strings"
)

// classified marks an error as transient or permanent. Adapters wrap the errors
// they understand so callers never have to inspect storage- or transport-specific
// error types or messages.
type classified struct {
	err       error
	transient bool
}

func (c *classified) Error() string { return c.err.Error() }

// Unwrap exposes the underlying error to support errors.Is / errors.As.
func (c *classified) Unwrap() error { return c.err }

// MarkTransient wraps err to record that it is expected to resolve on retry.
// It returns nil when err is nil.
func MarkTransient(err error) error {
	if err == nil {
		return nil
	}
	return &classified{err: err, transient: true}
}

// MarkPermanent wraps err to record that retrying cannot fix it.
// It returns nil when err is nil.
func MarkPermanent(err error) error {
	if err == nil {
		return nil
	}
	return &classified{err: err, transient: false}
}

// IsPermanent reports whether err was explicitly marked permanent with
// MarkPermanent. Unclassified errors are not permanent: callers should retry
// them rather than drop the work.
func IsPermanent(err error) bool {
	var c *classified
	return errors.As(err, &c) && !c.transient
}

// IsTransient reports whether err is likely to resolve on retry.
// An explicit MarkTransient or MarkPermanent classification wins; otherwise it
// falls back to matching common keywords indicating network or availability
// failures: timeout, connection, unavailable, and deadline.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	var c *classified
	if errors.As(err, &c) {
		return c.transient
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "connection") ||
		strings.Contains(s, "unavailable") ||
		strings.Contains(s, "deadline")
}
