// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"strconv"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// etagValidator validates ETag format and converts to uint64 for optimistic locking
// Following exact committee service pattern: etagValidator
func etagValidator(etag *string) (uint64, error) {
	// Parse ETag to get revision for optimistic locking
	if etag == nil || *etag == "" {
		return 0, errors.NewValidation("ETag is required for update operations")
	}

	parsedRevision, errParse := strconv.ParseUint(*etag, 10, 64)
	if errParse != nil {
		return 0, errors.NewValidation("invalid ETag format", errParse)
	}

	return parsedRevision, nil
}
