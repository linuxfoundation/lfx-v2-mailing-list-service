// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package main provides HTTP server setup and configuration for the mailing list API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"goa.design/clue/debug"
	goahttp "goa.design/goa/v3/http"

	mailinglistservicesvr "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/http/mailing_list/server"
	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/middleware"
)

// setupHTTPServer configures and starts a HTTP server on the given host address.
// It shuts down the server using the context received from shutdownCtxC, which
// must be sent by the caller exactly once after cancelling the request context.
// Using a shared context ensures srv.Shutdown and the caller's wg.Wait both
// observe the same absolute deadline.
func setupHTTPServer(ctx context.Context, host string, mailingListServiceEndpoints *mailinglistservice.Endpoints, wg *sync.WaitGroup, errc chan error, dbg bool, koDataPath string, shutdownCtxC <-chan context.Context) {
	mux := buildMux(dbg)

	koDataDir := http.Dir(koDataPath)
	eh := errorHandler()
	mailingListServiceServer := mailinglistservicesvr.New(
		mailingListServiceEndpoints, mux,
		goahttp.RequestDecoder, goahttp.ResponseEncoder,
		eh, nil,
		koDataDir, koDataDir, koDataDir, koDataDir,
	)
	mailinglistservicesvr.Mount(mux, mailingListServiceServer)

	var handler http.Handler = mux
	handler = middleware.RequestIDMiddleware()(handler)
	handler = middleware.AuthorizationMiddleware()(handler)
	if dbg {
		handler = debug.HTTP()(handler)
	}
	handler = otelhttp.NewHandler(handler, "mailing-list-api",
		otelhttp.WithFilter(func(r *http.Request) bool {
			p := r.URL.Path
			return p != mailinglistservicesvr.LivezMailingListPath() && p != mailinglistservicesvr.ReadyzMailingListPath()
		}),
	)

	for _, m := range mailingListServiceServer.Mounts {
		slog.InfoContext(ctx, "HTTP endpoint mounted",
			"method", m.Method,
			"verb", m.Verb,
			"pattern", m.Pattern,
		)
	}

	srv := &http.Server{
		Addr:              host,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
	}

	(*wg).Add(1)
	go func() {
		defer (*wg).Done()

		go func() {
			slog.InfoContext(ctx, "HTTP server listening", "host", host)
			// ListenAndServe always returns a non-nil error. ErrServerClosed is
			// the normal result when srv.Shutdown is called, so it is not a
			// failure. Only unexpected errors (e.g. port already in use) are
			// forwarded to errc so run() can distinguish them from OS signals
			// and return the correct exit code.
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				select {
				case errc <- err:
				case <-ctx.Done():
				}
			}
		}()

		<-ctx.Done()
		slog.InfoContext(ctx, "shutting down HTTP server", "host", host)

		// Use the shared shutdown context provided by run() so that srv.Shutdown
		// and the outer wg.Wait observe the same absolute deadline.
		shutdownCtx := <-shutdownCtxC
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.ErrorContext(shutdownCtx, "failed to shutdown HTTP server", "error", err)
		}
	}()
}

// buildMux constructs the GOA HTTP mux with route-tagging OTel middleware.
// Debug profiling endpoints are mounted when dbg is true.
func buildMux(dbg bool) goahttp.MiddlewareMuxer {
	mux := goahttp.NewMuxer()

	// Register route-tagging middleware before any mounts so chi sees it for all
	// routes. The pattern is read after next.ServeHTTP because chi populates it
	// during routing (inside ServeHTTP), not before.
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rctx := chi.RouteContext(r.Context())
				if rctx == nil {
					return
				}
				routePattern := rctx.RoutePattern()
				if routePattern == "" {
					return
				}
				if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
					labeler.Add(semconv.HTTPRoute(routePattern))
				}
				span := trace.SpanFromContext(r.Context())
				span.SetAttributes(semconv.HTTPRoute(routePattern))
				span.SetName(r.Method + " " + routePattern)
			}()
			next.ServeHTTP(w, r)
		})
	})

	if dbg {
		debug.MountPprofHandlers(debug.Adapt(mux))
		debug.MountDebugLogEnabler(debug.Adapt(mux))
	}

	return mux
}

// errorHandler returns a GOA error handler that logs unhandled HTTP errors.
func errorHandler() func(context.Context, http.ResponseWriter, error) {
	return func(ctx context.Context, _ http.ResponseWriter, err error) {
		slog.ErrorContext(ctx, "HTTP error occurred", "error", err)
	}
}
