// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"

	"gopkg.in/yaml.v3"
)

// translatorMappingsFile is the YAML schema for translator_mappings.yaml.
type translatorMappingsFile struct {
	Mappings map[string]string `yaml:"mappings"`
}

// MockTranslator implements port.Translator using a static YAML mapping file.
// It mirrors the key-building and response-parsing logic of NATSTranslator so
// that service code behaves identically in mock and production modes.
type MockTranslator struct {
	mappings map[string]string
}

var _ port.Translator = (*MockTranslator)(nil)

// NewMockTranslator parses filePath (a YAML file with a "mappings" map) and
// returns a MockTranslator ready to use. Returns an error if the file cannot
// be read or parsed.
func NewMockTranslator(filePath string) (*MockTranslator, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("mock translator: read mappings file %q: %w", filePath, err)
	}

	var f translatorMappingsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("mock translator: parse mappings file %q: %w", filePath, err)
	}

	if f.Mappings == nil {
		f.Mappings = make(map[string]string)
	}

	return &MockTranslator{mappings: f.Mappings}, nil
}

// MapID translates fromID according to subject and direction, using the static
// YAML mappings. The key format and committee response parsing are identical to
// NATSTranslator so that callers see no behavioural difference.
func (m *MockTranslator) MapID(_ context.Context, subject, direction, fromID string) (string, error) {
	if fromID == "" {
		return "", errs.NewValidation(fmt.Sprintf("%s ID is required", subject))
	}

	key, err := mockBuildKey(subject, direction, fromID)
	if err != nil {
		return "", err
	}

	response, ok := m.mappings[key]
	if !ok || response == "" {
		return "", errs.NewValidation(fmt.Sprintf("mapping not found for %s", key))
	}

	if subject == constants.TranslationSubjectCommittee && direction == constants.TranslationDirectionV2ToV1 {
		return mockParseCommitteeV2ToV1Response(response)
	}

	return response, nil
}

func mockBuildKey(subject, direction, fromID string) (string, error) {
	switch direction {
	case constants.TranslationDirectionV2ToV1:
		return fmt.Sprintf("%s.uid.%s", subject, fromID), nil
	case constants.TranslationDirectionV1ToV2:
		return fmt.Sprintf("%s.sfid.%s", subject, fromID), nil
	default:
		return "", errs.NewValidation(fmt.Sprintf("unknown translation direction: %s", direction))
	}
}

func mockParseCommitteeV2ToV1Response(response string) (string, error) {
	parts := strings.Split(response, ":")
	if len(parts) == 1 {
		return response, nil
	}
	if len(parts) != 2 || parts[1] == "" {
		return "", errs.NewServiceUnavailable(fmt.Sprintf("unexpected committee mapping format: %s", response))
	}
	return parts[1], nil
}
