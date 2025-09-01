// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

// GrpsIOReader combines all reader operations for services and mailing lists
type GrpsIOReader interface {
	GrpsIOServiceReader
	GrpsIOMailingListReader
}
