// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/suite"
)

type TranslatorSuite struct {
	suite.Suite
}

func TestTranslator(t *testing.T) {
	suite.Run(t, new(TranslatorSuite))
}

func (s *TranslatorSuite) TestBuildKey() {
	tests := []struct {
		name        string
		subject     string
		direction   string
		fromID      string
		expectKey   string
		expectError bool
		expectType  string // "validation"
	}{
		{
			name:      "V2ToV1 direction formats uid key",
			subject:   constants.TranslationSubjectProject,
			direction: constants.TranslationDirectionV2ToV1,
			fromID:    "abc-123",
			expectKey: "project.uid.abc-123",
		},
		{
			name:      "V1ToV2 direction formats sfid key",
			subject:   constants.TranslationSubjectProject,
			direction: constants.TranslationDirectionV1ToV2,
			fromID:    "a0B000001",
			expectKey: "project.sfid.a0B000001",
		},
		{
			name:      "committee subject is included in key",
			subject:   constants.TranslationSubjectCommittee,
			direction: constants.TranslationDirectionV2ToV1,
			fromID:    "c-uuid",
			expectKey: "committee.uid.c-uuid",
		},
		{
			name:        "unknown direction returns validation error",
			subject:     constants.TranslationSubjectProject,
			direction:   "bad_direction",
			fromID:      "id",
			expectError: true,
			expectType:  "validation",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			key, err := buildKey(tt.subject, tt.direction, tt.fromID)
			if tt.expectError {
				s.Require().Error(err)
				if tt.expectType == "validation" {
					var valErr errs.Validation
					s.True(errors.As(err, &valErr))
				}
				return
			}
			s.Require().NoError(err)
			s.Equal(tt.expectKey, key)
		})
	}
}

func (s *TranslatorSuite) TestParseCommitteeV2ToV1Response() {
	tests := []struct {
		name        string
		response    string
		expectID    string
		expectError bool
		expectType  string // "service_unavailable"
	}{
		{
			name:     "single value with no colon is returned as-is",
			response: "singleSFID",
			expectID: "singleSFID",
		},
		{
			name:     "valid compound extracts committee SFID after colon",
			response: "projectSFID:committeeSFID",
			expectID: "committeeSFID",
		},
		{
			name:        "empty committee SFID after colon returns error",
			response:    "projectSFID:",
			expectError: true,
			expectType:  "service_unavailable",
		},
		{
			name:        "too many colons returns error",
			response:    "a:b:c",
			expectError: true,
			expectType:  "service_unavailable",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			id, err := parseCommitteeV2ToV1Response(tt.response)
			if tt.expectError {
				s.Require().Error(err)
				if tt.expectType == "service_unavailable" {
					var svcErr errs.ServiceUnavailable
					s.True(errors.As(err, &svcErr))
				}
				return
			}
			s.Require().NoError(err)
			s.Equal(tt.expectID, id)
		})
	}
}

func (s *TranslatorSuite) TestMapID() {
	// These cases return early before any NATS connection is used, so a nil conn is safe.
	tests := []struct {
		name        string
		subject     string
		direction   string
		fromID      string
		expectError bool
		expectType  string // "validation"
	}{
		{
			name:        "empty fromID returns validation error before any NATS call",
			subject:     constants.TranslationSubjectProject,
			direction:   constants.TranslationDirectionV2ToV1,
			fromID:      "",
			expectError: true,
			expectType:  "validation",
		},
		{
			name:        "unknown direction returns validation error before any NATS call",
			subject:     constants.TranslationSubjectProject,
			direction:   "bad_direction",
			fromID:      "some-id",
			expectError: true,
			expectType:  "validation",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := &NATSTranslator{conn: nil}
			_, err := t.MapID(context.Background(), tt.subject, tt.direction, tt.fromID)
			s.Require().Error(err)
			if tt.expectType == "validation" {
				var valErr errs.Validation
				s.True(errors.As(err, &valErr))
			}
		})
	}
}
