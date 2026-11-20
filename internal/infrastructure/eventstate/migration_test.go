// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type metadataStream struct {
	jetstream.Stream
	info jetstream.StreamInfo
}

func (s *metadataStream) CachedInfo() *jetstream.StreamInfo { return &s.info }

type fakeMessageRequester struct {
	request func(context.Context, string, []byte) (*nats.Msg, error)
}

func (r fakeMessageRequester) RequestWithContext(ctx context.Context, subject string, data []byte) (*nats.Msg, error) {
	return r.request(ctx, subject, data)
}

func TestNatsSourceNext(t *testing.T) {
	stream := &metadataStream{info: jetstream.StreamInfo{Config: jetstream.StreamConfig{Name: "KV_v1-mappings", AllowDirect: true}}}
	source := natsSource{stream: stream, apiPrefix: "$JS.test.API.", requester: fakeMessageRequester{request: func(_ context.Context, subject string, data []byte) (*nats.Msg, error) {
		require.Equal(t, "$JS.test.API.STREAM.MSG.GET.KV_v1-mappings", subject)
		require.JSONEq(t, `{"seq":7,"next_by_subj":"$KV.v1-mappings.groupsio-service.>"}`, string(data))
		body, err := json.Marshal(map[string]any{"message": map[string]any{"subject": "$KV.v1-mappings.groupsio-service.test", "seq": 19, "data": []byte("!del"), "hdrs": []byte("NATS/1.0\r\nKV-Operation: DEL\r\n\r\n")}})
		require.NoError(t, err)
		return &nats.Msg{Data: body}, nil
	}}}
	msg, err := source.Next(context.Background(), 7, "$KV.v1-mappings.groupsio-service.>")
	require.NoError(t, err)
	require.Equal(t, uint64(19), msg.Sequence)
	require.Equal(t, "DEL", msg.Header.Get("KV-Operation"))
	require.True(t, stream.info.Config.AllowDirect)
}

func TestLegacyProjectReferenceImport(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	source := &fakeSource{messages: []*jetstream.RawStreamMsg{
		{Sequence: 1, Subject: "$KV.v1-mappings.groupsio-subgroup-project.legacy", Data: []byte("project-uid")},
		{Sequence: 4, Subject: "$KV.v1-mappings.project.sfid.unrelated", Data: []byte("unrelated")},
	}}
	n, err := s.importSource(ctx, source, false)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Error(t, s.Ready(ctx))
	n, err = s.importSource(ctx, source, true)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	entry, err := s.Lookup(ctx, "groupsio-subgroup-project.legacy")
	require.NoError(t, err)
	require.Equal(t, "project-uid|", entry.Value)
	require.NoError(t, s.Ready(ctx))
}
