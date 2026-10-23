// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"log/slog"
	"net/http"
)

const (
	// DefaultMaxRequestBodyBytes caps request bodies for the JSON API routes.
	// Payloads are small JSON documents, so 1 MiB is generous.
	DefaultMaxRequestBodyBytes int64 = 1 << 20

	// webhookMaxRequestBodyBytes matches the cap applied by
	// GrpsIOWebhookBodyCaptureMiddleware to the GroupsIO webhook route.
	webhookMaxRequestBodyBytes int64 = 10 << 20

	groupsIOWebhookPath = "/webhooks/groupsio"
)

// RequestBodyLimitMiddleware rejects request bodies larger than maxBytes before
// the Goa decoder buffers them. A declared Content-Length over the limit gets a
// 413 immediately; bodies without a trustworthy length (chunked) are wrapped in
// http.MaxBytesReader so reads stop at the limit instead of growing with the
// caller. The GroupsIO webhook route keeps its own larger limit.
func RequestBodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit := maxBytes
			if r.URL.Path == groupsIOWebhookPath && webhookMaxRequestBodyBytes > limit {
				limit = webhookMaxRequestBodyBytes
			}

			if r.ContentLength > limit {
				slog.WarnContext(r.Context(), "request body too large",
					"content_length", r.ContentLength,
					"limit", limit,
					"method", r.Method,
				)
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
