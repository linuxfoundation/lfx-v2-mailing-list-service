// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package eventstate persists only mailing-list-owned processing state. The
// separate subgroup service index stays in NATS and canonical helper mappings
// are accessed through the helper RPC by the composite adapter.
package eventstate

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

//go:embed schema.sql
var schema string

const opTimeout = 5 * time.Second

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("MAILING_LIST_STATE_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid mailing-list state database configuration")
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.ConnectTimeout = opTimeout
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("mailing-list state database pool unavailable")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("mailing-list state database unavailable")
	}
	return &Store{pool: pool}, nil
}
func (s *Store) Close() error { s.pool.Close(); return nil }
func (s *Store) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	var ready bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM mailing_list_state_cutover)").Scan(&ready)
	if err != nil || !ready {
		return errors.New("mailing-list state cutover has not been verified")
	}
	return nil
}
func (s *Store) ApplySchema(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1949001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// All SQL identifiers below are fixed owned schema names, never input data.
type binding struct {
	table         string
	keys, columns []string
	args          []any
}

var families = []string{"groupsio-service", "groupsio-service-domain", "groupsio-subgroup", "groupsio-member", "groupsio-subgroup-gid", "groupsio-subgroup-project", "groupsio-artifact", "groupsio-message", "groupsio-member-lfid-invite-sent"}

func bind(key string) (binding, error) {
	family, id, ok := strings.Cut(key, ".")
	if !ok || id == "" || strings.ContainsAny(key, " \t\r\n*>/\\") {
		return binding{}, pkgerrors.MarkPermanent(errors.New("invalid mailing-list state key"))
	}
	switch family {
	case "groupsio-service", "groupsio-subgroup", "groupsio-artifact", "groupsio-message":
		return binding{"mailing_list_sync_state", []string{"resource_kind", "source_id"}, []string{"source_uid", "deleted"}, []any{family, id}}, nil
	case "groupsio-member":
		return binding{"mailing_list_member_principals", []string{"member_id"}, []string{"source_uid", "username", "mailing_list_id", "structured", "deleted"}, []any{id}}, nil
	case "groupsio-service-domain":
		return binding{"mailing_list_service_domains", []string{"service_id"}, []string{"domain_name", "deleted"}, []any{id}}, nil
	case "groupsio-subgroup-gid":
		return binding{"mailing_list_group_lookup", []string{"group_id"}, []string{"mailing_list_id", "deleted"}, []any{id}}, nil
	case "groupsio-subgroup-project":
		return binding{"mailing_list_project_refs", []string{"mailing_list_id"}, []string{"project_uid", "project_slug", "deleted"}, []any{id}}, nil
	case "groupsio-member-lfid-invite-sent":
		return binding{"mailing_list_invitation_guards", []string{"member_id"}, []string{"invite_id", "dispatch_pending", "deleted"}, []any{id}}, nil
	default:
		return binding{}, pkgerrors.MarkPermanent(errors.New("unowned mailing-list state family"))
	}
}
func (b binding) decode(value string) ([]any, error) {
	deleted := value == "!del"
	if deleted {
		value = ""
	}
	switch b.table {
	case "mailing_list_member_principals":
		parts := strings.SplitN(value, "|", 3)
		if len(parts) == 3 {
			return []any{parts[0], parts[1], parts[2], true, deleted}, nil
		}
		return []any{value, "", "", false, deleted}, nil
	case "mailing_list_project_refs":
		if deleted {
			return []any{"", "", true}, nil
		}
		uid, slug, ok := strings.Cut(value, "|")
		if !ok {
			return []any{value, "", false}, nil
		}
		return []any{uid, slug, false}, nil
	case "mailing_list_invitation_guards":
		pending := value == "pending"
		if pending {
			value = ""
		}
		return []any{value, pending, deleted}, nil
	default:
		return []any{value, deleted}, nil
	}
}
func where(b binding) string {
	parts := make([]string, len(b.keys))
	for i, k := range b.keys {
		parts[i] = fmt.Sprintf("%s=$%d", k, i+1)
	}
	return strings.Join(parts, " AND ")
}
func dbError(err error) error {
	if err == nil {
		return nil
	}
	return pkgerrors.MarkTransient(errors.New("mailing-list state database unavailable"))
}
func (s *Store) Lookup(ctx context.Context, key string) (port.MappingEntry, error) {
	b, err := bind(key)
	if err != nil {
		return port.MappingEntry{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	row := s.pool.QueryRow(ctx, "SELECT "+strings.Join(b.columns, ",")+" FROM "+b.table+" WHERE "+where(b), b.args...)
	var value string
	var deleted bool
	switch b.table {
	case "mailing_list_member_principals":
		var username, parent string
		var structured bool
		err = row.Scan(&value, &username, &parent, &structured, &deleted)
		if structured {
			value += "|" + username + "|" + parent
		}
	case "mailing_list_project_refs":
		var slug string
		err = row.Scan(&value, &slug, &deleted)
		value += "|" + slug
	case "mailing_list_invitation_guards":
		var pending bool
		err = row.Scan(&value, &pending, &deleted)
		if pending {
			value = "pending"
		}
	default:
		err = row.Scan(&value, &deleted)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return port.MappingEntry{State: port.MappingAbsent}, nil
	}
	if err != nil {
		return port.MappingEntry{}, dbError(err)
	}
	if deleted {
		return port.MappingEntry{State: port.MappingTombstoned}, nil
	}
	return port.MappingEntry{State: port.MappingPresent, Value: value}, nil
}
func (s *Store) PutMapping(ctx context.Context, key, value string) error {
	return s.write(ctx, key, value, false)
}
func (s *Store) CreateMapping(ctx context.Context, key, value string) error {
	return s.write(ctx, key, value, true)
}
func (s *Store) PutTombstone(ctx context.Context, key string) error {
	return s.PutMapping(ctx, key, "!del")
}
func (s *Store) write(ctx context.Context, key, value string, create bool) error {
	b, err := bind(key)
	if err != nil {
		return err
	}
	values, err := b.decode(value)
	if err != nil {
		return err
	}
	args := append(append([]any{}, b.args...), values...)
	cols := append(append([]string{}, b.keys...), b.columns...)
	params := make([]string, len(args))
	for i := range params {
		params[i] = fmt.Sprintf("$%d", i+1)
	}
	query := "INSERT INTO " + b.table + " (" + strings.Join(cols, ",") + ") VALUES (" + strings.Join(params, ",") + ") ON CONFLICT (" + strings.Join(b.keys, ",") + ") "
	if create {
		query += "DO NOTHING"
	} else {
		assign := make([]string, len(b.columns))
		for i, col := range b.columns {
			assign[i] = col + "=EXCLUDED." + col
		}
		query += "DO UPDATE SET " + strings.Join(assign, ",")
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return dbError(err)
	}
	if create && tag.RowsAffected() == 0 {
		return port.ErrMappingAlreadyExists
	}
	return nil
}
func (s *Store) PurgeMapping(ctx context.Context, key string) error {
	b, err := bind(key)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	_, err = s.pool.Exec(ctx, "DELETE FROM "+b.table+" WHERE "+where(b), b.args...)
	return dbError(err)
}
