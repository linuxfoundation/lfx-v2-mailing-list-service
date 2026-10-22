// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// AccessChecker checks a single OpenFGA relationship tuple via fga-sync.
type AccessChecker interface {
	// HasRelation reports whether user has relation on object (e.g.
	// user="user:jdoe", relation="writer", object="groupsio_mailing_list:abc").
	HasRelation(ctx context.Context, user, relation, object string) (bool, error)
}
