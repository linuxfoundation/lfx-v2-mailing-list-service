// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Fixed errors preserve machine-readable recovery categories across wording changes.
var (
	// ErrImportBusy means another importer owns the migration lock.
	ErrImportBusy = errors.New("another state importer is active")
	// ErrAlreadyVerified prevents stale reimports after PG becomes authoritative.
	ErrAlreadyVerified = errors.New("verified state is already authoritative; refusing reimport")
	// ErrEmptySource rejects unproven empty cutovers.
	ErrEmptySource = errors.New("no owned state found; refusing an unproven empty cutover")
	// ErrInvalidSourceState requires manual source reconciliation.
	ErrInvalidSourceState = errors.New("invalid source state; manual reconciliation required")
	// ErrTargetConflict means an existing PG row differs from the source.
	ErrTargetConflict = errors.New("target differs from source; manual reconciliation required")
	// ErrTargetExtraRows means PG contains state not represented by the snapshot.
	ErrTargetExtraRows = errors.New("target contains unmatched rows; manual reconciliation required")
	// ErrSourceChanged means the fenced snapshot was not stable.
	ErrSourceChanged = errors.New("source changed; cutover not verified")
)

type stateSource interface {
	Info(context.Context) (*jetstream.StreamInfo, error)
	Next(context.Context, uint64, string) (*jetstream.RawStreamMsg, error)
}
type messageRequester interface {
	RequestWithContext(context.Context, string, []byte) (*nats.Msg, error)
}
type natsSource struct {
	stream    jetstream.Stream
	requester messageRequester
	apiPrefix string
}

func (s natsSource) Info(ctx context.Context) (*jetstream.StreamInfo, error) {
	return s.stream.Info(ctx)
}
func (s natsSource) Next(ctx context.Context, seq uint64, subject string) (*jetstream.RawStreamMsg, error) {
	// SDK GetMsg can use follower-served direct reads. Cutover needs the
	// leader-served API regardless of AllowDirect, without changing the stream.
	request, err := json.Marshal(struct {
		Sequence    uint64 `json:"seq"`
		NextSubject string `json:"next_by_subj"`
	}{seq, subject})
	if err != nil {
		return nil, err
	}
	apiSubject := strings.TrimSuffix(s.apiPrefix, ".") + ".STREAM.MSG.GET." + s.stream.CachedInfo().Config.Name
	response, err := s.requester.RequestWithContext(ctx, apiSubject, request)
	if err != nil {
		return nil, err
	}
	var result struct {
		Error   *jetstream.APIError `json:"error"`
		Message *struct {
			Subject  string    `json:"subject"`
			Sequence uint64    `json:"seq"`
			Data     []byte    `json:"data"`
			Headers  []byte    `json:"hdrs"`
			Time     time.Time `json:"time"`
		} `json:"message"`
	}
	if err := json.Unmarshal(response.Data, &result); err != nil {
		return nil, jetstream.ErrInvalidJetStreamResponse
	}
	if result.Error != nil {
		if result.Error.ErrorCode == jetstream.JSErrCodeMessageNotFound {
			return nil, jetstream.ErrMsgNotFound
		}
		return nil, result.Error
	}
	if result.Message == nil {
		return nil, jetstream.ErrInvalidJetStreamResponse
	}
	var headers nats.Header
	if len(result.Message.Headers) > 0 {
		headers, err = nats.DecodeHeadersMsg(result.Message.Headers)
		if err != nil {
			return nil, jetstream.ErrInvalidJetStreamResponse
		}
	}
	return &jetstream.RawStreamMsg{Subject: result.Message.Subject, Sequence: result.Message.Sequence, Data: result.Message.Data, Header: headers, Time: result.Message.Time}, nil
}

// Import is explicit offline migration, not a runtime fallback. All legacy
// mappings writers must remain externally fenced through deployment.
func (s *Store) Import(ctx context.Context, js jetstream.JetStream, apply bool) (int, error) {
	stream, err := js.Stream(ctx, "KV_v1-mappings")
	if err != nil {
		return 0, err
	}
	return s.importSource(ctx, natsSource{stream: stream, requester: js.Conn(), apiPrefix: js.Options().APIPrefix}, apply)
}
func (s *Store) importSource(ctx context.Context, source stateSource, apply bool) (int, error) {
	if apply {
		if err := s.ApplySchema(ctx); err != nil {
			return 0, err
		}
		conn, err := s.pool.Acquire(ctx)
		if err != nil {
			return 0, err
		}
		defer conn.Release()
		lockCtx, cancel := context.WithTimeout(ctx, opTimeout)
		var locked bool
		err = conn.QueryRow(lockCtx, "SELECT pg_try_advisory_lock(1949002)").Scan(&locked)
		cancel()
		if err != nil {
			return 0, err
		}
		if !locked {
			return 0, ErrImportBusy
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), opTimeout)
			defer cancel()
			_, _ = conn.Exec(cleanup, "SELECT pg_advisory_unlock(1949002)")
		}()
		checkCtx, cancel := context.WithTimeout(ctx, opTimeout)
		var ready bool
		err = conn.QueryRow(checkCtx, "SELECT EXISTS(SELECT 1 FROM mailing_list_state_cutover)").Scan(&ready)
		cancel()
		if err != nil {
			return 0, err
		}
		if ready {
			return 0, ErrAlreadyVerified
		}
	}
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	before, err := source.Info(opCtx)
	cancel()
	if err != nil {
		return 0, err
	}
	latest := map[string]string{}
	for _, family := range families {
		for seq := uint64(1); seq <= before.State.LastSeq; {
			opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			msg, err := source.Next(opCtx, seq, "$KV.v1-mappings."+family+".>")
			cancel()
			if errors.Is(err, jetstream.ErrMsgNotFound) {
				break
			}
			if err != nil {
				return 0, err
			}
			if msg.Sequence > before.State.LastSeq {
				break
			}
			seq = msg.Sequence + 1
			key := strings.TrimPrefix(msg.Subject, "$KV.v1-mappings.")
			if msg.Header.Get("KV-Operation") == "DEL" || msg.Header.Get("KV-Operation") == "PURGE" {
				delete(latest, key)
			} else {
				latest[key] = string(msg.Data)
			}
		}
	}
	if len(latest) == 0 {
		return 0, ErrEmptySource
	}
	counts := map[string]int{}
	for key, value := range latest {
		b, err := bind(key)
		if err != nil {
			return 0, ErrInvalidSourceState
		}
		if _, err = b.decode(value); err != nil {
			return 0, ErrInvalidSourceState
		}
		counts[b.table]++
	}
	if apply {
		for key, value := range latest {
			if err := s.CreateMapping(ctx, key, value); err != nil && !errors.Is(err, port.ErrMappingAlreadyExists) {
				return 0, err
			}
			entry, err := s.Lookup(ctx, key)
			if err != nil {
				return 0, err
			}
			b, _ := bind(key)
			want, _ := b.decode(value)
			gotValue := entry.Value
			if entry.Tombstoned() {
				gotValue = "!del"
			}
			got, err := b.decode(gotValue)
			if entry.State == port.MappingAbsent || err != nil || !reflect.DeepEqual(want, got) {
				return 0, ErrTargetConflict
			}
		}
		for _, table := range []string{"mailing_list_sync_state", "mailing_list_member_principals", "mailing_list_service_domains", "mailing_list_group_lookup", "mailing_list_project_refs", "mailing_list_invitation_guards"} {
			opCtx, cancel := context.WithTimeout(ctx, opTimeout)
			var n int
			err := s.pool.QueryRow(opCtx, "SELECT count(*) FROM "+table).Scan(&n)
			cancel()
			if err != nil {
				return 0, err
			}
			if n != counts[table] {
				return 0, ErrTargetExtraRows
			}
		}
	}
	opCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	after, err := source.Info(opCtx)
	cancel()
	if err != nil {
		return 0, err
	}
	if before.State.FirstSeq != after.State.FirstSeq || before.State.LastSeq != after.State.LastSeq || before.State.Msgs != after.State.Msgs {
		return 0, ErrSourceChanged
	}
	if apply {
		opCtx, cancel := context.WithTimeout(ctx, opTimeout)
		defer cancel()
		_, err := s.pool.Exec(opCtx, "INSERT INTO mailing_list_state_cutover(singleton,source_last_sequence) VALUES(true,$1)", after.State.LastSeq)
		if err != nil {
			return 0, err
		}
	}
	return len(latest), nil
}
