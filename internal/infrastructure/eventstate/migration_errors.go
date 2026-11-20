// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import "errors"

// MigrationFailureCategory is safe to log. Sentinel identity, not error wording,
// determines the recovery category. Raw driver/transport data is never returned.
func MigrationFailureCategory(err error) string {
	switch {
	case errors.Is(err, ErrImportBusy):
		return "import_busy"
	case errors.Is(err, ErrAlreadyVerified):
		return "already_verified"
	case errors.Is(err, ErrEmptySource):
		return "empty_source"
	case errors.Is(err, ErrTargetConflict):
		return "target_conflict"
	case errors.Is(err, ErrTargetExtraRows):
		return "target_extra_rows"
	case errors.Is(err, ErrSourceChanged):
		return "source_changed"
	case errors.Is(err, ErrInvalidSourceState):
		return "invalid_source_state"
	}
	return "infrastructure_failure"
}
