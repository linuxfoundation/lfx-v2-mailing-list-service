// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // required to match Groups.io's own webhook signing scheme, not a choice made here
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// groupsIOMembershipActions are the Groups.io subgroup webhook "action" values that represent a
// membership change relevant to auto sync. Other actions (created_subgroup, deleted_subgroup,
// ban_members, ...) are acknowledged (200/204) but otherwise ignored: this feature's scope is
// subscribe/unsubscribe, not subgroup lifecycle or moderation actions.
var groupsIOMembershipActions = map[string]bool{
	constants.SubGroupMemberAddedEvent:   true,
	constants.SubGroupMemberRemovedEvent: true,
}

// ReceiveGroupsioWebhook verifies the Groups.io webhook signature, then — for a subgroup
// member-added/removed event on a mailing list configured with sync_mode auto_ml_to_c — triggers
// an immediate committee membership reconciliation (issue #4023).
//
// A bad/missing signature or malformed payload is rejected with BadRequest so Groups.io's
// delivery logs surface the problem. Once the payload is verified and parsed, processing is
// best-effort: a failure reconciling auto sync is logged, not returned as an error, so Groups.io
// does not retry-storm an event whose webhook delivery itself succeeded.
func (s *mailingListAPI) ReceiveGroupsioWebhook(ctx context.Context, p *mailinglist.ReceiveGroupsioWebhookPayload) error {
	if err := verifyGroupsIOWebhookSignature(p.Body, p.Signature, s.webhookSecret); err != nil {
		return &mailinglist.BadRequestError{Message: err.Error()}
	}

	var evt model.GrpsIOWebhookEvent
	if err := json.Unmarshal(p.Body, &evt); err != nil {
		return &mailinglist.BadRequestError{Message: "malformed webhook payload: " + err.Error()}
	}

	if err := processGroupsIOWebhookEvent(ctx, evt, s.mailingListReader, s.realtimeAutoSyncer, slog.Default()); err != nil {
		slog.ErrorContext(ctx, "groupsio webhook auto-sync failed; best-effort, not retrying",
			"error", err,
			"action", evt.Action,
		)
	}
	return nil
}

// verifyGroupsIOWebhookSignature checks body's signature against secret using the same scheme
// Groups.io itself signs webhook deliveries with: HMAC-SHA1 of the raw body, base64-encoded (see
// itx-service-groupsio's verifyGroupsioRequest, the existing production verifier for this same
// webhook source) — not a scheme chosen by this service, so it must match exactly. An empty
// secret or signature is always rejected: webhook ingestion must not silently accept
// unsigned/unverifiable requests just because configuration is incomplete.
func verifyGroupsIOWebhookSignature(body []byte, signature *string, secret string) error {
	if secret == "" {
		return fmt.Errorf("webhook signature verification is not configured")
	}
	if signature == nil || *signature == "" {
		return fmt.Errorf("missing webhook signature header")
	}

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(*signature)) {
		return fmt.Errorf("webhook signature mismatch")
	}
	return nil
}

// processGroupsIOWebhookEvent resolves a membership-change webhook event to the mailing list it
// applies to and, if that mailing list is configured with sync_mode auto_ml_to_c, triggers an
// auto-sync reconciliation. Any other action (or an event with no member info) is a no-op.
//
// The mailing list ID is derived directly from the webhook's numeric group ID: ITX's own mailing
// list UID is that same ID, stringified (see proxy.fromWireSubgroup), so no separate lookup is
// needed to go from "which Groups.io subgroup fired this webhook" to "which mailing list is
// that in this service's API."
func processGroupsIOWebhookEvent(
	ctx context.Context,
	evt model.GrpsIOWebhookEvent,
	mailingListReader port.GroupsIOMailingListReader,
	autoSyncer port.RealtimeAutoSyncer,
	logger *slog.Logger,
) error {
	if !groupsIOMembershipActions[evt.Action] {
		logger.Debug("groupsio webhook action is not a membership change; ignoring", "action", evt.Action)
		return nil
	}
	if evt.MemberInfo == nil || evt.MemberInfo.GroupID == 0 {
		logger.Warn("groupsio webhook membership event missing group id; discarding", "action", evt.Action)
		return nil
	}

	mailingListID := strconv.FormatUint(evt.MemberInfo.GroupID, 10)

	ml, err := mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return err
	}
	if len(ml.Committees) == 0 || ml.Committees[0].SyncMode != model.SyncModeAutoMailingListToCommittee {
		logger.Debug("mailing list is not configured for auto_ml_to_c sync; ignoring webhook event",
			"mailing_list_id", mailingListID)
		return nil
	}

	if err := autoSyncer.TriggerAutoSync(ctx, mailingListID); err != nil {
		return err
	}
	logger.Info("groupsio webhook event triggered auto-sync", "mailing_list_id", mailingListID, "action", evt.Action)
	return nil
}
