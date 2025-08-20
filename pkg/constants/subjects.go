// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// NATS subject constants for message publishing
const (
	// Indexing subjects for search and discovery
	IndexGrpsIOServiceSubject = "lfx.index.grpsio_service"

	// Access control subjects for OpenFGA integration
	UpdateAccessGrpsIOServiceSubject    = "lfx.update_access.grpsio_service"
	DeleteAllAccessGrpsIOServiceSubject = "lfx.delete_all_access.grpsio_service"
)
