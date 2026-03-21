// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package port defines the interfaces for external dependencies and adapters.
package port

// GroupsIOReaderWriter combines service reader and writer into a single interface,
// used by infrastructure adapters (e.g. the ITX proxy) that implement both.
type GroupsIOReaderWriter interface {
	GroupsIOServiceReader
	GroupsIOServiceWriter
}
