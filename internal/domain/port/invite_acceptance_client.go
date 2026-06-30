// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// InviteAcceptanceClient enriches member records after an LFID invite is accepted.
// It posts the acceptor's email and new username to the ITX backend so that all
// mailing-list member records tied to that email are updated with the username.
type InviteAcceptanceClient interface {
	AcceptInvite(ctx context.Context, email, username string) error
}
