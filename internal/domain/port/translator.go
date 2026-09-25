// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// Translator translates identifiers between the LFX v2 UUID space (our API)
// and the v1 SFID space used by ITX.
//
// Implementations resolve mappings via the v1-sync-helper NATS request/reply
// protocol on subject "lfx.lookup_v1_mapping".
//
// Use constants from pkg/constants for subject and direction values:
//
//	v1SFID, err := translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV2ToV1, v2UUID)
//	v2UUID, err := translator.MapID(ctx, constants.TranslationSubjectProject, constants.TranslationDirectionV1ToV2, v1SFID)
type Translator interface {
	// MapID translates fromID according to the given subject and direction.
	// Returns the translated ID or an error if the mapping is not found.
	MapID(ctx context.Context, subject, direction, fromID string) (string, error)
}
