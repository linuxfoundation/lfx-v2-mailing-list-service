// Copyright 2025 The Linux Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
