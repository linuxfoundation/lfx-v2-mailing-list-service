// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
)

// setProjectMapping is a helper that writes the groupsio-subgroup-project mapping
// for the given mailingListUID into the fake mapping store.
func setProjectMapping(m *mock.FakeMappingStore, mailingListUID, projectUID, projectSlug string) {
	key := fmt.Sprintf("%s.%s", constants.KVMappingPrefixSubgroupProject, mailingListUID)
	m.Set(key, projectUID+"|"+projectSlug)
}

// --- HandleDataStreamMemberUpdate ---

func TestHandleDataStreamMemberUpdate_MissingGroupID_ACK(t *testing.T) {
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore(), nil)
	assert.False(t, nak, "missing group_id should ACK (malformed data, no retry)")
}

func TestHandleDataStreamMemberUpdate_ParentSubgroupAbsent_NAK(t *testing.T) {
	// group_id present but no subgroup mapping written yet
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{"group_id": float64(42)},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore(), nil)
	assert.True(t, nak, "absent subgroup mapping should NAK for retry")
}

func TestHandleDataStreamMemberUpdate_ProjectMappingAbsent_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	// project mapping deliberately absent

	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{"group_id": float64(42)},
		&mock.SpyMessagePublisher{}, m, nil)
	assert.True(t, nak, "absent project mapping should NAK for retry")
}

func TestHandleDataStreamMemberUpdate_Tombstoned_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")
	_ = m.PutTombstone(ctx, fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{"group_id": float64(42)},
		pub, m, nil)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "tombstoned member should not publish")
}

func TestHandleDataStreamMemberUpdate_HappyPath_ACKAndPublishesAndWritesMapping(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{
			"group_id":  float64(42),
			"member_id": float64(99),
			"email":     "alice@example.com",
			"full_name": "Alice Smith",
		},
		pub, m, nil)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMemberSubject, pub.IndexerCalls[0].Subject)
	assert.Empty(t, pub.AccessCalls, "member access is inherited — no access message expected")

	_, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember))
	assert.True(t, present, "forward mapping should be written after successful processing")
}

func TestHandleDataStreamMemberUpdate_ProjectFieldsPopulated(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid-123", "my-project-slug")

	pub := &mock.SpyMessagePublisher{}
	HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{
			"group_id": float64(42),
			"email":    "alice@example.com",
		},
		pub, m, nil)

	assert.Len(t, pub.IndexerCalls, 1)
	indexerMsg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	assert.True(t, ok, "indexer message should be *model.IndexerMessage")
	// Build marshals the member to JSON then stores it as map[string]any
	memberData, ok := indexerMsg.Data.(map[string]any)
	assert.True(t, ok, "indexer message data should be map[string]any")
	assert.Equal(t, "proj-uid-123", memberData["project_uid"])
	assert.Equal(t, "my-project-slug", memberData["project_slug"])
}

func TestHandleDataStreamMemberUpdate_CreateVsUpdate_Action(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")

	data := func() map[string]any { return map[string]any{"group_id": float64(42)} }
	ctx := context.Background()
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)

	assert.Equal(t, model.ActionCreated, m.ResolveAction(ctx, mKey))
	HandleDataStreamMemberUpdate(ctx, "mem-1", data(), &mock.SpyMessagePublisher{}, m, nil)
	assert.Equal(t, model.ActionUpdated, m.ResolveAction(ctx, mKey))
}

// --- HandleDataStreamMemberDelete ---

func TestHandleDataStreamMemberDelete_DuplicateDelete_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	_ = m.PutTombstone(ctx, fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberDelete(ctx, "mem-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should not publish")
}

func TestHandleDataStreamMemberDelete_NeverIndexed_TombstonesWithoutPublishing(t *testing.T) {
	m := mock.NewFakeMappingStore()
	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberDelete(context.Background(), "mem-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "never-indexed member should not publish indexer message")
	assert.True(t, m.IsTombstoned(context.Background(),
		fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)),
		"should still tombstone to prevent future re-processing")
}

func TestHandleDataStreamMemberDelete_HappyPath_ACKAndTombstones(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	_ = m.PutMapping(ctx, mKey, "mem-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberDelete(ctx, "mem-1", pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMemberSubject, pub.IndexerCalls[0].Subject)
	assert.Empty(t, pub.AccessCalls, "member delete should not publish access message when no username in mapping")

	assert.True(t, m.IsTombstoned(ctx, mKey))
}

func TestHandleDataStreamMemberUpdate_WithUsername_PublishesMemberPut(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{
			"group_id":  float64(42),
			"username":  "alice@example.com",
			"full_name": "Alice Smith",
		},
		pub, m, nil)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, fgaconstants.GenericMemberPutSubject, pub.AccessCalls[0].Subject)

	msg, ok := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	assert.Equal(t, constants.ObjectTypeGroupsIOMailingList, msg.ObjectType)
	assert.Equal(t, "member_put", msg.Operation)

	data, ok := msg.Data.(fgatypes.GenericMemberData)
	assert.True(t, ok)
	assert.Equal(t, "sg-1", data.UID)
	assert.Equal(t, "alice@example.com", data.Username)
	assert.Equal(t, []string{constants.RelationMember}, data.Relations)
}

func TestHandleDataStreamMemberUpdate_WithLFXHandleUsername_PublishesMemberPut(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{
			"group_id":  float64(42),
			"username":  "alice.smith",
			"full_name": "Alice Smith",
		},
		pub, m, nil)

	assert.False(t, nak)
	assert.Len(t, pub.AccessCalls, 1)

	msg, ok := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	data, ok := msg.Data.(fgatypes.GenericMemberData)
	assert.True(t, ok)
	assert.Equal(t, "alice.smith", data.Username)
}

func TestHandleDataStreamMemberUpdate_UsernameCleared_PublishesMemberRemove(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")
	// Pre-existing mapping with username stored
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	_ = m.PutMapping(ctx, mKey, "mem-1|alice@example.com|sg-1")

	pub := &mock.SpyMessagePublisher{}
	// Update arrives with username removed
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{
			"group_id":  float64(42),
			"full_name": "Alice Smith",
			// username intentionally absent
		},
		pub, m, nil)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	// Should have exactly one access call: member_remove for the old username
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, fgaconstants.GenericMemberRemoveSubject, pub.AccessCalls[0].Subject)

	msg, ok := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	assert.Equal(t, "member_remove", msg.Operation)
	data, ok := msg.Data.(fgatypes.GenericMemberData)
	assert.True(t, ok)
	assert.Equal(t, "sg-1", data.UID)
	assert.Equal(t, "alice@example.com", data.Username)
	assert.Empty(t, data.Relations)
}

func TestHandleDataStreamMemberUpdate_UsernameChanged_RemovesOldAndAddsNew(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	_ = m.PutMapping(ctx, mKey, "mem-1|alice@example.com|sg-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{
			"group_id":  float64(42),
			"username":  "bob@example.com",
			"full_name": "Bob Jones",
		},
		pub, m, nil)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	// remove for old username, then put for new username
	assert.Len(t, pub.AccessCalls, 2)
	assert.Equal(t, fgaconstants.GenericMemberRemoveSubject, pub.AccessCalls[0].Subject)
	removeData, ok := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	assert.Equal(t, "alice@example.com", removeData.Data.(fgatypes.GenericMemberData).Username)

	assert.Equal(t, fgaconstants.GenericMemberPutSubject, pub.AccessCalls[1].Subject)
	putData, ok := pub.AccessCalls[1].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	assert.Equal(t, "bob@example.com", putData.Data.(fgatypes.GenericMemberData).Username)
}

func TestHandleDataStreamMemberUpdate_MappingReadError_TreatedAsCreate(t *testing.T) {
	// When GetMappingValue returns false (including on transient KV error), the handler cannot
	// distinguish "not found" from "error" — it treats the event as ActionCreated and skips
	// any stale-tuple removal. This is an accepted limitation pending an interface change that
	// exposes the raw error.
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	_ = m.PutMapping(ctx, mKey, "mem-1|alice@example.com|sg-1")
	m.SimulateGetError(mKey)

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{"group_id": float64(42), "username": "bob@example.com"},
		pub, m, nil)

	// ACKed — no NAK since we can't distinguish transient from not-found without interface change.
	assert.False(t, nak)
	// No remove: oldUsername is unknown, so we cannot revoke the stale tuple.
	assert.Equal(t, 1, len(pub.AccessCalls), "only member_put should be published — no remove since old username is unknown")
	assert.Equal(t, fgaconstants.GenericMemberPutSubject, pub.AccessCalls[0].Subject)
}

func TestHandleDataStreamMemberUpdate_MemberPutFailure_Transient_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")

	pub := &mock.SpyMessagePublisher{AccessError: errors.New("connection refused")}
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{"group_id": float64(42), "username": "alice@example.com"},
		pub, m, nil)

	assert.True(t, nak, "transient member_put failure should NAK for retry")
	// Mapping must NOT have been written — redelivery needs to retry member_put too.
	_, mappingWritten := m.GetMappingValue(ctx, fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember))
	assert.False(t, mappingWritten, "mapping must not be written when member_put fails")
}

func TestHandleDataStreamMemberUpdate_PutMappingFailure_Transient_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	setProjectMapping(m, "sg-1", "proj-uid", "my-project")
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	m.SimulatePutError(mKey, errors.New("connection timeout"))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{"group_id": float64(42), "username": "alice@example.com"},
		pub, m, nil)

	assert.True(t, nak, "transient PutMapping failure should NAK for retry")
	// FGA put was already published — redelivery will resend it, which is idempotent.
	assert.Len(t, pub.AccessCalls, 1, "member_put was published before the mapping write failed")
}

func TestHandleDataStreamMemberUpdate_MailingListChanged_RemovesOldTuple(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	// Member was previously in sg-old, now arriving under sg-new
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-new")
	setProjectMapping(m, "sg-new", "proj-uid", "my-project")
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	_ = m.PutMapping(ctx, mKey, "mem-1|alice@example.com|sg-old")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{"group_id": float64(42), "username": "alice@example.com"},
		pub, m, nil)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	// remove for old (sg-old, alice), then put for new (sg-new, alice)
	assert.Len(t, pub.AccessCalls, 2)
	assert.Equal(t, fgaconstants.GenericMemberRemoveSubject, pub.AccessCalls[0].Subject)
	removeData, ok := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	assert.Equal(t, "sg-old", removeData.Data.(fgatypes.GenericMemberData).UID)
	assert.Equal(t, "alice@example.com", removeData.Data.(fgatypes.GenericMemberData).Username)

	assert.Equal(t, fgaconstants.GenericMemberPutSubject, pub.AccessCalls[1].Subject)
	putData, ok := pub.AccessCalls[1].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	assert.Equal(t, "sg-new", putData.Data.(fgatypes.GenericMemberData).UID)
}

func TestHandleDataStreamMemberDelete_WithUsername_PublishesMemberRemove(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	// Store mapping in uid|username|mailingListUID format
	_ = m.PutMapping(ctx, mKey, "mem-1|alice@example.com|sg-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberDelete(ctx, "mem-1", pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, fgaconstants.GenericMemberRemoveSubject, pub.AccessCalls[0].Subject)

	msg, ok := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage)
	assert.True(t, ok)
	assert.Equal(t, constants.ObjectTypeGroupsIOMailingList, msg.ObjectType)
	assert.Equal(t, "member_remove", msg.Operation)

	data, ok := msg.Data.(fgatypes.GenericMemberData)
	assert.True(t, ok)
	assert.Equal(t, "sg-1", data.UID)
	assert.Equal(t, "alice@example.com", data.Username)
	assert.Empty(t, data.Relations)

	assert.True(t, m.IsTombstoned(ctx, mKey))
}
