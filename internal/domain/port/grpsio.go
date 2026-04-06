// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

// GroupsIOReaderWriter combines all ITX proxy operations into a single interface.
type GroupsIOReaderWriter interface {
	GroupsIOServiceReader
	GroupsIOServiceWriter
	GroupsIOMailingListReader
	GroupsIOMailingListWriter
	GroupsIOMailingListMemberReader
	GroupsIOMailingListMemberWriter
	GroupsIOArtifactReader
}
