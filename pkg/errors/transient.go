// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package errors

import "strings"

// IsTransient reports whether err is likely to resolve on retry.
// It matches against common keywords indicating network or availability failures:
// timeout, connection, unavailable, and deadline.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "connection") ||
		strings.Contains(s, "unavailable") ||
		strings.Contains(s, "deadline")
}
