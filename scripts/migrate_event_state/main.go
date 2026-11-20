// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// migrate_event_state is the explicit offline import of mailing-list-owned
// legacy mappings state. It does not change NATS or deployed writer settings.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/eventstate"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func run() error {
	apply := flag.Bool("apply", false, "apply schema, import and verify state; all legacy mappings writers must already be stopped")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	s, err := eventstate.Open(ctx, os.Getenv("MAILING_LIST_STATE_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	nc, err := nats.Connect(os.Getenv("NATS_URL"), nats.Timeout(10*time.Second))
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	n, err := s.Import(ctx, js, *apply)
	if err == nil {
		slog.Info("mailing-list state migration verified", "entries", n, "applied", *apply)
	}
	return err
}
func main() {
	if err := run(); err != nil {
		slog.Error("mailing-list state migration failed; verify cutover state before retrying", "failure_category", eventstate.MigrationFailureCategory(err))
		os.Exit(1)
	}
}
