// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassification(t *testing.T) {
	base := errors.New("boom")

	tests := []struct {
		name          string
		err           error
		wantTransient bool
		wantPermanent bool
	}{
		{name: "nil", err: nil},
		{name: "unclassified", err: base},
		{name: "unclassified keyword heuristic", err: errors.New("nats: timeout"), wantTransient: true},
		{name: "marked transient", err: MarkTransient(base), wantTransient: true},
		{name: "marked permanent", err: MarkPermanent(base), wantPermanent: true},
		{name: "permanent beats keyword heuristic", err: MarkPermanent(errors.New("connection refused")), wantPermanent: true},
		{name: "transient survives wrapping", err: fmt.Errorf("ctx: %w", MarkTransient(base)), wantTransient: true},
		{name: "permanent survives wrapping", err: fmt.Errorf("ctx: %w", MarkPermanent(base)), wantPermanent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantTransient, IsTransient(tt.err))
			assert.Equal(t, tt.wantPermanent, IsPermanent(tt.err))
		})
	}
}

func TestMarkPreservesCause(t *testing.T) {
	base := errors.New("boom")
	assert.Nil(t, MarkTransient(nil))
	assert.Nil(t, MarkPermanent(nil))
	assert.ErrorIs(t, MarkTransient(base), base)
	assert.ErrorIs(t, MarkPermanent(base), base)
	assert.Equal(t, "boom", MarkPermanent(base).Error())
}
