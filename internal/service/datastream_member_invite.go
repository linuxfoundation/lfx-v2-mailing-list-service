// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	msgpack "github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
	"github.com/nats-io/nats.go/jetstream"
)

// kvPrefixSubgroupV1 is the v1-objects key prefix for GroupsIO subgroup records.
// Used by MemberInviteHandler to resolve the mailing-list display name.
const kvPrefixSubgroupV1 = "itx-groupsio-v2-subgroup."

// memberInviteSentKeyFmt is the format for the v1-mappings dedup key.
const memberInviteSentKeyFmt = "%s.%s"

func memberInviteSentKey(memberUID string) string {
	return fmt.Sprintf(memberInviteSentKeyFmt, constants.KVMemberLFIDInviteSentPrefix, memberUID)
}

// MemberInviteHandler performs best-effort LFID invite sending for new mailing-list
// members who do not yet have an LFID (username is empty but email is present).
type MemberInviteHandler struct {
	inviteSender     port.InviteSender
	userReader       port.UserReader
	mappings         port.MappingReaderWriter
	v1ObjectsKV      jetstream.KeyValue
	selfServeBaseURL string
}

// NewMemberInviteHandler creates a MemberInviteHandler. Returns nil when any
// dependency is missing — callers must nil-check before using.
func NewMemberInviteHandler(
	inviteSender port.InviteSender,
	userReader port.UserReader,
	mappings port.MappingReaderWriter,
	v1ObjectsKV jetstream.KeyValue,
	selfServeBaseURL string,
) *MemberInviteHandler {
	if inviteSender == nil || userReader == nil || mappings == nil || v1ObjectsKV == nil || strings.TrimSpace(selfServeBaseURL) == "" {
		return nil
	}
	return &MemberInviteHandler{
		inviteSender:     inviteSender,
		userReader:       userReader,
		mappings:         mappings,
		v1ObjectsKV:      v1ObjectsKV,
		selfServeBaseURL: selfServeBaseURL,
	}
}

// MaybeSendInvite performs a best-effort LFID invite for a new mailing-list member
// who has no username. All errors are logged and swallowed so they never block the
// data-stream event handler.
func (h *MemberInviteHandler) MaybeSendInvite(
	ctx context.Context,
	logger *slog.Logger,
	member *model.GrpsIOMember,
) {
	if h == nil {
		return
	}

	email := strings.TrimSpace(member.Email)
	if email == "" {
		return
	}

	// Dedup: atomically claim the invite slot via CreateMapping. Because Create only
	// succeeds when the key is absent, concurrent redeliveries or replicas that race
	// here will all fail on the same atomic KV operation and skip gracefully — no
	// separate Get+Put window to exploit.
	inviteSentKey := memberInviteSentKey(member.UID)
	if err := h.mappings.CreateMapping(ctx, inviteSentKey, "pending"); err != nil {
		if errors.Is(err, port.ErrMappingAlreadyExists) {
			logger.DebugContext(ctx, "LFID invite already sent or in-flight for mailing-list member, skipping",
				"member_uid", member.UID)
		} else {
			logger.WarnContext(ctx, "failed to claim invite dedup slot; skipping to avoid duplicate",
				"member_uid", member.UID, "error", err)
		}
		return
	}

	// Check whether the participant already has an LFID. Transient auth-service errors
	// fall through and still attempt the invite — skipping here would permanently lose the
	// invite opportunity because the KV mapping is already stored (this message won't be
	// redelivered as ActionCreated again). The invite service handles the edge case where
	// the user already has an LFID.
	username, err := h.userReader.UsernameByEmail(ctx, email)
	if err == nil && username != "" {
		logger.DebugContext(ctx, "mailing-list member already has LFID, skipping invite",
			"member_uid", member.UID)
		return
	}
	if err != nil && !errors.Is(err, port.ErrUserNotFound) {
		logger.WarnContext(ctx, "failed to check LFID for mailing-list member; proceeding with invite as best-effort",
			"member_uid", member.UID, "error", err)
	}

	// Resolve the mailing-list display name for the invite email.
	listName, ok := h.mailingListName(ctx, member.MailingListUID)
	if !ok {
		logger.WarnContext(ctx, "could not resolve mailing-list name; skipping invite to avoid confusing email",
			"member_uid", member.UID, "mailing_list_uid", member.MailingListUID)
		return
	}

	returnURL := fmt.Sprintf("%s/mailing-lists/%s",
		strings.TrimRight(h.selfServeBaseURL, "/"),
		url.PathEscape(member.MailingListUID))

	displayName := strings.TrimSpace(member.FirstName + " " + member.LastName)
	req := inviteapi.SendInviteRequest{
		Recipient: &inviteapi.Recipient{
			Email: email,
			Name:  displayName,
		},
		Resource: &inviteapi.Resource{
			UID:  member.MailingListUID,
			Name: listName,
			Type: constants.ResourceTypeMailingList,
		},
		Role:           constants.InviteRoleMember,
		ReturnURL:      returnURL,
		ExpirationDays: 30,
	}

	result, sendErr := h.inviteSender.SendInvite(ctx, req)
	if sendErr != nil {
		logger.WarnContext(ctx, "failed to send LFID invite for mailing-list member; continuing",
			"member_uid", member.UID, "error", sendErr)
		return
	}

	if err := h.mappings.PutMapping(ctx, inviteSentKey, result.InviteUID); err != nil {
		logger.WarnContext(ctx, "failed to update mailing-list member LFID invite sent marker",
			"member_uid", member.UID, "error", err)
	}
	logger.InfoContext(ctx, "sent LFID invite for mailing-list member",
		"member_uid", member.UID,
		"invite_uid", result.InviteUID,
		"expires_at", result.ExpiresAt,
	)
}

// mailingListName looks up the mailing-list's display name from the v1-objects KV bucket.
// It reads the subgroup record keyed by "<kvPrefixSubgroupV1><mailingListUID>" and
// returns the group_name field (falling back to title). Returns ("", false) when the
// name cannot be resolved.
func (h *MemberInviteHandler) mailingListName(ctx context.Context, mailingListUID string) (string, bool) {
	kvKey := kvPrefixSubgroupV1 + mailingListUID
	entry, err := h.v1ObjectsKV.Get(ctx, kvKey)
	if err != nil {
		return "", false
	}

	data, decErr := decodeMapData(entry.Value())
	if decErr != nil {
		return "", false
	}

	if name := strings.TrimSpace(mapconv.StringVal(data, "group_name")); name != "" {
		return name, true
	}
	if title := strings.TrimSpace(mapconv.StringVal(data, "title")); title != "" {
		return title, true
	}
	return "", false
}

// decodeMapData unmarshals KV entry bytes as JSON first, then msgpack.
// Mirrors the decode logic used by the data-stream consumer.
func decodeMapData(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err == nil {
		return result, nil
	}
	if err := msgpack.Unmarshal(data, &result); err == nil {
		return result, nil
	}
	return nil, fmt.Errorf("failed to decode KV data as JSON or msgpack")
}

// ShouldSendMemberInvite reports whether a new member event without an LFID should
// trigger an invite.
func ShouldSendMemberInvite(action model.MessageAction, username, email string) bool {
	return action == model.ActionCreated &&
		strings.TrimSpace(username) == "" &&
		strings.TrimSpace(email) != ""
}
