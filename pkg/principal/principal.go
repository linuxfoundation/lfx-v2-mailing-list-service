// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package principal converts v1 usernames to the Auth0 "sub" format expected by v2
// services: "auth0|{userID}".
//
// The mapping logic is shared with lfx-v1-sync-helper and must remain in sync with it.
package principal

import (
	"crypto/sha512"
	"regexp"

	"github.com/akamensky/base58"
)

var (
	// safeNameRE matches usernames safe to use directly as Auth0 user IDs.
	safeNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,58}[A-Za-z0-9]$`)
	// hexUserRE matches usernames that could collide with Auth0 native-DB hexadecimal IDs.
	hexUserRE = regexp.MustCompile(`^[0-9a-f]{24,60}$`)
)

// FromUsername converts a raw v1 username to the Auth0 "sub" format: "auth0|{userID}".
//
// Safe usernames (matching safeNameRE but not hexUserRE) are used verbatim as the userID.
// All others are SHA-512 hashed and base58-encoded (~80 chars) to handle legacy usernames
// that are too long, contain non-standard characters, or risk colliding with future Auth0
// native-DB hexadecimal IDs.
//
// Returns an empty string when username is empty.
func FromUsername(username string) string {
	if username == "" {
		return ""
	}

	var userID string
	if safeNameRE.MatchString(username) && !hexUserRE.MatchString(username) {
		userID = username
	} else {
		hash := sha512.Sum512([]byte(username))
		userID = base58.Encode(hash[:])
	}

	return "auth0|" + userID
}
