// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- test doubles ----

// spyInternalPublisher records every Internal() call; Indexer/Access are no-ops.
type spyInternalPublisher struct {
	calls []internalCall
	err   error
}

type internalCall struct {
	subject string
	message any
}

func (s *spyInternalPublisher) Internal(_ context.Context, subject string, message any) error {
	s.calls = append(s.calls, internalCall{subject, message})
	return s.err
}
func (s *spyInternalPublisher) Indexer(_ context.Context, _ string, _ any) error { return nil }
func (s *spyInternalPublisher) Access(_ context.Context, _ string, _ any) error  { return nil }

var _ port.MessagePublisher = (*spyInternalPublisher)(nil)

// stubMLWriter returns configured responses for Create/Update; Delete returns deleteErr.
type stubMLWriter struct {
	createResp *model.GroupsIOMailingList
	updateResp *model.GroupsIOMailingList
	createErr  error
	updateErr  error
	deleteErr  error
}

func (w *stubMLWriter) CreateMailingList(_ context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	if w.createResp != nil {
		return w.createResp, w.createErr
	}
	return ml, w.createErr
}

func (w *stubMLWriter) UpdateMailingList(_ context.Context, _ string, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	if w.updateResp != nil {
		return w.updateResp, w.updateErr
	}
	return ml, w.updateErr
}

func (w *stubMLWriter) DeleteMailingList(_ context.Context, _ string) error { return w.deleteErr }

var _ port.GroupsIOMailingListWriter = (*stubMLWriter)(nil)

// stubMLReader always returns the configured ml/err from GetMailingList.
// listMLs/listErr control ListMailingLists responses for the count check.
type stubMLReader struct {
	ml      *model.GroupsIOMailingList
	err     error
	listMLs []*model.GroupsIOMailingList
	listErr error
}

func (r *stubMLReader) GetMailingList(_ context.Context, _ string) (*model.GroupsIOMailingList, error) {
	return r.ml, r.err
}
func (r *stubMLReader) ListMailingLists(_ context.Context, _, _ string) ([]*model.GroupsIOMailingList, int, error) {
	return r.listMLs, len(r.listMLs), r.listErr
}
func (r *stubMLReader) GetMailingListCount(_ context.Context, _ string) (int, error) { return 0, nil }
func (r *stubMLReader) GetMailingListMemberCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

var _ port.GroupsIOMailingListReader = (*stubMLReader)(nil)

// passthroughTranslator returns fromID unchanged — lets us omit NATS in unit tests.
type passthroughTranslator struct{}

func (p *passthroughTranslator) MapID(_ context.Context, _, _, fromID string) (string, error) {
	return fromID, nil
}

var _ port.Translator = (*passthroughTranslator)(nil)

// stubServiceReader returns the configured service/err from GetService.
type stubServiceReader struct {
	svc *model.GroupsIOService
	err error
}

func (r *stubServiceReader) GetService(_ context.Context, _ string) (*model.GroupsIOService, error) {
	return r.svc, r.err
}
func (r *stubServiceReader) ListServices(_ context.Context, _ string) ([]*model.GroupsIOService, int, error) {
	return nil, 0, nil
}
func (r *stubServiceReader) GetProjects(_ context.Context) ([]string, error) { return nil, nil }
func (r *stubServiceReader) FindParentService(_ context.Context, _ string) (*model.GroupsIOService, error) {
	return nil, nil
}

var _ port.GroupsIOServiceReader = (*stubServiceReader)(nil)

// stubCommitteeProjectLookup returns the configured projectUID/err from GetCommitteeProject.
type stubCommitteeProjectLookup struct {
	projectUID string
	err        error
}

func (s *stubCommitteeProjectLookup) GetCommitteeProject(_ context.Context, _ string) (string, error) {
	return s.projectUID, s.err
}

var _ port.CommitteeProjectLookup = (*stubCommitteeProjectLookup)(nil)

// ---- helpers ----

func mlWith(committeeUID string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		ServiceUID: "test-service",
		Committees: []model.Committee{{UID: committeeUID}},
	}
}

func newTestOrchestrator(
	writer port.GroupsIOMailingListWriter,
	reader port.GroupsIOMailingListReader,
	pub port.MessagePublisher,
) *GroupsIOMailingListOrchestrator {
	return &GroupsIOMailingListOrchestrator{
		writer:                 writer,
		reader:                 reader,
		translator:             &passthroughTranslator{},
		publisher:              pub,
		serviceReader:          &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "test-project"}},
		committeeProjectLookup: &stubCommitteeProjectLookup{projectUID: "test-project"},
	}
}

func newTestOrchestratorWithValidation(
	writer port.GroupsIOMailingListWriter,
	reader port.GroupsIOMailingListReader,
	pub port.MessagePublisher,
	svcReader port.GroupsIOServiceReader,
	committeeProjectLookup port.CommitteeProjectLookup,
) *GroupsIOMailingListOrchestrator {
	return &GroupsIOMailingListOrchestrator{
		writer:                 writer,
		reader:                 reader,
		translator:             &passthroughTranslator{},
		publisher:              pub,
		serviceReader:          svcReader,
		committeeProjectLookup: committeeProjectLookup,
	}
}

func mlWithService(committeeUID, serviceUID string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		ServiceUID: serviceUID,
		Committees: []model.Committee{{UID: committeeUID}},
	}
}

// ---- publishCommitteeMailingListChanged ----

func TestPublishCommitteeMailingListChanged_EmptyUID_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	o := newTestOrchestrator(&stubMLWriter{}, nil, spy)
	o.publishCommitteeMailingListChanged(context.Background(), "", true)
	assert.Empty(t, spy.calls)
}

func TestPublishCommitteeMailingListChanged_NilPublisher_NoPublish(t *testing.T) {
	o := newTestOrchestrator(&stubMLWriter{}, nil, nil)
	// must not panic
	assert.NotPanics(t, func() {
		o.publishCommitteeMailingListChanged(context.Background(), "committee-uid-1", true)
	})
}

func TestPublishCommitteeMailingListChanged_ValidUID_PublishesTrue(t *testing.T) {
	spy := &spyInternalPublisher{}
	o := newTestOrchestrator(&stubMLWriter{}, nil, spy)
	o.publishCommitteeMailingListChanged(context.Background(), "committee-uid-1", true)

	require.Len(t, spy.calls, 1)
	assert.Equal(t, constants.CommitteeMailingListChangedSubject, spy.calls[0].subject)
	evt, ok := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	require.True(t, ok, "message should be *CommitteeMailingListChangedEvent")
	assert.Equal(t, "committee-uid-1", evt.CommitteeUID)
	assert.True(t, evt.HasMailingList)
}

func TestPublishCommitteeMailingListChanged_ValidUID_PublishesFalse(t *testing.T) {
	spy := &spyInternalPublisher{}
	o := newTestOrchestrator(&stubMLWriter{}, nil, spy)
	o.publishCommitteeMailingListChanged(context.Background(), "committee-uid-1", false)

	require.Len(t, spy.calls, 1)
	assert.Equal(t, constants.CommitteeMailingListChangedSubject, spy.calls[0].subject)
	evt, ok := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	require.True(t, ok)
	assert.Equal(t, "committee-uid-1", evt.CommitteeUID)
	assert.False(t, evt.HasMailingList)
}

// ---- fetchCommitteeUID ----

func TestFetchCommitteeUID_NilReader_ReturnsEmpty(t *testing.T) {
	o := newTestOrchestrator(&stubMLWriter{}, nil, nil)
	uid := o.fetchCommitteeUID(context.Background(), "ml-1")
	assert.Empty(t, uid)
}

func TestFetchCommitteeUID_ReaderError_ReturnsEmpty(t *testing.T) {
	reader := &stubMLReader{err: errors.New("not found")}
	o := newTestOrchestrator(&stubMLWriter{}, reader, nil)
	uid := o.fetchCommitteeUID(context.Background(), "ml-1")
	assert.Empty(t, uid)
}

func TestFetchCommitteeUID_NoCommittees_ReturnsEmpty(t *testing.T) {
	reader := &stubMLReader{ml: &model.GroupsIOMailingList{}}
	o := newTestOrchestrator(&stubMLWriter{}, reader, nil)
	uid := o.fetchCommitteeUID(context.Background(), "ml-1")
	assert.Empty(t, uid)
}

func TestFetchCommitteeUID_WithCommittee_ReturnsUID(t *testing.T) {
	reader := &stubMLReader{ml: mlWith("committee-abc")}
	o := newTestOrchestrator(&stubMLWriter{}, reader, nil)
	uid := o.fetchCommitteeUID(context.Background(), "ml-1")
	assert.Equal(t, "committee-abc", uid)
}

// ---- CreateMailingList ----

func TestCreateMailingList_WithCommittee_PublishesTrue(t *testing.T) {
	spy := &spyInternalPublisher{}
	writer := &stubMLWriter{createResp: mlWith("committee-create")}
	o := newTestOrchestrator(writer, nil, spy)

	resp, err := o.CreateMailingList(context.Background(), mlWith("committee-create"))
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Len(t, spy.calls, 1)
	evt := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "committee-create", evt.CommitteeUID)
	assert.True(t, evt.HasMailingList)
}

func TestCreateMailingList_NoCommittee_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	ml := &model.GroupsIOMailingList{}
	writer := &stubMLWriter{createResp: ml}
	o := newTestOrchestrator(writer, nil, spy)

	_, err := o.CreateMailingList(context.Background(), ml)
	require.NoError(t, err)
	assert.Empty(t, spy.calls)
}

func TestCreateMailingList_WriterError_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	writer := &stubMLWriter{createErr: errors.New("backend error")}
	o := newTestOrchestrator(writer, nil, spy)

	_, err := o.CreateMailingList(context.Background(), mlWith("committee-xyz"))
	require.Error(t, err)
	assert.Empty(t, spy.calls)
}

// ---- UpdateMailingList ----

func TestUpdateMailingList_SameCommittee_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: mlWith("committee-same")}
	writer := &stubMLWriter{updateResp: mlWith("committee-same")}
	o := newTestOrchestrator(writer, reader, spy)

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("committee-same"))
	require.NoError(t, err)
	assert.Empty(t, spy.calls, "no event when committee unchanged")
}

func TestUpdateMailingList_CommitteeChanged_PublishesFalseOldTrueNew(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: mlWith("old-committee")}
	writer := &stubMLWriter{updateResp: mlWith("new-committee")}
	o := newTestOrchestrator(writer, reader, spy)

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("new-committee"))
	require.NoError(t, err)

	require.Len(t, spy.calls, 2)
	// first: old committee removed
	evtOld := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "old-committee", evtOld.CommitteeUID)
	assert.False(t, evtOld.HasMailingList)
	// second: new committee added
	evtNew := spy.calls[1].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "new-committee", evtNew.CommitteeUID)
	assert.True(t, evtNew.HasMailingList)
}

func TestUpdateMailingList_CommitteeRemovedOnUpdate_PublishesFalseOldNoNew(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: mlWith("old-committee")}              // listMLs empty → no remaining MLs
	writer := &stubMLWriter{updateResp: &model.GroupsIOMailingList{}} // no committee in response
	o := newTestOrchestrator(writer, reader, spy)

	_, err := o.UpdateMailingList(context.Background(), "ml-1", &model.GroupsIOMailingList{})
	require.NoError(t, err)

	// old gets false (no remaining MLs); new is empty so no second event
	require.Len(t, spy.calls, 1)
	evt := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "old-committee", evt.CommitteeUID)
	assert.False(t, evt.HasMailingList)
}

func TestUpdateMailingList_CommitteeAddedOnUpdate_PublishesTrueNew(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: &model.GroupsIOMailingList{}} // no old committee
	writer := &stubMLWriter{updateResp: mlWith("new-committee")}
	o := newTestOrchestrator(writer, reader, spy)

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("new-committee"))
	require.NoError(t, err)

	// old is empty so no remove event; new gets true
	require.Len(t, spy.calls, 1)
	evt := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "new-committee", evt.CommitteeUID)
	assert.True(t, evt.HasMailingList)
}

func TestUpdateMailingList_ReaderError_PublishesOnlyNewCommittee(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{err: errors.New("fetch failed")}
	writer := &stubMLWriter{updateResp: mlWith("new-committee")}
	o := newTestOrchestrator(writer, reader, spy)

	resp, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("new-committee"))
	require.NoError(t, err)
	require.NotNil(t, resp)

	// oldCUID is "" due to reader error, newCUID is "new-committee" → only the "add" event fires.
	// The "remove" for the unknown old committee is intentionally skipped (best-effort).
	require.Len(t, spy.calls, 1)
	evt := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "new-committee", evt.CommitteeUID)
	assert.True(t, evt.HasMailingList)
}

func TestUpdateMailingList_WriterError_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: mlWith("old-committee")}
	writer := &stubMLWriter{updateErr: errors.New("backend error")}
	o := newTestOrchestrator(writer, reader, spy)

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("new-committee"))
	require.Error(t, err)
	assert.Empty(t, spy.calls)
}

// ---- DeleteMailingList ----

func TestDeleteMailingList_WithCommittee_PublishesFalse(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: mlWith("committee-del")}
	writer := &stubMLWriter{}
	o := newTestOrchestrator(writer, reader, spy)

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.NoError(t, err)

	require.Len(t, spy.calls, 1)
	evt := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "committee-del", evt.CommitteeUID)
	assert.False(t, evt.HasMailingList)
}

func TestDeleteMailingList_NoCommittee_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: &model.GroupsIOMailingList{}}
	writer := &stubMLWriter{}
	o := newTestOrchestrator(writer, reader, spy)

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, spy.calls)
}

func TestDeleteMailingList_WriterError_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{ml: mlWith("committee-del")}
	writer := &stubMLWriter{deleteErr: errors.New("backend error")}
	o := newTestOrchestrator(writer, reader, spy)

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.Error(t, err)
	assert.Empty(t, spy.calls)
}

func TestDeleteMailingList_ReaderFailure_NoPublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	// reader fails → fetchCommitteeUID returns "" → no publish even after successful delete
	reader := &stubMLReader{err: errors.New("reader unavailable")}
	writer := &stubMLWriter{}
	o := newTestOrchestrator(writer, reader, spy)

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, spy.calls)
}

// ---- Multi-mailing-list-per-committee guard ----

func TestDeleteMailingList_CommitteeHasOtherMLs_NoFalsePublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{
		ml: mlWith("committee-shared"),
		// Another ML still references this committee (different UID from the one being deleted)
		listMLs: []*model.GroupsIOMailingList{{UID: "ml-2"}},
	}
	writer := &stubMLWriter{}
	o := newTestOrchestrator(writer, reader, spy)

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, spy.calls, "should not publish false when committee has other mailing lists")
}

func TestDeleteMailingList_CommitteeHasOnlySelf_PublishesFalse(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{
		ml: mlWith("committee-sole"),
		// ITX stale read: returns the ML we just deleted — client-side filter excludes it
		listMLs: []*model.GroupsIOMailingList{{UID: "ml-1"}},
	}
	writer := &stubMLWriter{}
	o := newTestOrchestrator(writer, reader, spy)

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.NoError(t, err)

	require.Len(t, spy.calls, 1)
	evt := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "committee-sole", evt.CommitteeUID)
	assert.False(t, evt.HasMailingList)
}

func TestDeleteMailingList_ListMLsError_SkipsFalsePublish(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{
		ml:      mlWith("committee-err"),
		listErr: errors.New("ITX unavailable"),
	}
	writer := &stubMLWriter{}
	o := newTestOrchestrator(writer, reader, spy)

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, spy.calls, "should not publish false when count check fails — assume others exist")
}

func TestUpdateMailingList_OldCommitteeHasOtherMLs_NoFalseForOld(t *testing.T) {
	spy := &spyInternalPublisher{}
	reader := &stubMLReader{
		ml: mlWith("old-committee"),
		// Another ML still references old-committee
		listMLs: []*model.GroupsIOMailingList{{UID: "ml-2"}},
	}
	writer := &stubMLWriter{updateResp: mlWith("new-committee")}
	o := newTestOrchestrator(writer, reader, spy)

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("new-committee"))
	require.NoError(t, err)

	// Only the "true" event for new committee, no "false" for old since it has ml-2
	require.Len(t, spy.calls, 1)
	evt := spy.calls[0].message.(*model.CommitteeMailingListChangedEvent)
	assert.Equal(t, "new-committee", evt.CommitteeUID)
	assert.True(t, evt.HasMailingList)
}

// ---- committeeHasRemainingMailingLists ----

func TestCommitteeHasRemainingMailingLists_NilReader_ReturnsFalse(t *testing.T) {
	o := newTestOrchestrator(&stubMLWriter{}, nil, nil)
	assert.False(t, o.committeeHasRemainingMailingLists(context.Background(), "c-1", "ml-1"))
}

func TestCommitteeHasRemainingMailingLists_Error_ReturnsTrue(t *testing.T) {
	reader := &stubMLReader{listErr: errors.New("ITX down")}
	o := newTestOrchestrator(&stubMLWriter{}, reader, nil)
	assert.True(t, o.committeeHasRemainingMailingLists(context.Background(), "c-1", "ml-1"),
		"should assume others exist when uncertain")
}

func TestCommitteeHasRemainingMailingLists_OnlySelf_ReturnsFalse(t *testing.T) {
	reader := &stubMLReader{listMLs: []*model.GroupsIOMailingList{{UID: "ml-1"}}}
	o := newTestOrchestrator(&stubMLWriter{}, reader, nil)
	assert.False(t, o.committeeHasRemainingMailingLists(context.Background(), "c-1", "ml-1"),
		"excluded ML is the only one — no remaining")
}

func TestCommitteeHasRemainingMailingLists_OtherExists_ReturnsTrue(t *testing.T) {
	reader := &stubMLReader{listMLs: []*model.GroupsIOMailingList{{UID: "ml-1"}, {UID: "ml-2"}}}
	o := newTestOrchestrator(&stubMLWriter{}, reader, nil)
	assert.True(t, o.committeeHasRemainingMailingLists(context.Background(), "c-1", "ml-1"),
		"ml-2 still exists after excluding ml-1")
}

func TestCommitteeHasRemainingMailingLists_Empty_ReturnsFalse(t *testing.T) {
	reader := &stubMLReader{listMLs: []*model.GroupsIOMailingList{}}
	o := newTestOrchestrator(&stubMLWriter{}, reader, nil)
	assert.False(t, o.committeeHasRemainingMailingLists(context.Background(), "c-1", "ml-1"))
}

// ---- validateCommitteeProject ----

func TestValidateCommitteeProject_NoCommittee_Skips(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := &model.GroupsIOMailingList{ServiceUID: "svc-1"}
	require.NoError(t, o.validateCommitteeProject(context.Background(), ml))
}

func TestValidateCommitteeProject_NilServiceReader_ReturnsServiceUnavailable(t *testing.T) {
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, nil, lookup)

	ml := mlWithService("committee-1", "svc-1")
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.IsType(t, errs.ServiceUnavailable{}, err)
}

func TestValidateCommitteeProject_NilLookup_ReturnsServiceUnavailable(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, nil)

	ml := mlWithService("committee-1", "svc-1")
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.IsType(t, errs.ServiceUnavailable{}, err)
}

func TestValidateCommitteeProject_NilService_ReturnsServiceUnavailable(t *testing.T) {
	svcReader := &stubServiceReader{svc: nil} // returns nil, nil
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := mlWithService("committee-1", "svc-1")
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.IsType(t, errs.ServiceUnavailable{}, err)
}

func TestValidateCommitteeProject_MissingServiceUID_ReturnsValidationError(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := &model.GroupsIOMailingList{
		Committees: []model.Committee{{UID: "committee-1"}},
	}
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.IsType(t, errs.Validation{}, err)
}

func TestValidateCommitteeProject_SameProject_Passes(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := mlWithService("committee-1", "svc-1")
	require.NoError(t, o.validateCommitteeProject(context.Background(), ml))
	assert.Equal(t, "proj-A", ml.ProjectUID, "ProjectUID should be set from the service")
}

func TestValidateCommitteeProject_DifferentProject_ReturnsValidationError(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-B"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := mlWithService("committee-1", "svc-1")
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.IsType(t, errs.Validation{}, err)
}

func TestValidateCommitteeProject_ServiceReaderError_Propagates(t *testing.T) {
	svcErr := errors.New("service not found")
	svcReader := &stubServiceReader{err: svcErr}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := mlWithService("committee-1", "svc-1")
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.Equal(t, svcErr, err)
}

func TestValidateCommitteeProject_LookupError_Propagates(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookupErr := errors.New("committee not found")
	lookup := &stubCommitteeProjectLookup{err: lookupErr}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := mlWithService("committee-1", "svc-1")
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.Equal(t, lookupErr, err)
}

// ---- CreateMailingList + UpdateMailingList validation integration ----

func TestCreateMailingList_CrossProjectCommittee_ReturnsError(t *testing.T) {
	spy := &spyInternalPublisher{}
	writer := &stubMLWriter{createResp: mlWith("committee-1")}
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-B"}
	o := newTestOrchestratorWithValidation(writer, nil, spy, svcReader, lookup)

	_, err := o.CreateMailingList(context.Background(), mlWithService("committee-1", "svc-1"))
	require.Error(t, err)
	assert.IsType(t, errs.Validation{}, err)
	assert.Empty(t, spy.calls, "no event published on validation failure")
}

func TestCreateMailingList_SameProjectCommittee_Succeeds(t *testing.T) {
	spy := &spyInternalPublisher{}
	writer := &stubMLWriter{createResp: mlWith("committee-1")}
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(writer, nil, spy, svcReader, lookup)

	resp, err := o.CreateMailingList(context.Background(), mlWithService("committee-1", "svc-1"))
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestUpdateMailingList_CrossProjectCommittee_ReturnsError(t *testing.T) {
	spy := &spyInternalPublisher{}
	writer := &stubMLWriter{updateResp: mlWith("committee-1")}
	reader := &stubMLReader{ml: mlWith("committee-old")}
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-B"}
	o := newTestOrchestratorWithValidation(writer, reader, spy, svcReader, lookup)

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWithService("committee-1", "svc-1"))
	require.Error(t, err)
	assert.IsType(t, errs.Validation{}, err)
	assert.Empty(t, spy.calls, "no event published on validation failure")
}

// ---- sync_mode validation ----

func TestValidateCommitteeProject_SyncModeWithoutCommitteeUID_ReturnsValidationError(t *testing.T) {
	o := newTestOrchestrator(&stubMLWriter{}, nil, nil)
	ml := &model.GroupsIOMailingList{
		ServiceUID: "svc-1",
		Committees: []model.Committee{{SyncMode: "auto_c_to_ml"}},
	}
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.IsType(t, errs.Validation{}, err)
}

func TestValidateCommitteeProject_InvalidSyncMode_ReturnsValidationError(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := &model.GroupsIOMailingList{
		ServiceUID: "svc-1",
		Committees: []model.Committee{{UID: "committee-1", SyncMode: "bogus"}},
	}
	err := o.validateCommitteeProject(context.Background(), ml)
	require.Error(t, err)
	assert.IsType(t, errs.Validation{}, err)
}

func TestValidateCommitteeProject_ValidSyncMode_Passes(t *testing.T) {
	svcReader := &stubServiceReader{svc: &model.GroupsIOService{ProjectUID: "proj-A"}}
	lookup := &stubCommitteeProjectLookup{projectUID: "proj-A"}
	o := newTestOrchestratorWithValidation(&stubMLWriter{}, nil, nil, svcReader, lookup)

	ml := &model.GroupsIOMailingList{
		ServiceUID: "svc-1",
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeAutoCommitteeToMailingList}},
	}
	require.NoError(t, o.validateCommitteeProject(context.Background(), ml))
}

// ---- sync_mode persistence ----

func mlWithSync(committeeUID, syncMode string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		ServiceUID: "test-service",
		Committees: []model.Committee{{UID: committeeUID, SyncMode: syncMode}},
	}
}

func TestCreateMailingList_PersistsSyncMode(t *testing.T) {
	store := mock.NewMockCommitteeSyncSettingsStore()
	createResp := mlWith("committee-1")
	createResp.UID = "ml-new"
	writer := &stubMLWriter{createResp: createResp}
	o := newTestOrchestrator(writer, nil, &spyInternalPublisher{})
	o.syncSettingsStore = store

	resp, err := o.CreateMailingList(context.Background(), mlWithSync("committee-1", "auto_c_to_ml"))
	require.NoError(t, err)
	assert.Equal(t, "auto_c_to_ml", resp.Committees[0].SyncMode)

	mode, _, found, err := store.GetSyncMode(context.Background(), "ml-new", "committee-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "auto_c_to_ml", mode)
}

func TestCreateMailingList_NoSyncMode_DoesNotPersist(t *testing.T) {
	store := mock.NewMockCommitteeSyncSettingsStore()
	createResp := mlWith("committee-1")
	createResp.UID = "ml-new"
	writer := &stubMLWriter{createResp: createResp}
	o := newTestOrchestrator(writer, nil, &spyInternalPublisher{})
	o.syncSettingsStore = store

	_, err := o.CreateMailingList(context.Background(), mlWith("committee-1"))
	require.NoError(t, err)

	_, _, found, err := store.GetSyncMode(context.Background(), "ml-new", "committee-1")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestUpdateMailingList_PersistsSyncModeForCurrentCommittee(t *testing.T) {
	store := mock.NewMockCommitteeSyncSettingsStore()
	reader := &stubMLReader{ml: mlWith("committee-1")}
	updateResp := mlWith("committee-1")
	writer := &stubMLWriter{updateResp: updateResp}
	o := newTestOrchestrator(writer, reader, &spyInternalPublisher{})
	o.syncSettingsStore = store

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWithSync("committee-1", "auto_ml_to_c"))
	require.NoError(t, err)

	mode, _, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "auto_ml_to_c", mode)
}

func TestUpdateMailingList_CommitteeSwap_CleansUpOldSyncMode(t *testing.T) {
	store := mock.NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "old-committee", "auto_c_to_ml"))

	reader := &stubMLReader{ml: mlWith("old-committee")}
	writer := &stubMLWriter{updateResp: mlWith("new-committee")}
	o := newTestOrchestrator(writer, reader, &spyInternalPublisher{})
	o.syncSettingsStore = store

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("new-committee"))
	require.NoError(t, err)

	_, _, found, err := store.GetSyncMode(context.Background(), "ml-1", "old-committee")
	require.NoError(t, err)
	assert.False(t, found, "stale sync_mode for the old committee must be cleaned up")
}

func TestDeleteMailingList_CleansUpSyncMode(t *testing.T) {
	store := mock.NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-del", "manual"))

	reader := &stubMLReader{ml: mlWith("committee-del")}
	writer := &stubMLWriter{}
	o := newTestOrchestrator(writer, reader, &spyInternalPublisher{})
	o.syncSettingsStore = store

	err := o.DeleteMailingList(context.Background(), "ml-1")
	require.NoError(t, err)

	_, _, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-del")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCreateMailingList_NilSyncSettingsStore_NoPanic(t *testing.T) {
	createResp := mlWith("committee-1")
	writer := &stubMLWriter{createResp: createResp}
	o := newTestOrchestrator(writer, nil, &spyInternalPublisher{})

	assert.NotPanics(t, func() {
		_, err := o.CreateMailingList(context.Background(), mlWithSync("committee-1", "auto_c_to_ml"))
		require.NoError(t, err)
	})
}

func TestCreateMailingList_PutSyncModeFails_StillSucceedsWithoutEchoingSyncMode(t *testing.T) {
	createResp := mlWith("committee-1")
	createResp.UID = "ml-new"
	writer := &stubMLWriter{createResp: createResp}
	o := newTestOrchestrator(writer, nil, &spyInternalPublisher{})
	o.syncSettingsStore = &stubSyncSettingsStore{putErr: errors.New("kv down")}

	resp, err := o.CreateMailingList(context.Background(), mlWithSync("committee-1", "auto_c_to_ml"))
	require.NoError(t, err, "a sync_mode persistence failure must not fail the overall create")
	assert.Empty(t, resp.Committees[0].SyncMode, "must not echo a sync_mode that failed to persist")
}

func TestUpdateMailingList_PutSyncModeFails_HydratesActualStoredValue(t *testing.T) {
	store := mock.NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "manual"))

	reader := &stubMLReader{ml: mlWith("committee-1")}
	writer := &stubMLWriter{updateResp: mlWith("committee-1")}
	o := newTestOrchestrator(writer, reader, &spyInternalPublisher{})
	o.syncSettingsStore = &failingPutSyncSettingsStore{CommitteeSyncSettingsStore: store}

	resp, err := o.UpdateMailingList(context.Background(), "ml-1", mlWithSync("committee-1", "auto_ml_to_c"))
	require.NoError(t, err, "a sync_mode persistence failure must not fail the overall update")
	assert.Equal(t, "manual", resp.Committees[0].SyncMode, "must echo the store's actual current value, not the unpersisted request value")
}

func TestUpdateMailingList_CommitteeSwap_CleanupSkipped_WhenNewerValueWrittenConcurrently(t *testing.T) {
	store := mock.NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "old-committee", "auto_c_to_ml"))
	// Simulate a concurrent request re-writing a newer value for the same (ml, old-committee)
	// pair between cleanupStaleSyncMode's read and its delete, by bumping the revision via
	// another Put right before the update runs.
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "old-committee", "manual"))

	reader := &stubMLReader{ml: mlWith("old-committee")}
	writer := &stubMLWriter{updateResp: mlWith("new-committee")}
	o := newTestOrchestrator(writer, reader, &spyInternalPublisher{})
	o.syncSettingsStore = &staleRevisionSyncSettingsStore{CommitteeSyncSettingsStore: store}

	_, err := o.UpdateMailingList(context.Background(), "ml-1", mlWith("new-committee"))
	require.NoError(t, err)

	mode, _, found, err := store.GetSyncMode(context.Background(), "ml-1", "old-committee")
	require.NoError(t, err)
	require.True(t, found, "the newer concurrent value must survive cleanup")
	assert.Equal(t, "manual", mode)
}

// failingPutSyncSettingsStore wraps a working store but forces PutSyncMode to fail, so
// Get/Delete still behave realistically while simulating a write-path failure.
type failingPutSyncSettingsStore struct {
	port.CommitteeSyncSettingsStore
}

func (f *failingPutSyncSettingsStore) PutSyncMode(_ context.Context, _, _, _ string) error {
	return errors.New("kv down")
}

// staleRevisionSyncSettingsStore wraps a working store but reports a stale revision from
// GetSyncMode, simulating a TOCTOU race where a newer value was written between the caller's
// read and its revision-gated delete.
type staleRevisionSyncSettingsStore struct {
	port.CommitteeSyncSettingsStore
}

func (s *staleRevisionSyncSettingsStore) GetSyncMode(ctx context.Context, mailingListUID, committeeUID string) (string, uint64, bool, error) {
	mode, revision, ok, err := s.CommitteeSyncSettingsStore.GetSyncMode(ctx, mailingListUID, committeeUID)
	if ok {
		revision--
	}
	return mode, revision, ok, err
}
