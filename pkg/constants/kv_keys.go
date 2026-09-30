// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// ValidKVKeySegment reports whether an ID can be used as a single NATS KV key
// segment without introducing wildcard tokens or extra subject components.
func ValidKVKeySegment(uid string) bool {
	if uid == "" {
		return false
	}
	for _, r := range uid {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '=') {
			return false
		}
	}
	return true
}
