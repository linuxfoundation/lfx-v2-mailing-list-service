// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
)

type fakeRequester struct {
	reply []byte
	err   error
}

func (f *fakeRequester) RequestWithContext(_ context.Context, _ string, _ []byte) (*nats.Msg, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &nats.Msg{Data: f.reply}, nil
}

func TestNATSAccessChecker_HasRelation(t *testing.T) {
	tests := []struct {
		name      string
		user      string
		relation  string
		object    string
		reply     []byte
		wantAllow bool
		wantErr   bool
	}{
		{
			name:      "true result",
			user:      "user:alice",
			relation:  "writer",
			object:    "groupsio_mailing_list:ml-1",
			reply:     []byte("groupsio_mailing_list:ml-1#writer@user:alice\ttrue\n"),
			wantAllow: true,
		},
		{
			name:      "false result",
			user:      "user:alice",
			relation:  "writer",
			object:    "groupsio_mailing_list:ml-1",
			reply:     []byte("groupsio_mailing_list:ml-1#writer@user:alice\tfalse\n"),
			wantAllow: false,
		},
		{
			name:     "missing tab separator",
			user:     "user:alice",
			relation: "writer",
			object:   "groupsio_mailing_list:ml-1",
			reply:    []byte("groupsio_mailing_list:ml-1#writer@user:alice"),
			wantErr:  true,
		},
		{
			name:     "unrecognized result value",
			user:     "user:alice",
			relation: "writer",
			object:   "groupsio_mailing_list:ml-1",
			reply:    []byte("groupsio_mailing_list:ml-1#writer@user:alice\tmaybe\n"),
			wantErr:  true,
		},
		{
			name:     "user contains delimiter",
			user:     "user:ali#ce",
			relation: "writer",
			object:   "groupsio_mailing_list:ml-1",
			wantErr:  true,
		},
		{
			name:      "client-credentials user with @ is allowed",
			user:      "user:abc123@clients",
			relation:  "writer",
			object:    "groupsio_mailing_list:ml-1",
			reply:     []byte("groupsio_mailing_list:ml-1#writer@user:abc123@clients\ttrue\n"),
			wantAllow: true,
		},
		{
			name:     "relation contains delimiter",
			user:     "user:alice",
			relation: "wri@ter",
			object:   "groupsio_mailing_list:ml-1",
			wantErr:  true,
		},
		{
			name:     "object contains tab",
			user:     "user:alice",
			relation: "writer",
			object:   "groupsio_mailing_list:ml\t1",
			wantErr:  true,
		},
		{
			name:     "object contains newline",
			user:     "user:alice",
			relation: "writer",
			object:   "groupsio_mailing_list:ml\n1",
			wantErr:  true,
		},
		{
			name:     "empty object",
			user:     "user:alice",
			relation: "writer",
			object:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewAccessChecker(&fakeRequester{reply: tt.reply}, nil)
			allow, err := checker.HasRelation(context.Background(), tt.user, tt.relation, tt.object)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (allow=%v)", allow)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if allow != tt.wantAllow {
				t.Fatalf("got allow=%v, want %v", allow, tt.wantAllow)
			}
		})
	}
}

func TestNATSAccessChecker_HasRelation_RequestError(t *testing.T) {
	checker := NewAccessChecker(&fakeRequester{err: context.DeadlineExceeded}, nil)
	_, err := checker.HasRelation(context.Background(), "user:alice", "writer", "groupsio_mailing_list:ml-1")
	if err == nil {
		t.Fatal("expected error from underlying request failure, got nil")
	}
}
