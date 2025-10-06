// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// GrpsIOWebhookValidator handles validation of GroupsIO webhook signatures
type GrpsIOWebhookValidator struct {
	Secret string
}

// NewGrpsIOWebhookValidator creates a new GroupsIO webhook validator
func NewGrpsIOWebhookValidator(secret string) port.GrpsIOWebhookValidator {
	return &GrpsIOWebhookValidator{Secret: secret}
}

// ValidateSignature validates the GroupsIO HMAC-SHA1 signature
func (v *GrpsIOWebhookValidator) ValidateSignature(body []byte, signature string) error {
	if v.Secret == "" {
		return fmt.Errorf("webhook secret not configured")
	}

	if signature == "" {
		return fmt.Errorf("missing webhook signature")
	}

	// Calculate HMAC-SHA1 (GroupsIO algorithm)
	mac := hmac.New(sha1.New, []byte(v.Secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		slog.Error("invalid webhook signature")
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}

// IsValidEvent checks if the event type is supported by GroupsIO
func (v *GrpsIOWebhookValidator) IsValidEvent(eventType string) bool {
	validEvents := map[string]bool{
		"created_subgroup": true,
		"deleted_subgroup": true,
		"added_member":     true,
		"removed_member":   true,
		"ban_members":      true,
	}
	return validEvents[eventType]
}
