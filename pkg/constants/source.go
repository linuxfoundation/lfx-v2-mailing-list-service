// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

import (
	"fmt"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// Source constants define the origin of operations for business logic tracking
const (
	// SourceAPI indicates the operation originated from our REST API
	SourceAPI = "api"

	// SourceWebhook indicates the operation originated from a Groups.io webhook
	SourceWebhook = "webhook"

	// SourceMock indicates the operation originated from mock/test infrastructure
	SourceMock = "mock"
)

// ValidateSource validates that the source is one of the allowed values
func ValidateSource(source string) error {
	switch source {
	case SourceAPI, SourceWebhook, SourceMock:
		return nil
	case "":
		return errors.NewValidation("source is required")
	default:
		return errors.NewValidation(
			fmt.Sprintf("unsupported source: %s (must be api, webhook, or mock)", source))
	}
}

// ValidSources returns list of all valid sources for documentation
func ValidSources() []string {
	return []string{SourceAPI, SourceWebhook, SourceMock}
}

// SourceDescription returns human-readable description of source behavior
func SourceDescription(source string) string {
	switch source {
	case SourceAPI:
		return "Creates entity in Groups.io via API, then stores locally"
	case SourceWebhook:
		return "Adopts existing Groups.io entity from webhook, stores locally"
	case SourceMock:
		return "Skips Groups.io coordination, stores locally only (testing mode)"
	default:
		return "Unknown source"
	}
}
