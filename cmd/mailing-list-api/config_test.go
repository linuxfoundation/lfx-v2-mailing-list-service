// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
)

func TestParseEnv_AutoSyncEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset defaults to enabled", value: "", want: true},
		{name: "true", value: "true", want: true},
		{name: "TRUE mixed case", value: "TRUE", want: true},
		{name: "explicit false", value: "false", want: false},
		{name: "false with surrounding whitespace", value: " false ", want: false},
		{name: "FALSE mixed case", value: "FALSE", want: false},
		{name: "0", value: "0", want: false},
		{name: "no", value: "no", want: false},
		{name: "off", value: "off", want: false},
		{name: "unrecognized value defaults to enabled", value: "disabled", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AUTO_SYNC_ENABLED", tt.value)
			env := parseEnv()
			if env.AutoSyncEnabled != tt.want {
				t.Errorf("AUTO_SYNC_ENABLED=%q: got AutoSyncEnabled=%v, want %v", tt.value, env.AutoSyncEnabled, tt.want)
			}
		})
	}
}
