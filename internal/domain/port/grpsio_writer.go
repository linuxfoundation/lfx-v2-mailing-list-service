// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

// GrpsIOWriter combines all writer operations for services and mailing lists
type GrpsIOWriter interface {
	GrpsIOServiceWriter
	GrpsIOMailingListWriter
}
