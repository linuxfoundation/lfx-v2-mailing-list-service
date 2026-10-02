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
	"github.com/stretchr/testify/require"
)

func TestHandleDataStreamSubgroupUpdate_MissingProjectID_ACK(t *testing.T) {
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore(), mock.NewFakeProjectLookup())
	assert.False(t, nak, "missing project_id should ACK")
}

func TestHandleDataStreamSubgroupUpdate_ProjectMappingAbsent_NAK(t *testing.T) {
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{"project_id": "sfid-proj"},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore(), mock.NewFakeProjectLookup())
	assert.True(t, nak, "unknown project mapping should NAK")
}

func TestHandleDataStreamSubgroupUpdate_ProjectSlugLookupFails_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Err = fmt.Errorf("project service unavailable")

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"},
		&mock.SpyMessagePublisher{}, m, pl)
	assert.True(t, nak, "project slug lookup failure should NAK")
}

func TestHandleDataStreamSubgroupUpdate_CommitteeMappingAbsent_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			"committee":  "sfid-committee", // mapping absent
		},
		&mock.SpyMessagePublisher{}, m, pl)
	assert.True(t, nak, "unknown committee mapping should NAK")
}

func TestHandleDataStreamSubgroupUpdate_ParentServiceAbsent_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	// service mapping deliberately absent

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
		},
		&mock.SpyMessagePublisher{}, m, pl)
	assert.True(t, nak, "absent parent service should NAK")
}

func TestHandleDataStreamSubgroupUpdate_ParentServiceDomainAbsent_IndexesWithoutDomain(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"},
		pub, m, pl)

	assert.False(t, nak, "missing parent service domain should not block indexing")
	assert.Len(t, pub.IndexerCalls, 1)
	indexMessage := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	indexedData := indexMessage.Data.(map[string]any)
	assert.NotContains(t, indexedData, "domain", "domain should be omitted when the parent mapping is unavailable")
}

func TestHandleDataStreamSubgroupUpdate_ParentServiceDomainReadFails_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")
	m.SimulateGetError(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain))

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"
	pub := &mock.SpyMessagePublisher{}

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"}, pub, m, pl)

	assert.True(t, nak, "failed domain mapping reads should NAK for retry")
	assert.Empty(t, pub.IndexerCalls, "failed domain reads must not publish an incomplete document")
}

func TestHandleDataStreamSubgroupUpdate_HappyPath_ACKAndPublishesAndWritesMappings(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			"group_id":   float64(42),
			"group_name": "dev",
		},
		pub, m, pl)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMailingListSubject, pub.IndexerCalls[0].Subject)
	indexMessage := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	indexedData := indexMessage.Data.(map[string]any)
	assert.Equal(t, "lists.example.org", indexedData["domain"])
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, fgaconstants.GenericUpdateAccessSubject, pub.AccessCalls[0].Subject)

	_, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup))
	assert.True(t, present, "forward mapping should be written")

	rev, ok := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID))
	assert.True(t, ok, "reverse group_id index should be written")
	assert.Equal(t, "sg-1", rev)

	projMapping, ok := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroupProject))
	assert.True(t, ok, "project mapping should be written")
	assert.Equal(t, "proj-uid|my-project", projMapping)
}

func TestHandleDataStreamSubgroupUpdate_WithCommittee_ResolvesAndPublishes(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.sfid-committee", constants.KVMappingPrefixCommitteeBySFID), "committee-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			"committee":  "sfid-committee",
		},
		pub, m, pl)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
}

func TestHandleDataStreamSubgroupUpdate_CommitteeMappingFormat_WithCommittee(t *testing.T) {
	// Verify the subgroup handler writes the committee mapping in the format the message
	// handler expects: "{committeeUID}|{isPublic}".
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.sfid-committee", constants.KVMappingPrefixCommitteeBySFID), "committee-uid-abc")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "ml-uid",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			"committee":  "sfid-committee",
			"visibility": "public",
		},
		&mock.SpyMessagePublisher{}, m, pl)

	assert.False(t, nak)
	val, ok := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.ml-uid", constants.KVMappingPrefixSubgroupCommittee))
	assert.True(t, ok, "committee mapping should be written")
	assert.Equal(t, "committee-uid-abc|true", val,
		"format must be {committeeUID}|{isPublic} — message handler splits on '|'")
}

func TestHandleDataStreamSubgroupUpdate_CommitteeMappingFormat_NoCommittee(t *testing.T) {
	// Verify the subgroup handler writes an empty-committeeUID marker when no committee is
	// linked, so the message handler can distinguish "no committee" from "not yet processed".
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "ml-uid",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			// no "committee" field — private list without committee
			"visibility": "private",
		},
		&mock.SpyMessagePublisher{}, m, pl)

	assert.False(t, nak)
	val, ok := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.ml-uid", constants.KVMappingPrefixSubgroupCommittee))
	assert.True(t, ok, "committee mapping must be written even with no committee")
	assert.Equal(t, "|false", val,
		"format must be empty committeeUID with isPublic=false for private lists")
}

func TestHandleDataStreamSubgroupUpdate_NoGroupID_NoReverseIndex(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"},
		&mock.SpyMessagePublisher{}, m, pl)

	_, ok := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.0", constants.KVMappingPrefixSubgroupByGroupID))
	assert.False(t, ok, "should not write reverse index when group_id is absent")
}

func TestSubgroupUnassociated_PurgeThenParentFailureRetriesCleanup(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	uid := "ml-1"
	indexKey := constants.KVMappingPrefixSubgroupByService + ".svc-1." + uid
	parentKey := constants.KVMappingPrefixSubgroupParent + "." + uid
	subgroupKey := constants.KVMappingPrefixSubgroup + "." + uid
	m.Set(indexKey, uid)
	m.Set(parentKey, "svc-1")
	m.Set(subgroupKey, uid)
	m.SimulateTombstoneError(parentKey, errors.New("connection unavailable"))
	pub := &mock.SpyMessagePublisher{}
	assert.True(t, HandleDataStreamSubgroupUnassociated(ctx, uid, "svc-1", map[string]any{"visibility": "public"}, pub, m))
	require.Len(t, pub.IndexerCalls, 1)
	msg := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	assert.Equal(t, model.ActionDeleted, msg.Action)
	require.NotNil(t, msg.IndexingConfig)
	assert.Equal(t, uid, msg.IndexingConfig.ObjectID)
	assert.NotEmpty(t, msg.IndexingConfig.AccessCheckObject)
	assert.NotEmpty(t, msg.IndexingConfig.HistoryCheckRelation)
	require.Len(t, pub.AccessCalls, 1)
	access := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage)
	assert.Equal(t, fgaconstants.GenericUpdateAccessSubject, pub.AccessCalls[0].Subject)
	data := access.Data.(fgatypes.GenericAccessData)
	assert.Empty(t, data.References[constants.RelationGroupsIOService])
	assert.True(t, data.Public, "unassociation must preserve the list's current public visibility")
	assert.NotContains(t, data.ExcludeRelations, constants.RelationViewer, "viewer must track current visibility")
	assert.Contains(t, data.ExcludeRelations, constants.RelationMember)
	assert.Contains(t, data.ExcludeRelations, constants.RelationWriter)
	_, present := m.GetMappingValue(ctx, indexKey)
	assert.False(t, present)
	assert.Equal(t, "svc-1", mustMapping(t, ctx, m, parentKey))
	m.SimulateTombstoneError(parentKey, nil)
	assert.False(t, HandleDataStreamSubgroupUnassociated(ctx, uid, "svc-1", map[string]any{"visibility": "public"}, pub, m))
	assert.Len(t, pub.AccessCalls, 2, "redelivery after index purge must still revoke service access")
	assert.True(t, m.IsTombstoned(ctx, parentKey))
	assert.Equal(t, uid, mustMapping(t, ctx, m, subgroupKey), "reparenting must remain possible")
}

func TestSubgroupUnassociated_AccessFailurePreservesIndexForRetry(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	indexKey := constants.KVMappingPrefixSubgroupByService + ".svc-1.ml-1"
	m.Set(indexKey, "ml-1")
	m.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	pub := &mock.SpyMessagePublisher{AccessError: errors.New("connection unavailable")}
	assert.True(t, HandleDataStreamSubgroupUnassociated(ctx, "ml-1", "svc-1", map[string]any{"visibility": "private"}, pub, m))
	assert.True(t, m.IsMappingPresent(ctx, indexKey))
	pub.AccessError = nil
	assert.False(t, HandleDataStreamSubgroupUnassociated(ctx, "ml-1", "svc-1", map[string]any{"visibility": "private"}, pub, m))
	assert.False(t, m.IsMappingPresent(ctx, indexKey))
}

func TestSubgroupUnassociated_UsesPublishedParentWithoutNormalMapping(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	uid := "ml-1"
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + "." + uid
	m.Set(publishedKey, "svc-1")
	m.Set(constants.KVMappingPrefixSubgroup+"."+uid, uid)
	pub := &mock.SpyMessagePublisher{}
	assert.False(t, HandleDataStreamSubgroupUnassociated(ctx, uid, "svc-1", map[string]any{"visibility": "private"}, pub, m))
	require.Len(t, pub.IndexerCalls, 1, "published document must be deleted even without a normal parent/index")
	require.NotNil(t, pub.IndexerCalls[0].Message.(*model.IndexerMessage).IndexingConfig)
	require.Len(t, pub.AccessCalls, 1)
	assert.True(t, m.IsTombstoned(ctx, publishedKey))
	assert.Equal(t, uid, mustMapping(t, ctx, m, constants.KVMappingPrefixSubgroup+"."+uid))
}

func TestSubgroupUnassociated_MarkerFailureRetriesUIDCleanup(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	uid := "ml-1"
	m.Set(constants.KVMappingPrefixSubgroup+"."+uid, uid)
	markerKey := constants.KVMappingPrefixSubgroupUnassociated + "." + uid
	pendingKey := constants.KVMappingPrefixSubgroupUnassociationPending + "." + uid
	m.SimulatePutError(markerKey, errors.New("connection unavailable"))
	pub := &mock.SpyMessagePublisher{}
	require.True(t, HandleDataStreamSubgroupUnassociated(ctx, uid, "", map[string]any{"visibility": "private"}, pub, m))
	assert.False(t, m.IsMappingPresent(ctx, markerKey), "failed completion write must not conceal incomplete cleanup")
	assert.Equal(t, uid, mustMapping(t, ctx, m, pendingKey), "progress survives a failed completion write")
	require.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, model.ActionDeleted, pub.IndexerCalls[0].Message.(*model.IndexerMessage).Action)
	m.SimulatePutError(markerKey, nil)
	require.False(t, HandleDataStreamSubgroupUnassociated(ctx, uid, "", map[string]any{"visibility": "private"}, pub, m))
	assert.Equal(t, uid, mustMapping(t, ctx, m, markerKey))
	assert.True(t, m.IsTombstoned(ctx, pendingKey))
	assert.Len(t, pub.IndexerCalls, 2, "redelivery must repeat idempotent UID-based cleanup")
}

func TestSubgroupUnassociated_ProgressWriteFailureDoesNotPublish(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	uid := "ml-1"
	m.Set(constants.KVMappingPrefixSubgroup+"."+uid, uid)
	pendingKey := constants.KVMappingPrefixSubgroupUnassociationPending + "." + uid
	m.SimulatePutError(pendingKey, errors.New("connection unavailable"))
	pub := &mock.SpyMessagePublisher{}
	require.True(t, HandleDataStreamSubgroupUnassociated(ctx, uid, "", map[string]any{"visibility": "private"}, pub, m))
	assert.Empty(t, pub.IndexerCalls)
	assert.Empty(t, pub.AccessCalls)
	assert.False(t, m.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupUnassociated+"."+uid))
}

func TestParentlessCommitteeVisibilityReadAndWriteFailuresRetry(t *testing.T) {
	ctx := context.Background()
	uid := "ml-1"
	key := constants.KVMappingPrefixSubgroupCommittee + "." + uid
	source := map[string]any{"visibility": "private"}
	pub := &mock.SpyMessagePublisher{}
	readFailure := mock.NewFakeMappingStore()
	readFailure.Set(key, "committee-uid|true")
	readFailure.SimulateGetError(key)
	require.True(t, HandleDataStreamSubgroupUnassociatedAccess(ctx, uid, source, pub, readFailure))
	assert.Empty(t, pub.AccessCalls, "a failed mapping read must not publish an access change")

	writeFailure := mock.NewFakeMappingStore()
	writeFailure.Set(key, "committee-uid|true")
	writeFailure.SimulatePutError(key, errors.New("connection unavailable"))
	require.True(t, HandleDataStreamSubgroupUnassociatedAccess(ctx, uid, source, pub, writeFailure))
	assert.Equal(t, "committee-uid|true", mustMapping(t, ctx, writeFailure, key))
	writeFailure.SimulatePutError(key, nil)
	require.False(t, HandleDataStreamSubgroupUnassociatedAccess(ctx, uid, source, pub, writeFailure))
	assert.Equal(t, "committee-uid|false", mustMapping(t, ctx, writeFailure, key))
}

func TestParentlessCommitteeVisibilityRestoresMissingCacheFromSource(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixCommitteeBySFID+".committee-sfid", "committee-uid")
	pub := &mock.SpyMessagePublisher{}
	require.False(t, HandleDataStreamSubgroupUnassociatedAccess(ctx, "ml-1", map[string]any{
		"visibility": "public", "committee": "committee-sfid",
	}, pub, m))
	assert.Equal(t, "committee-uid|true", mustMapping(t, ctx, m, constants.KVMappingPrefixSubgroupCommittee+".ml-1"))
}

func TestSubgroupUpdate_MarkerInvalidationFailureRetriesCompletedReassociation(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	markerKey := constants.KVMappingPrefixSubgroupUnassociated + ".ml-1"
	m.Set(markerKey, "ml-1")
	m.SimulateTombstoneError(markerKey, errors.New("connection unavailable"))
	lookup := mock.NewFakeProjectLookup()
	lookup.Slugs["proj-uid"] = "project"
	pub := &mock.SpyMessagePublisher{}
	data := map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"}
	require.True(t, HandleDataStreamSubgroupUpdate(ctx, "ml-1", data, pub, m, lookup))
	assert.Len(t, pub.IndexerCalls, 1, "document is published before completing reassociation")
	assert.Len(t, pub.AccessCalls, 1, "access is published before clearing the marker")
	assert.Equal(t, "ml-1", mustMapping(t, ctx, m, markerKey))
	m.SimulateTombstoneError(markerKey, nil)
	require.False(t, HandleDataStreamSubgroupUpdate(ctx, "ml-1", map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"}, pub, m, lookup))
	assert.True(t, m.IsTombstoned(ctx, markerKey))
	assert.Len(t, pub.IndexerCalls, 2)
}

func TestSubgroupUpdate_ReassociationAccessFailurePreservesMarker(t *testing.T) {
	for _, completed := range []bool{false, true} {
		name := "interrupted before completion marker"
		if completed {
			name = "completed unassociation"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			m := mock.NewFakeMappingStore()
			m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
			m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
			m.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
			markerKey := constants.KVMappingPrefixSubgroupUnassociated + ".ml-1"
			pendingKey := constants.KVMappingPrefixSubgroupUnassociationPending + ".ml-1"
			m.Set(pendingKey, "ml-1")
			if completed {
				m.Set(markerKey, "ml-1")
			}
			lookup := mock.NewFakeProjectLookup()
			lookup.Slugs["proj-uid"] = "project"
			pub := &mock.SpyMessagePublisher{AccessError: errors.New("connection unavailable")}
			data := func() map[string]any { return map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"} }
			require.True(t, HandleDataStreamSubgroupUpdate(ctx, "ml-1", data(), pub, m, lookup))
			assert.Equal(t, "ml-1", mustMapping(t, ctx, m, pendingKey))
			if completed {
				assert.Equal(t, "ml-1", mustMapping(t, ctx, m, markerKey))
			}
			assert.False(t, m.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupParent+".ml-1"))
			pub.AccessError = nil
			require.False(t, HandleDataStreamSubgroupUpdate(ctx, "ml-1", data(), pub, m, lookup))
			assert.True(t, m.IsTombstoned(ctx, markerKey))
			assert.True(t, m.IsTombstoned(ctx, pendingKey))
			assert.True(t, m.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupParent+".ml-1"))
			assert.Len(t, pub.AccessCalls, 2)
		})
	}
}

func TestSubgroupServiceIndex_CreateMoveAndDelete(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixService+".svc-2", "svc-2")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"
	pub := &mock.SpyMessagePublisher{}
	update := func(uid, serviceUID string) bool {
		return HandleDataStreamSubgroupUpdate(ctx, uid, map[string]any{
			"project_id": "sfid-proj", "parent_id": serviceUID,
		}, pub, m, pl)
	}

	assert.False(t, update("sg-1", "svc-1"))
	assert.False(t, update("sg-2", "svc-1"))
	assert.False(t, update("sg-3", "svc-2"))
	indexKey := constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1"
	indexedUID, present := m.GetMappingValue(ctx, indexKey)
	assert.True(t, present)
	assert.Equal(t, "sg-1", indexedUID)
	uids, err := m.ListSubgroupsByService(ctx, "svc-1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"sg-1", "sg-2"}, uids)
	uids, err = m.ListSubgroupsByService(ctx, "svc-missing")
	require.NoError(t, err)
	assert.Empty(t, uids)

	assert.False(t, update("sg-1", "svc-2"))
	assert.True(t, m.IsTombstoned(ctx, indexKey))
	uids, err = m.ListSubgroupsByService(ctx, "svc-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"sg-2"}, uids)
	uids, err = m.ListSubgroupsByService(ctx, "svc-2")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"sg-1", "sg-3"}, uids)

	assert.False(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m))
	assert.True(t, m.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupByService+".svc-2.sg-1"))
	assert.True(t, m.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupParent+".sg-1"))
	uids, err = m.ListSubgroupsByService(ctx, "svc-2")
	require.NoError(t, err)
	assert.Equal(t, []string{"sg-3"}, uids)
}

func TestSubgroupServiceIndex_MoveCleanupFailureRetries(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixService+".svc-2", "svc-2")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"
	update := func(serviceUID string) bool {
		return HandleDataStreamSubgroupUpdate(ctx, "sg-1", map[string]any{
			"project_id": "sfid-proj", "parent_id": serviceUID,
		}, &mock.SpyMessagePublisher{}, m, pl)
	}
	require.False(t, update("svc-1"))
	oldIndexKey := constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1"
	m.SimulateTombstoneError(oldIndexKey, errors.New("connection timeout"))
	assert.True(t, update("svc-2"))
	parent, present := m.GetMappingValue(ctx, constants.KVMappingPrefixSubgroupParent+".sg-1")
	assert.True(t, present)
	assert.Equal(t, "svc-2", parent)
	assert.Equal(t, "svc-1", mustMapping(t, ctx, m, constants.KVMappingPrefixSubgroupPreviousService+".sg-1"))
	m.SimulateTombstoneError(oldIndexKey, nil)
	assert.False(t, update("svc-2"))
	assert.True(t, m.IsTombstoned(ctx, oldIndexKey))
	assert.True(t, m.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupPreviousService+".sg-1"))
}

func mustMapping(t *testing.T, ctx context.Context, m *mock.FakeMappingStore, key string) string {
	t.Helper()
	value, present := m.GetMappingValue(ctx, key)
	require.True(t, present, key)
	return value
}

func TestSubgroupServiceIndex_ParentWriteFailurePreservesOldLookup(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixService+".svc-2", "svc-2")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"
	update := func(parent string) bool {
		return HandleDataStreamSubgroupUpdate(ctx, "sg-1", map[string]any{"project_id": "sfid-proj", "parent_id": parent}, &mock.SpyMessagePublisher{}, m, pl)
	}
	require.False(t, update("svc-1"))
	parentKey := constants.KVMappingPrefixSubgroupParent + ".sg-1"
	m.SimulatePutError(parentKey, errors.New("connection timeout"))
	require.True(t, update("svc-2"))
	assert.Equal(t, "svc-1", mustMapping(t, ctx, m, parentKey))
	assert.False(t, m.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupByService+".svc-1.sg-1"))
	m.SimulatePutError(parentKey, nil)
	require.False(t, update("svc-2"))
	assert.True(t, m.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupByService+".svc-1.sg-1"))
}

func TestSubgroupServiceIndex_PendingCleanupSurvivesRedelivery(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixService+".svc-2", "svc-2")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"
	update := func(parent string) bool {
		return HandleDataStreamSubgroupUpdate(ctx, "sg-1", map[string]any{"project_id": "sfid-proj", "parent_id": parent}, &mock.SpyMessagePublisher{}, m, pl)
	}
	require.False(t, update("svc-1"))
	oldKey := constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1"
	m.SimulateTombstoneError(oldKey, errors.New("connection timeout"))
	require.True(t, update("svc-2"))
	assert.Equal(t, "svc-2", mustMapping(t, ctx, m, constants.KVMappingPrefixSubgroupParent+".sg-1"))
	assert.Equal(t, "svc-1", mustMapping(t, ctx, m, constants.KVMappingPrefixSubgroupPreviousService+".sg-1"))
	m.SimulateTombstoneError(oldKey, nil)
	require.False(t, update("svc-2"))
	assert.True(t, m.IsTombstoned(ctx, oldKey))
	assert.True(t, m.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupPreviousService+".sg-1"))
}

func TestSubgroupServiceIndex_PermanentFailureACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"
	m.SimulatePutError(constants.KVMappingPrefixSubgroupByService+".svc-1.sg-1", errors.New("invalid key"))
	assert.False(t, HandleDataStreamSubgroupUpdate(context.Background(), "sg-1", map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"}, &mock.SpyMessagePublisher{}, m, pl))
}

func TestSubgroupServiceIndex_FailedWriteRetries(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"
	indexKey := constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1"
	m.SimulatePutError(indexKey, errors.New("connection timeout"))
	update := func() bool {
		return HandleDataStreamSubgroupUpdate(ctx, "sg-1", map[string]any{
			"project_id": "sfid-proj", "parent_id": "svc-1", "group_id": float64(42),
		}, &mock.SpyMessagePublisher{}, m, pl)
	}
	assert.True(t, update())
	// An unavailable optional lookup bucket must not gate mappings required
	// by member and message handlers, even if event deliveries are exhausted.
	for key, value := range map[string]string{
		constants.KVMappingPrefixSubgroupByGroupID + ".42":   "sg-1",
		constants.KVMappingPrefixSubgroupProject + ".sg-1":   "proj-uid|my-project",
		constants.KVMappingPrefixSubgroupCommittee + ".sg-1": "|false",
	} {
		stored, present := m.GetMappingValue(ctx, key)
		assert.True(t, present, key)
		assert.Equal(t, value, stored)
	}
	uids, err := m.ListSubgroupsByService(ctx, "svc-1")
	require.NoError(t, err)
	assert.Empty(t, uids)
	m.SimulatePutError(indexKey, nil)
	assert.False(t, update())
	uids, err = m.ListSubgroupsByService(ctx, "svc-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"sg-1"}, uids)

	m.SimulateTombstoneError(indexKey, errors.New("connection timeout"))
	assert.True(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", &mock.SpyMessagePublisher{}, m))
	m.SimulateTombstoneError(indexKey, nil)
	assert.False(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", &mock.SpyMessagePublisher{}, m))
	uids, err = m.ListSubgroupsByService(ctx, "svc-1")
	require.NoError(t, err)
	assert.Empty(t, uids)
}

func TestHandleDataStreamSubgroupDelete_DuplicateDelete_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	_ = m.PutTombstone(ctx, fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should not publish")
}

func TestHandleDataStreamSubgroupDelete_ForwardMappingWriteFailedAfterPublication(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "project"
	forwardKey := constants.KVMappingPrefixSubgroup + ".sg-1"
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + ".sg-1"
	m.SimulatePutError(forwardKey, errors.New("connection timeout"))
	pub := &mock.SpyMessagePublisher{}
	require.True(t, HandleDataStreamSubgroupUpdate(ctx, "sg-1", map[string]any{
		"project_id": "sfid-proj", "parent_id": "svc-1",
	}, pub, m, pl))
	require.Len(t, pub.IndexerCalls, 1)
	assert.True(t, m.IsMappingPresent(ctx, publishedKey))
	assert.False(t, m.IsMappingPresent(ctx, forwardKey))

	m.SimulatePutError(forwardKey, nil)
	require.False(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m))
	require.Len(t, pub.IndexerCalls, 2)
	assert.Equal(t, model.ActionDeleted, pub.IndexerCalls[1].Message.(*model.IndexerMessage).Action)
	require.Len(t, pub.AccessCalls, 2)
	assert.Equal(t, fgaconstants.GenericDeleteAccessSubject, pub.AccessCalls[1].Subject)
	assert.True(t, m.IsTombstoned(ctx, publishedKey))
	assert.True(t, m.IsTombstoned(ctx, forwardKey))
}

func TestHandleDataStreamSubgroupDelete_AfterUnassociationWithoutForwardMappingDeletesAccess(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixCommitteeBySFID+".committee-sfid", "committee-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	lookup := mock.NewFakeProjectLookup()
	lookup.Slugs["proj-uid"] = "project"
	uid := "ml-1"
	forwardKey := constants.KVMappingPrefixSubgroup + "." + uid
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + "." + uid
	completedKey := constants.KVMappingPrefixSubgroupUnassociated + "." + uid
	m.SimulatePutError(forwardKey, errors.New("connection timeout"))
	pub := &mock.SpyMessagePublisher{}
	require.True(t, HandleDataStreamSubgroupUpdate(ctx, uid, map[string]any{
		"project_id": "sfid-proj", "parent_id": "svc-1", "committee": "committee-sfid", "writers": []string{"writer"},
	}, pub, m, lookup))
	assert.False(t, m.IsMappingPresent(ctx, forwardKey))
	assert.True(t, m.IsMappingPresent(ctx, publishedKey))
	require.False(t, HandleDataStreamSubgroupUnassociated(ctx, uid, "", map[string]any{
		"visibility": "private", "committee": "committee-sfid",
	}, pub, m))
	assert.True(t, m.IsTombstoned(ctx, publishedKey))
	assert.Equal(t, uid, mustMapping(t, ctx, m, completedKey))
	assert.Equal(t, "committee-uid|false", mustMapping(t, ctx, m, constants.KVMappingPrefixSubgroupCommittee+"."+uid))
	assert.False(t, m.IsMappingPresent(ctx, forwardKey))

	pub.AccessError = errors.New("connection unavailable")
	require.True(t, HandleDataStreamSubgroupDelete(ctx, uid, pub, m))
	assert.Equal(t, uid, mustMapping(t, ctx, m, completedKey), "failed delete_access must preserve recovery evidence")
	pub.AccessError = nil
	require.False(t, HandleDataStreamSubgroupDelete(ctx, uid, pub, m))
	assert.Equal(t, fgaconstants.GenericDeleteAccessSubject, pub.AccessCalls[len(pub.AccessCalls)-1].Subject)
	assert.True(t, m.IsTombstoned(ctx, completedKey))
	assert.True(t, m.IsTombstoned(ctx, forwardKey))
}

func TestHandleDataStreamSubgroupDelete_ParentWriteFailedAfterIndexWrite(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "project"
	parentKey := constants.KVMappingPrefixSubgroupParent + ".sg-1"
	indexKey := constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1"
	m.SimulatePutError(parentKey, errors.New("connection timeout"))
	pub := &mock.SpyMessagePublisher{}
	require.True(t, HandleDataStreamSubgroupUpdate(ctx, "sg-1", map[string]any{
		"project_id": "sfid-proj", "parent_id": "svc-1",
	}, pub, m, pl))
	assert.True(t, m.IsMappingPresent(ctx, indexKey))
	assert.False(t, m.IsMappingPresent(ctx, parentKey))
	m.SimulatePutError(parentKey, nil)
	require.False(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m))
	assert.True(t, m.IsTombstoned(ctx, indexKey))
	assert.True(t, m.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupPublishedParent+".sg-1"))
}

func TestHandleDataStreamSubgroupDelete_PartialMoveCleansBothServiceIndexes(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".sfid-proj", "proj-uid")
	for _, serviceUID := range []string{"svc-a", "svc-b"} {
		m.Set(constants.KVMappingPrefixService+"."+serviceUID, serviceUID)
	}
	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "project"
	pub := &mock.SpyMessagePublisher{}
	update := func(parent string) bool {
		return HandleDataStreamSubgroupUpdate(ctx, "sg-1", map[string]any{
			"project_id": "sfid-proj", "parent_id": parent,
		}, pub, m, pl)
	}
	parentKey := constants.KVMappingPrefixSubgroupParent + ".sg-1"
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + ".sg-1"
	pendingKey := constants.KVMappingPrefixSubgroupPreviousService + ".sg-1"
	oldIndex := constants.KVMappingPrefixSubgroupByService + ".svc-a.sg-1"
	newIndex := constants.KVMappingPrefixSubgroupByService + ".svc-b.sg-1"
	require.False(t, update("svc-a"))
	m.SimulatePutError(parentKey, errors.New("connection timeout"))
	require.True(t, update("svc-b"))
	assert.Equal(t, "svc-a", mustMapping(t, ctx, m, parentKey))
	assert.Equal(t, "svc-b", mustMapping(t, ctx, m, publishedKey))
	assert.Equal(t, "svc-a", mustMapping(t, ctx, m, pendingKey))
	assert.True(t, m.IsMappingPresent(ctx, oldIndex))
	assert.True(t, m.IsMappingPresent(ctx, newIndex))

	m.SimulatePutError(parentKey, nil)
	m.SimulateTombstoneError(newIndex, errors.New("connection timeout"))
	require.True(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m))
	assert.True(t, m.IsMappingPresent(ctx, newIndex), "failed index cleanup must remain retryable")
	assert.Equal(t, "svc-b", mustMapping(t, ctx, m, publishedKey))
	assert.Equal(t, "svc-a", mustMapping(t, ctx, m, parentKey))

	m.SimulateTombstoneError(newIndex, nil)
	require.False(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m))
	for _, key := range []string{oldIndex, newIndex, parentKey, publishedKey, pendingKey, constants.KVMappingPrefixSubgroup + ".sg-1"} {
		assert.True(t, m.IsTombstoned(ctx, key), key)
	}
	callCount := len(pub.IndexerCalls)
	require.False(t, HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m))
	assert.Len(t, pub.IndexerCalls, callCount, "duplicate delete should not publish")
}

func TestHandleDataStreamSubgroupDelete_ParentAlreadyClearedStillRemovesPublishedIndex(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	uid := "sg-1"
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + "." + uid
	indexKey := constants.KVMappingPrefixSubgroupByService + ".svc-b." + uid
	m.Set(publishedKey, "svc-b")
	m.Set(indexKey, uid)
	m.Set(constants.KVMappingPrefixSubgroup+"."+uid, uid)
	pub := &mock.SpyMessagePublisher{}
	require.False(t, HandleDataStreamSubgroupDelete(ctx, uid, pub, m))
	assert.True(t, m.IsTombstoned(ctx, indexKey))
	assert.True(t, m.IsTombstoned(ctx, publishedKey))
}

func TestHandleDataStreamSubgroupUpdate_PutMappingFailure_Transient_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	mKey := fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup)
	m.SimulatePutError(mKey, errors.New("connection timeout"))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
		},
		pub, m, pl)

	assert.True(t, nak, "transient PutMapping failure should NAK for retry")
	assert.Len(t, pub.IndexerCalls, 1, "indexer message was published before mapping write failed")
	assert.Len(t, pub.AccessCalls, 1, "access message was published before mapping write failed")
}

func TestHandleDataStreamSubgroupUpdate_PutMappingFailure_Permanent_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain), "lists.example.org")

	pl := mock.NewFakeProjectLookup()
	pl.Slugs["proj-uid"] = "my-project"

	mKey := fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup)
	m.SimulatePutError(mKey, errors.New("internal server error"))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
		},
		pub, m, pl)

	assert.False(t, nak, "permanent PutMapping failure should ACK (not retry)")
}

func TestHandleDataStreamSubgroupDelete_HappyPath_ACKAndTombstones(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup), "sg-1")
	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupDelete(context.Background(), "sg-1", pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMailingListSubject, pub.IndexerCalls[0].Subject)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, fgaconstants.GenericDeleteAccessSubject, pub.AccessCalls[0].Subject)

	assert.True(t, m.IsTombstoned(context.Background(),
		fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup)))
}

func TestTransformV1ToGrpsIOMailingList_VisibilityMapping(t *testing.T) {
	tests := []struct {
		name               string
		visibility         any
		wantPublic         bool
		wantAudienceAccess string
	}{
		{"lowercase public", "public", true, "public"},
		{"mixed-case Public", "Public", true, "Public"},
		{"uppercase PUBLIC", "PUBLIC", true, "PUBLIC"},
		{"approval_required", "approval_required", false, "approval_required"},
		{"invite_only", "invite_only", false, "invite_only"},
		{"missing visibility", nil, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{}
			if tt.visibility != nil {
				data["visibility"] = tt.visibility
			}

			list := transformV1ToGrpsIOMailingList("sg-1", data)

			assert.Equal(t, tt.wantPublic, list.Public)
			assert.Equal(t, tt.wantAudienceAccess, list.AudienceAccess)
		})
	}
}
