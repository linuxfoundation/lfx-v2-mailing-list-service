// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"
	"errors"
)

// ErrV1ObjectDecode identifies malformed source data, distinct from a KV read failure.
var ErrV1ObjectDecode = errors.New("failed to decode v1 object")

// V1ObjectReader reads current v1 service and subgroup objects without generating KV events.
type V1ObjectReader interface {
	GetSubgroup(ctx context.Context, uid string) (map[string]any, bool, error)
	GetService(ctx context.Context, uid string) (map[string]any, bool, error)
}
