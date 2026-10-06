// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mapconv

import (
	"encoding/json"
	"errors"

	msgpack "github.com/vmihailenco/msgpack/v5"
)

// DecodeMapData decodes a v1-objects entry as JSON or msgpack.
func DecodeMapData(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err == nil {
		return result, nil
	}
	if err := msgpack.Unmarshal(data, &result); err == nil {
		return result, nil
	}
	return nil, errors.New("failed to decode KV data as JSON or msgpack")
}
