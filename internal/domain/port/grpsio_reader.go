// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

// GrpsIOReader combines all reader operations for services, mailing lists, and members
type GrpsIOReader interface {
	GrpsIOServiceReader
	GrpsIOMailingListReader
	GrpsIOMemberReader
}
