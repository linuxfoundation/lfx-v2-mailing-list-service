// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// Committee represents a committee associated with a mailing list.
// Multiple committees can be associated with a single mailing list,
// and any committee grants access (OR logic for access control).
type Committee struct {
	// UID is the unique identifier of the committee (required).
	UID string `json:"uid"`

	// Name is the display name of the committee (read-only, populated by server).
	Name string `json:"name,omitempty"`

	// Filters are the committee member filters that determine which members
	// are synced to the mailing list (e.g., "Voting Rep", "Alternate Voting Rep").
	Filters []string `json:"filters,omitempty"`
}
