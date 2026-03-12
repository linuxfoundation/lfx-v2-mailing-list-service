// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package mapconv provides typed field extraction from map[string]any payloads.
//
// JSON decoded with json.Unmarshal produces map[string]any where numeric values
// are float64 and compound values are []any or map[string]any. Each function
// handles these standard representations so callers do not need to branch on
// the raw type.
package mapconv

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// StringVal extracts a string value from data[key].
// Numeric values are formatted with %g (no unnecessary trailing zeros).
// Returns "" if the key is absent or nil.
func StringVal(data map[string]any, key string) string {
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// Int64Ptr extracts a nullable int64 from data[key].
// Accepts float64 (standard JSON number) or string representations.
// Returns nil if the key is absent, nil, or unparseable.
func Int64Ptr(data map[string]any, key string) *int64 {
	v, ok := data[key]
	if !ok || v == nil {
		return nil
	}
	var n int64
	switch t := v.(type) {
	case float64:
		if t != math.Trunc(t) {
			return nil
		}
		n = int64(t)
	case string:
		if t == "" {
			return nil
		}
		parsed, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return nil
		}
		n = parsed
	default:
		return nil
	}
	return &n
}

// IntVal extracts an int from data[key].
// Accepts float64 or string representations.
// Returns 0 if the key is absent, nil, or unparseable.
func IntVal(data map[string]any, key string) int {
	v, ok := data[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		if t != math.Trunc(t) {
			return 0
		}
		return int(t)
	case string:
		n, err := strconv.Atoi(t)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// BoolVal extracts a bool from data[key].
// Accepts JSON boolean or the strings "true" / "false" (case-insensitive).
// Returns false if the key is absent or the value cannot be interpreted.
func BoolVal(data map[string]any, key string) bool {
	v, ok := data[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

// StringSliceVal extracts a []string from data[key].
// Accepts a JSON array of strings or a bare string (returned as a one-element slice).
// Returns nil if the key is absent or the value is an empty string.
func StringSliceVal(data map[string]any, key string) []string {
	v, ok := data[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		return nil
	}
}
