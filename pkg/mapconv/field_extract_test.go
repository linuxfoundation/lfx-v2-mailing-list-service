// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mapconv_test

import (
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
	"github.com/stretchr/testify/assert"
)

func TestStringVal(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		key      string
		expected string
	}{
		{"string value", map[string]any{"k": "hello"}, "k", "hello"},
		{"float64 whole number", map[string]any{"k": float64(42)}, "k", "42"},
		{"float64 with decimals", map[string]any{"k": float64(3.14)}, "k", "3.14"},
		{"nil value", map[string]any{"k": nil}, "k", ""},
		{"missing key", map[string]any{}, "k", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapconv.StringVal(tt.data, tt.key))
		})
	}
}

func TestInt64Ptr(t *testing.T) {
	ptr := func(n int64) *int64 { return &n }

	tests := []struct {
		name     string
		data     map[string]any
		key      string
		expected *int64
	}{
		{"float64 value", map[string]any{"k": float64(99)}, "k", ptr(99)},
		{"string value", map[string]any{"k": "12345"}, "k", ptr(12345)},
		{"empty string", map[string]any{"k": ""}, "k", nil},
		{"nil value", map[string]any{"k": nil}, "k", nil},
		{"missing key", map[string]any{}, "k", nil},
		{"unparseable string", map[string]any{"k": "abc"}, "k", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapconv.Int64Ptr(tt.data, tt.key))
		})
	}
}

func TestIntVal(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		key      string
		expected int
	}{
		{"float64 value", map[string]any{"k": float64(7)}, "k", 7},
		{"string value", map[string]any{"k": "250"}, "k", 250},
		{"nil value", map[string]any{"k": nil}, "k", 0},
		{"missing key", map[string]any{}, "k", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapconv.IntVal(tt.data, tt.key))
		})
	}
}

func TestBoolVal(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		key      string
		expected bool
	}{
		{"bool true", map[string]any{"k": true}, "k", true},
		{"bool false", map[string]any{"k": false}, "k", false},
		{"string true", map[string]any{"k": "true"}, "k", true},
		{"string TRUE uppercase", map[string]any{"k": "TRUE"}, "k", true},
		{"string false", map[string]any{"k": "false"}, "k", false},
		{"nil value", map[string]any{"k": nil}, "k", false},
		{"missing key", map[string]any{}, "k", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapconv.BoolVal(tt.data, tt.key))
		})
	}
}

func TestStringSliceVal(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		key      string
		expected []string
	}{
		{"array of strings", map[string]any{"k": []any{"a", "b", "c"}}, "k", []string{"a", "b", "c"}},
		{"single string", map[string]any{"k": "only"}, "k", []string{"only"}},
		{"empty string", map[string]any{"k": ""}, "k", nil},
		{"nil value", map[string]any{"k": nil}, "k", nil},
		{"missing key", map[string]any{}, "k", nil},
		{"non-string items in array are skipped", map[string]any{"k": []any{"a", float64(1), "b"}}, "k", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapconv.StringSliceVal(tt.data, tt.key))
		})
	}
}
