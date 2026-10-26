// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

const accessCheckTimeout = 5 * time.Second

// NATSAccessChecker implements port.AccessChecker using NATS request/reply to fga-sync.
type NATSAccessChecker struct {
	nc     Requester
	logger *slog.Logger
}

// NewAccessChecker creates a new NATS-based access checker.
func NewAccessChecker(nc Requester, logger *slog.Logger) *NATSAccessChecker {
	if logger == nil {
		logger = slog.Default()
	}
	return &NATSAccessChecker{nc: nc, logger: logger}
}

// requestLineDelimiters are the characters with meaning in the
// "object#relation@user" request line format. A value containing any of
// these (or a newline, which separates lines in a batched request) could
// split or alter the line fga-sync parses, so object and relation are
// rejected if they contain one rather than being interpolated unescaped.
const requestLineDelimiters = "#@\t\n"

// userDelimiters are the characters with meaning in the "user" position of
// the request line. fga-sync splits the line on the first "#" and then the
// first "@", so only one "@" is structurally significant — everything after
// it, including further "@" characters, is taken verbatim as the user
// value. "@" is therefore allowed here (unlike requestLineDelimiters) so
// that client-credentials principals such as Heimdall's
// "user:<client_id>@clients" are not rejected before the request is sent.
const userDelimiters = "#\t\n"

// HasRelation reports whether user has relation on object, per fga-sync's
// access-check reply format: the request line echoed back with a
// tab-separated "true"/"false" result appended.
func (c *NATSAccessChecker) HasRelation(ctx context.Context, user, relation, object string) (bool, error) {
	for name, v := range map[string]string{"relation": relation, "object": object} {
		if v == "" || strings.ContainsAny(v, requestLineDelimiters) {
			return false, fmt.Errorf("invalid access_check %s: must be non-empty and free of %q", name, requestLineDelimiters)
		}
	}
	if user == "" || strings.ContainsAny(user, userDelimiters) {
		return false, fmt.Errorf("invalid access_check user: must be non-empty and free of %q", userDelimiters)
	}

	reqCtx, cancel := context.WithTimeout(ctx, accessCheckTimeout)
	defer cancel()

	line := fmt.Sprintf("%s#%s@%s", object, relation, user)
	msg, err := c.nc.RequestWithContext(reqCtx, constants.AccessCheckSubject, []byte(line+"\n"))
	if err != nil {
		return false, fmt.Errorf("access_check request failed: %w", err)
	}

	body := strings.TrimSpace(string(msg.Data))
	_, result, found := strings.Cut(body, "\t")
	if !found {
		return false, fmt.Errorf("unexpected access_check response: %s", body)
	}

	switch strings.TrimSpace(result) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected access_check result: %s", result)
	}
}

var _ port.AccessChecker = (*NATSAccessChecker)(nil)
