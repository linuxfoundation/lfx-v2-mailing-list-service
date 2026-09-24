// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunRejectsResponseWithoutResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page_token":""}`))
	}))
	defer server.Close()

	err := run(context.Background(), server.Client(), server.URL, "", &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "missing resources field") {
		t.Fatalf("expected missing resources error, got %v", err)
	}
}

func TestRunPaginatesAndCountsDomains(t *testing.T) {
	var requests []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("page_token") {
		case "":
			_, _ = fmt.Fprint(w, `{"resources":[{"data":{}},{"data":{"domain":"  "}}],"page_token":"next-page"}`)
		case "next-page":
			_, _ = fmt.Fprint(w, `{"resources":[{"data":{"domain":"lists.example.org"}},{"data":{"domain":""}}],"page_token":""}`)
		default:
			http.Error(w, "unexpected page token", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	var output strings.Builder
	if err := run(context.Background(), server.Client(), server.URL, "", &output); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
	if got := requests[1].Get("page_token"); got != "next-page" {
		t.Errorf("second page_token = %q, want %q", got, "next-page")
	}
	for i, query := range requests {
		if got := query.Get("v"); got != "1" {
			t.Errorf("request %d v = %q, want %q", i+1, got, "1")
		}
		if got := query.Get("type"); got != "groupsio_mailing_list" {
			t.Errorf("request %d type = %q, want groupsio_mailing_list", i+1, got)
		}
	}

	want := "Pages: 2\nVisible mailing lists: 4\nVisible mailing lists with domain: 1\nVisible mailing lists without domain: 3\n"
	if got := output.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunReturnsOutputWriteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resources":[],"page_token":""}`))
	}))
	defer server.Close()

	err := run(context.Background(), server.Client(), server.URL, "", failingWriter{})
	if err == nil || err.Error() != "write failed" {
		t.Fatalf("expected writer error, got %v", err)
	}
}

func TestRunRequiresHTTPSEndpointWhenTokenIsSet(t *testing.T) {
	err := run(context.Background(), http.DefaultClient, "http://example.com/resources", "secret", &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("expected HTTPS requirement error, got %v", err)
	}
}

func TestRunAllowsSameOriginHTTPSRedirectWithToken(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/resources", http.StatusTemporaryRedirect)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization header = %q, want bearer token", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"resources":[],"page_token":""}`))
	}))
	defer server.Close()

	endpoint := server.URL + "/start"
	err := run(context.Background(), server.Client(), endpoint, "secret", &strings.Builder{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRunRejectsCrossOriginRedirectWithToken(t *testing.T) {
	redirectTargetHit := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectTargetHit = true
		_, _ = w.Write([]byte(`{"resources":[],"page_token":""}`))
	}))
	defer target.Close()

	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	err := run(context.Background(), source.Client(), source.URL, "secret", &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "refusing token-bearing redirect") {
		t.Fatalf("expected cross-origin redirect rejection, got %v", err)
	}
	if redirectTargetHit {
		t.Fatal("cross-origin redirect target was requested")
	}
}
