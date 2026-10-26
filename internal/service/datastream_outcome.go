// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// outcome is what a datastream handler decides to do with the message it was given.
type outcome int

const (
	// ack finishes the message: it was processed, or it can never be processed.
	ack outcome = iota
	// nak asks the data-stream consumer to redeliver the message after a delay.
	nak
)

// requeue reports whether the consumer should redeliver the message. It is the
// boundary conversion to the bool contract of port.DataEventHandler.
func (o outcome) requeue() bool { return o == nak }

// outcomeForError decides the outcome for a failed I/O step. Only an error an
// adapter explicitly marked permanent is ACKed; everything else is retried, so
// an unrecognised failure is bounded by the consumer's MaxDeliver instead of
// silently dropping an event the v1 sync will not resend.
func outcomeForError(err error) outcome {
	if pkgerrors.IsPermanent(err) {
		return ack
	}
	return nak
}

// failed logs a failed I/O step and returns the outcome for it. A retry is logged
// as a warning; a dropped message is logged as an error.
func failed(ctx context.Context, msg string, err error, attrs ...any) outcome {
	o := outcomeForError(err)
	attrs = append(attrs, "error", err)
	if o == nak {
		slog.WarnContext(ctx, msg+", will retry", attrs...)
	} else {
		slog.ErrorContext(ctx, msg+", ACKing permanent failure", attrs...)
	}
	return o
}
