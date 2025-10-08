// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// GrpsIOWebhookBodyCaptureMiddleware captures the raw request body before GOA parsing
// Required for signature validation which needs the exact raw bytes
func GrpsIOWebhookBodyCaptureMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only capture body for GroupsIO webhook endpoints
			if r.URL.Path == "/webhooks/groupsio" {
				// Limit body size to prevent memory exhaustion (e.g., 10MB)
				r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Failed to read request body", http.StatusBadRequest)
					return
				}

				// Replace body so GOA can still read it
				r.Body = io.NopCloser(bytes.NewReader(body))

				// Store raw body in context for validator
				ctx := context.WithValue(r.Context(), constants.GrpsIOWebhookBodyContextKey, body)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
