// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import "time"

// InviteResult holds the key fields returned by the invite service after an invite is sent.
type InviteResult struct {
	InviteUID      string
	RecipientEmail string
	ExpiresAt      time.Time
}
