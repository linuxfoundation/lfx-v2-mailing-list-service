// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// SubgroupObjectReader reads current v1 subgroup objects without generating KV events.
type SubgroupObjectReader interface {
	GetSubgroup(ctx context.Context, uid string) (map[string]any, bool, error)
	GetService(ctx context.Context, uid string) (map[string]any, bool, error)
}
