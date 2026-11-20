// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("MAILING_LIST_STATE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MAILING_LIST_STATE_TEST_DATABASE_URL to a disposable local Postgres database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := fmt.Sprintf("mailing_state_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+name)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = s.Close()
		_, err := admin.Exec(ctx, "DROP SCHEMA "+name+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	require.NoError(t, s.ApplySchema(ctx))
	require.NoError(t, s.ApplySchema(ctx))
	return s
}

// Fixtures deliberately do not derive from the importer's family list.
var fixtures = map[string]string{
	"groupsio-service.service":                "service",
	"groupsio-service-domain.service":         "example.com",
	"groupsio-subgroup.list":                  "list",
	"groupsio-member.member":                  "member|test.user|list",
	"groupsio-subgroup-gid.123":               "list",
	"groupsio-subgroup-project.list":          "project|test-project",
	"groupsio-artifact.artifact":              "artifact",
	"groupsio-message.message":                "message",
	"groupsio-member-lfid-invite-sent.member": "pending",
}

func TestCodec(t *testing.T) {
	for _, key := range []string{"project.sfid.test", "committee.sfid.test", "groupsio-subgroup-service.service.list", "groupsio-member.bad>key"} {
		t.Run(key, func(t *testing.T) { _, err := bind(key); require.Error(t, err) })
	}
	t.Run("principal fields are decomposed", func(t *testing.T) {
		b, err := bind("groupsio-member.member")
		require.NoError(t, err)
		v, err := b.decode("member|test.user|list")
		require.NoError(t, err)
		require.Equal(t, []any{"member", "test.user", "list", true, false}, v)
	})
	t.Run("no database means no fallback", func(t *testing.T) { _, err := Open(context.Background(), ""); require.Error(t, err) })
}

func TestMigrationFailureCategory(t *testing.T) {
	for err, want := range map[error]string{
		ErrAlreadyVerified:                   "already_verified",
		ErrSourceChanged:                     "source_changed",
		ErrTargetConflict:                    "target_conflict",
		ErrTargetExtraRows:                   "target_extra_rows",
		ErrImportBusy:                        "import_busy",
		ErrEmptySource:                       "empty_source",
		ErrInvalidSourceState:                "invalid_source_state",
		errors.New("private-driver-details"): "infrastructure_failure",
	} {
		t.Run(want, func(t *testing.T) {
			require.Equal(t, want, MigrationFailureCategory(fmt.Errorf("arbitrary changed wording: %w", err)))
		})
	}
}
func TestPostgresState(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	require.Error(t, s.Ready(ctx))
	for key, value := range fixtures {
		t.Run(key, func(t *testing.T) {
			require.NoError(t, s.CreateMapping(ctx, key, value))
			require.ErrorIs(t, s.CreateMapping(ctx, key, value), port.ErrMappingAlreadyExists)
			e, err := s.Lookup(ctx, key)
			require.NoError(t, err)
			require.True(t, e.Present())
			require.Equal(t, value, e.Value)
			require.NoError(t, s.PutTombstone(ctx, key))
			e, err = s.Lookup(ctx, key)
			require.NoError(t, err)
			require.True(t, e.Tombstoned())
			require.ErrorIs(t, s.CreateMapping(ctx, key, value), port.ErrMappingAlreadyExists)
			require.NoError(t, s.PutMapping(ctx, key, value))
			e, err = s.Lookup(ctx, key)
			require.NoError(t, err)
			require.True(t, e.Present())
			require.NoError(t, s.PurgeMapping(ctx, key))
			e, err = s.Lookup(ctx, key)
			require.NoError(t, err)
			require.Equal(t, port.MappingAbsent, e.State)
		})
	}
	t.Run("empty present marker is distinct from absent", func(t *testing.T) {
		require.NoError(t, s.PutMapping(ctx, "groupsio-service.empty", ""))
		e, err := s.Lookup(ctx, "groupsio-service.empty")
		require.NoError(t, err)
		require.True(t, e.Present())
		require.Empty(t, e.Value)
	})
	t.Run("only one concurrent invitation claim succeeds", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 20)
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- s.CreateMapping(ctx, "groupsio-member-lfid-invite-sent.concurrent", "pending")
			}()
		}
		wg.Wait()
		close(results)
		wins := 0
		for err := range results {
			if err == nil {
				wins++
			} else {
				require.ErrorIs(t, err, port.ErrMappingAlreadyExists)
			}
		}
		require.Equal(t, 1, wins)
	})
}

type fakeSource struct {
	messages []*jetstream.RawStreamMsg
	changing bool
	infos    int
}

func (s *fakeSource) Info(context.Context) (*jetstream.StreamInfo, error) {
	s.infos++
	last := s.messages[len(s.messages)-1].Sequence
	if s.changing && s.infos > 1 {
		last++
	}
	return &jetstream.StreamInfo{State: jetstream.StreamState{FirstSeq: 1, LastSeq: last, Msgs: uint64(len(s.messages))}}, nil
}
func (s *fakeSource) Next(_ context.Context, seq uint64, subject string) (*jetstream.RawStreamMsg, error) {
	for _, m := range s.messages {
		if m.Sequence >= seq && strings.HasPrefix(m.Subject, strings.TrimSuffix(subject, ">")) {
			return m, nil
		}
	}
	return nil, jetstream.ErrMsgNotFound
}
func TestImport(t *testing.T) {
	for _, mode := range []string{"stable", "changed", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			source := &fakeSource{changing: mode == "changed"}
			seq := uint64(0)
			add := func(key, value, op string) {
				seq += 3
				source.messages = append(source.messages, &jetstream.RawStreamMsg{Sequence: seq, Subject: "$KV.v1-mappings." + key, Data: []byte(value), Header: nats.Header{"KV-Operation": []string{op}}})
			}
			for key, value := range fixtures {
				add("project.sfid.unrelated", "project", "")
				add(key, "old", "")
				add(key, value, "")
			}
			for _, op := range []string{"DEL", "PURGE"} {
				add("groupsio-service.deleted-"+op, "old", "")
				add("groupsio-service.deleted-"+op, "", op)
				add("groupsio-service.restored-"+op, "old", "")
				add("groupsio-service.restored-"+op, "", op)
				add("groupsio-service.restored-"+op, "restored", "")
			}
			add("groupsio-service.tombstone", "!del", "")
			add("groupsio-subgroup-service.service.list", "list", "")
			if mode == "conflict" {
				require.NoError(t, s.PutMapping(ctx, "groupsio-service.service", "conflict"))
			}
			n, err := s.importSource(ctx, source, true)
			if mode != "stable" {
				require.Error(t, err)
				if mode == "changed" {
					require.ErrorIs(t, err, ErrSourceChanged)
				} else {
					require.ErrorIs(t, err, ErrTargetConflict)
				}
				require.Error(t, s.Ready(ctx))
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(fixtures)+3, n)
			for key, value := range fixtures {
				e, err := s.Lookup(ctx, key)
				require.NoError(t, err)
				require.True(t, e.Present())
				require.Equal(t, value, e.Value)
			}
			for _, op := range []string{"DEL", "PURGE"} {
				e, err := s.Lookup(ctx, "groupsio-service.deleted-"+op)
				require.NoError(t, err)
				require.Equal(t, port.MappingAbsent, e.State)
				e, err = s.Lookup(ctx, "groupsio-service.restored-"+op)
				require.NoError(t, err)
				require.Equal(t, "restored", e.Value)
			}
			require.NoError(t, s.Ready(ctx))
			_, err = s.importSource(ctx, source, true)
			require.Error(t, err)
		})
	}
}
