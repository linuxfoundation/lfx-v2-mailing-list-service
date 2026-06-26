// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

const userReaderTimeout = 10 * time.Second

// NATSUserReader implements port.UserReader using NATS request/reply to the auth service.
type NATSUserReader struct {
	nc     Requester
	logger *slog.Logger
}

// NewUserReader creates a new NATS-based user reader.
func NewUserReader(nc Requester, logger *slog.Logger) *NATSUserReader {
	logger.Info("user reader initialized", "subject", constants.AuthEmailToUsernameSubject)
	return &NATSUserReader{nc: nc, logger: logger}
}

// UsernameByEmail returns the LFX username for the LFID account that owns the given email
// address. Returns port.ErrUserNotFound when no user matches.
func (r *NATSUserReader) UsernameByEmail(ctx context.Context, email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", port.ErrUserNotFound
	}

	reqCtx, cancel := context.WithTimeout(ctx, userReaderTimeout)
	defer cancel()

	msg, err := r.nc.RequestWithContext(reqCtx, constants.AuthEmailToUsernameSubject, []byte(email))
	if err != nil {
		return "", fmt.Errorf("email_to_username request failed: %w", err)
	}

	body := strings.TrimSpace(string(msg.Data))
	if body == "" {
		return "", port.ErrUserNotFound
	}

	// The auth service may reply with either:
	//   (a) a bare username string (legacy), or
	//   (b) a JSON envelope {"success": bool, "error": string} /
	//       {"success": true, "username": string} (newer contract).
	if body[0] == '{' {
		var envelope struct {
			Success *bool  `json:"success"`
			Error   string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(msg.Data, &envelope); err != nil {
			return "", fmt.Errorf("failed to parse email_to_username response: %w", err)
		}
		if envelope.Success == nil {
			return "", fmt.Errorf("email_to_username response missing success field")
		}
		if !*envelope.Success {
			if errMsg := strings.TrimSpace(envelope.Error); errMsg != "" && !isEmailToUsernameNotFound(errMsg) {
				return "", fmt.Errorf("email_to_username failed: %s", errMsg)
			}
			return "", port.ErrUserNotFound
		}
		// Success envelope: extract username field if present.
		var successEnvelope struct {
			Username string `json:"username"`
		}
		if jsonErr := json.Unmarshal(msg.Data, &successEnvelope); jsonErr == nil && successEnvelope.Username != "" {
			return successEnvelope.Username, nil
		}
		return "", fmt.Errorf("unexpected email_to_username success envelope: %s", body)
	}

	return body, nil
}

func isEmailToUsernameNotFound(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "not found") || strings.Contains(lower, "no user")
}

var _ port.UserReader = (*NATSUserReader)(nil)
