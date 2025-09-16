// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// NATS subject constants for message publishing
const (
	// Indexing subjects for search and discovery
	IndexGroupsIOServiceSubject     = "lfx.index.groupsio_service"
	IndexGroupsIOMailingListSubject = "lfx.index.groupsio_mailing_list"
	IndexGroupsIOMemberSubject      = "lfx.index.groupsio_member"

	// Access control subjects for OpenFGA integration
	UpdateAccessGroupsIOServiceSubject    = "lfx.update_access.groupsio_service"
	DeleteAllAccessGroupsIOServiceSubject = "lfx.delete_all_access.groupsio_service"

	UpdateAccessGroupsIOMailingListSubject    = "lfx.update_access.groupsio_mailing_list"
	DeleteAllAccessGroupsIOMailingListSubject = "lfx.delete_all_access.groupsio_mailing_list"
)
