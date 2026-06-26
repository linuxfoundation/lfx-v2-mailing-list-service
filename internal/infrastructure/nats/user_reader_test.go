// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// stubRequester is a test double for the NATS Requester interface.
type stubRequester struct {
	msg *natsgo.Msg
	err error
}

func (s *stubRequester) RequestWithContext(_ context.Context, _ string, _ []byte) (*natsgo.Msg, error) {
	return s.msg, s.err
}

func newUserReader(msg *natsgo.Msg, err error) *NATSUserReader {
	sr := &stubRequester{msg: msg, err: err}
	return &NATSUserReader{nc: sr, logger: slog.New(slog.DiscardHandler)}
}

func TestUsernameByEmail_EmptyEmail_NotFound(t *testing.T) {
	r := newUserReader(nil, nil)
	_, err := r.UsernameByEmail(context.Background(), "   ")
	assert.ErrorIs(t, err, port.ErrUserNotFound)
}

func TestUsernameByEmail_RequestError_ReturnsError(t *testing.T) {
	r := newUserReader(nil, errors.New("nats timeout"))
	_, err := r.UsernameByEmail(context.Background(), "user@example.com")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, port.ErrUserNotFound)
}

func TestUsernameByEmail_EmptyBody_NotFound(t *testing.T) {
	r := newUserReader(&natsgo.Msg{Data: []byte("   ")}, nil)
	_, err := r.UsernameByEmail(context.Background(), "user@example.com")
	assert.ErrorIs(t, err, port.ErrUserNotFound)
}

func TestUsernameByEmail_BareString_ReturnsUsername(t *testing.T) {
	r := newUserReader(&natsgo.Msg{Data: []byte("jsmith")}, nil)
	username, err := r.UsernameByEmail(context.Background(), "jsmith@example.com")
	require.NoError(t, err)
	assert.Equal(t, "jsmith", username)
}

func TestUsernameByEmail_BareStringWithWhitespace_Trimmed(t *testing.T) {
	r := newUserReader(&natsgo.Msg{Data: []byte("  jsmith  ")}, nil)
	username, err := r.UsernameByEmail(context.Background(), "jsmith@example.com")
	require.NoError(t, err)
	assert.Equal(t, "jsmith", username)
}

func TestUsernameByEmail_JSONSuccessEnvelopeWithUsername(t *testing.T) {
	body := []byte(`{"success":true,"username":"alice"}`)
	r := newUserReader(&natsgo.Msg{Data: body}, nil)
	username, err := r.UsernameByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, "alice", username)
}

func TestUsernameByEmail_JSONFailureEnvelope_NotFoundError(t *testing.T) {
	body := []byte(`{"success":false,"error":"user not found"}`)
	r := newUserReader(&natsgo.Msg{Data: body}, nil)
	_, err := r.UsernameByEmail(context.Background(), "nobody@example.com")
	assert.ErrorIs(t, err, port.ErrUserNotFound)
}

func TestUsernameByEmail_JSONFailureEnvelope_NoUserError(t *testing.T) {
	body := []byte(`{"success":false,"error":"no user with that email"}`)
	r := newUserReader(&natsgo.Msg{Data: body}, nil)
	_, err := r.UsernameByEmail(context.Background(), "nobody@example.com")
	assert.ErrorIs(t, err, port.ErrUserNotFound)
}

func TestUsernameByEmail_JSONFailureEnvelope_OtherError(t *testing.T) {
	body := []byte(`{"success":false,"error":"internal server error"}`)
	r := newUserReader(&natsgo.Msg{Data: body}, nil)
	_, err := r.UsernameByEmail(context.Background(), "user@example.com")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, port.ErrUserNotFound)
}

func TestUsernameByEmail_JSONMissingSuccessField_ReturnsError(t *testing.T) {
	body := []byte(`{"username":"bob"}`)
	r := newUserReader(&natsgo.Msg{Data: body}, nil)
	_, err := r.UsernameByEmail(context.Background(), "bob@example.com")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, port.ErrUserNotFound)
}

func TestUsernameByEmail_InvalidJSON_ReturnsError(t *testing.T) {
	body := []byte(`{not valid json}`)
	r := newUserReader(&natsgo.Msg{Data: body}, nil)
	_, err := r.UsernameByEmail(context.Background(), "bob@example.com")
	assert.Error(t, err)
}
