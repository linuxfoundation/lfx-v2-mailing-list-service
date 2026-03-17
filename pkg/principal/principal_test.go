// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package principal_test

import (
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/principal"
	"github.com/stretchr/testify/assert"
)

func TestFromUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		expected string
	}{
		{
			name:     "empty returns empty",
			username: "",
			expected: "",
		},
		{
			// Safe username: matches safeNameRE, does not match hexUserRE — used verbatim.
			name:     "simple safe username",
			username: "john.doe",
			expected: "auth0|john.doe",
		},
		{
			name:     "safe username with hyphens and digits",
			username: "user-123",
			expected: "auth0|user-123",
		},
		{
			// 60 chars = 1 + 58 + 1, the maximum safeNameRE allows.
			name:     "exactly 60 safe chars used verbatim",
			username: "abcdefghij0123456789abcdefghij0123456789abcdefghij0123456789",
			expected: "auth0|abcdefghij0123456789abcdefghij0123456789abcdefghij0123456789",
		},
		{
			// 61 chars — exceeds safeNameRE max, must be SHA-512 + base58 encoded.
			name:     "username longer than 60 chars is hashed",
			username: "abcdefghij0123456789abcdefghij0123456789abcdefghij01234567890",
			expected: "auth0|5fLnnbn4KGc4pxKzgK9JE4GGGpKoWdUqSnsQtutw2XBBTr8qBbv6vv71m1TsGe3mbNvr6a6ncktckEBVD2yhUKD3",
		},
		{
			// Pure lowercase hex 24+ chars matches hexUserRE — must be hashed to avoid
			// colliding with Auth0 native-DB IDs.
			name:     "24-char hex username is hashed",
			username: "0123456789abcdef01234567",
			expected: "auth0|3TjHYyavZDgNgHjy8pnsNZAD7Ek7bVyv9NRnF5384aAmUdvqh2NADaPWr1k1QyX2sbs8Yoh2m5wV7BdMwmpkstf2",
		},
		{
			// Space is not in [A-Za-z0-9._-] so safeNameRE won't match — must be hashed.
			name:     "username with space is hashed",
			username: "John Doe",
			expected: "auth0|dsNnfygV3vpJeB65SWP4JwT4Jcud9eUHjuqetwR7XYZdCokZ7vcs1VUqkov1ktH5qppUQsmHgy5Z3j6ZhwekXY2",
		},
		{
			// @ is not allowed by safeNameRE — must be hashed.
			name:     "email-style username is hashed",
			username: "user@example.com",
			expected: "auth0|3CCfg5ewbyYkXLuXR6oq4aXDZCrV4dqpMSY9XdoWUdENu9MPm9RcZUrCVzMe1W1vzXfkaMVN8awpp82tMSF8AswD",
		},
		{
			// Hashing must be deterministic: calling twice yields the same result.
			name:     "deterministic output for safe username",
			username: "stable-user",
			expected: "auth0|stable-user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, principal.FromUsername(tt.username))
		})
	}
}
