// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// ctxWithPrincipal returns a context carrying a test principal, needed for the committee
// writer-access check both sync orchestrators now perform.
func ctxWithPrincipal() context.Context {
	return context.WithValue(context.Background(), constants.PrincipalContextID, "test-user")
}

// stubITXSyncClient returns configured responses for the ITXSyncClient methods used by the
// sync-diff/sync-apply orchestrators.
type stubITXSyncClient struct {
	postDiffJobID     string
	postDiffErr       error
	postDiffDirection string

	diffJob    *port.SyncDiffJob
	diffJobErr error

	postApplyJobID     string
	postApplyErr       error
	postApplyDirection string
	postApplyAdds      []string
	postApplyRemoves   []string

	applyJob    *port.SyncApplyJob
	applyJobErr error
}

func (c *stubITXSyncClient) PostSyncDiff(_ context.Context, _, direction string) (string, error) {
	c.postDiffDirection = direction
	return c.postDiffJobID, c.postDiffErr
}

func (c *stubITXSyncClient) GetSyncDiffJob(_ context.Context, _, _ string) (*port.SyncDiffJob, error) {
	return c.diffJob, c.diffJobErr
}

func (c *stubITXSyncClient) PostSyncApply(_ context.Context, _, direction string, adds, removes []string, _ string) (string, error) {
	c.postApplyDirection = direction
	c.postApplyAdds = adds
	c.postApplyRemoves = removes
	return c.postApplyJobID, c.postApplyErr
}

func (c *stubITXSyncClient) GetSyncApplyJob(_ context.Context, _, _ string) (*port.SyncApplyJob, error) {
	return c.applyJob, c.applyJobErr
}

var _ port.ITXSyncClient = (*stubITXSyncClient)(nil)

// stubAccessChecker lets tests control the HasRelation outcome, unlike mock.MockAccessChecker
// which always allows.
type stubAccessChecker struct {
	allow bool
	err   error
}

func (c *stubAccessChecker) HasRelation(_ context.Context, _, _, _ string) (bool, error) {
	return c.allow, c.err
}

var _ port.AccessChecker = (*stubAccessChecker)(nil)

// stubTranslator is a fixed v1->v2 identity-style mapping for tests exercising
// verifyJobCommitteeMatch's translated-ID comparison.
type stubTranslator struct {
	v2ByV1 map[string]string
}

func (t *stubTranslator) MapID(_ context.Context, _, _, fromID string) (string, error) {
	if v2, ok := t.v2ByV1[fromID]; ok {
		return v2, nil
	}
	return "", errors.New("no mapping for " + fromID)
}

var _ port.Translator = (*stubTranslator)(nil)

func mlWithCommittee(committeeUID string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		UID:        "ml-1",
		Committees: []model.Committee{{UID: committeeUID}},
	}
}

func mlWithCommitteeSyncMode(committeeUID, syncMode string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		UID:        "ml-1",
		Committees: []model.Committee{{UID: committeeUID, SyncMode: syncMode}},
	}
}

func TestComputeSyncDiff_NoLinkedCommittee(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_ManualSyncMode(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeManual)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_EmptySyncModeTreatedAsManual(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommittee("committee-1")}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_UnrecognizedSyncMode(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", "bogus")}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_MailingListReaderError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{err: errors.New("not found")}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
}

func TestComputeSyncDiff_PostSyncDiffError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{postDiffErr: errs.NewServiceUnavailable("itx down")}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
}

func TestComputeSyncDiff_GetSyncDiffJobError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			postDiffJobID: "job-1",
			diffJobErr:    errors.New("itx unreachable"),
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
}

func TestComputeSyncDiff_JobFailed(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			postDiffJobID: "job-1",
			diffJob:       &port.SyncDiffJob{ID: "job-1", Status: port.SyncJobStatusFailed, ErrorMessage: "subgroup not found"},
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
	assert.Contains(t, err.Error(), "subgroup not found")
}

// TestComputeSyncDiff_Reconciliation pins the happy-path translation of a completed ITX
// sync-diff job into a model.SyncDiff.
func TestComputeSyncDiff_Reconciliation(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			postDiffJobID: "job-1",
			diffJob: &port.SyncDiffJob{
				ID:     "job-1",
				Status: port.SyncJobStatusCompleted,
				ToAdd: []port.SyncJobMember{
					{MemberID: "cm-2", Email: "bob@example.com", FullName: "Bob"},
				},
				ToRemove: []port.SyncJobMember{
					{MemberID: "mlm-2", Email: "carol@example.com", FullName: "Carol"},
				},
			},
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)

	diff, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.NoError(t, err)

	require.Len(t, diff.Adds, 1)
	assert.Equal(t, "bob@example.com", diff.Adds[0].Email)
	assert.Equal(t, "cm-2", diff.Adds[0].MemberID)
	assert.Equal(t, model.MemberTypeCommitteeMember, diff.Adds[0].MemberType)
	assert.Equal(t, "Bob", diff.Adds[0].Name)

	require.Len(t, diff.Removes, 1)
	assert.Equal(t, "carol@example.com", diff.Removes[0].Email)
	assert.Equal(t, "mlm-2", diff.Removes[0].MemberID)
	assert.Equal(t, model.MemberTypeMailingListMember, diff.Removes[0].MemberType)

	assert.Equal(t, "ml-1", diff.MailingListID)
	assert.Equal(t, "committee-1", diff.CommitteeUID)
	assert.Equal(t, model.SyncModeAutoCommitteeToMailingList, diff.SyncMode)
}

// TestComputeSyncDiff_MailingListToCommitteeDirection pins that auto_ml_to_c mailing lists are
// fully supported (not blocked), and that the reverse direction is sent to ITX.
func TestComputeSyncDiff_MailingListToCommitteeDirection(t *testing.T) {
	stub := &stubITXSyncClient{
		postDiffJobID: "job-1",
		diffJob: &port.SyncDiffJob{
			ID:     "job-1",
			Status: port.SyncJobStatusCompleted,
			ToAdd: []port.SyncJobMember{
				{MemberID: "mlm-1", Email: "dana@example.com", FullName: "Dana"},
			},
		},
	}
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoMailingListToCommittee)}),
		WithSyncDiffITXSyncClient(stub),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)

	diff, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobDirectionMailingListToCommittee, stub.postDiffDirection)
	assert.Equal(t, model.SyncModeAutoMailingListToCommittee, diff.SyncMode)
	require.Len(t, diff.Adds, 1)
	assert.Equal(t, "dana@example.com", diff.Adds[0].Email)
	assert.Equal(t, model.MemberTypeMailingListMember, diff.Adds[0].MemberType)
}

func TestComputeSyncDiff_NoChangesNeeded(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			postDiffJobID: "job-1",
			diffJob:       &port.SyncDiffJob{ID: "job-1", Status: port.SyncJobStatusCompleted},
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)

	diff, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, diff.Adds)
	assert.Empty(t, diff.Removes)
}

// TestComputeSyncDiff_CommitteeChangedWhileJobRunning pins that if the mailing list's linked
// committee changes between the sync-diff job starting and completing, the request fails instead
// of mislabeling the (stale) job's diff data as belonging to the newly-linked committee.
func TestComputeSyncDiff_CommitteeChangedWhileJobRunning(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{
			ml:             mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList),
			mlOnSecondCall: mlWithCommitteeSyncMode("committee-2", model.SyncModeAutoCommitteeToMailingList),
		}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			postDiffJobID: "job-1",
			diffJob:       &port.SyncDiffJob{ID: "job-1", Status: port.SyncJobStatusCompleted},
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)

	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var conflict errs.Conflict
	assert.True(t, errors.As(err, &conflict))
}

// TestComputeSyncDiff_JobOmittedCommitteeIDFailsClosed pins that once a translator is configured,
// a completed job that didn't report which committee it ran against is rejected instead of being
// trusted unverified — this is the one case verifyJobCommitteeMatch exists to catch.
func TestComputeSyncDiff_JobOmittedCommitteeIDFailsClosed(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			postDiffJobID: "job-1",
			diffJob:       &port.SyncDiffJob{ID: "job-1", Status: port.SyncJobStatusCompleted},
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
		WithSyncDiffTranslator(&stubTranslator{v2ByV1: map[string]string{"v1-committee-1": "committee-1"}}),
	)
	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
}

// TestComputeSyncDiff_AccessDenied pins that a caller with mailing-list access but no writer
// access on the linked committee cannot read the diff (which includes committee roster PII).
func TestComputeSyncDiff_AccessDenied(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(&stubAccessChecker{allow: false}),
	)

	_, err := o.ComputeSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}

// TestComputeSyncDiff_NoPrincipal pins that a caller with no principal in context (e.g. a
// missing/misconfigured auth middleware) is rejected before any access check is attempted.
func TestComputeSyncDiff_NoPrincipal(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)

	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}

func TestStartSyncDiff_NoLinkedCommittee(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.StartSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestStartSyncDiff_AccessDenied(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(&stubAccessChecker{allow: false}),
	)
	_, err := o.StartSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}

// TestStartSyncDiff_ReturnsJobID pins the happy path: it returns ITX's job ID directly without
// polling for completion, unlike ComputeSyncDiff.
func TestStartSyncDiff_ReturnsJobID(t *testing.T) {
	stub := &stubITXSyncClient{postDiffJobID: "job-1"}
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(stub),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	jobID, err := o.StartSyncDiff(ctxWithPrincipal(), "ml-1")
	require.NoError(t, err)
	assert.Equal(t, "job-1", jobID)
	assert.Equal(t, port.SyncJobDirectionCommitteeToMailingList, stub.postDiffDirection)
}

func TestStartSyncDiff_PostSyncDiffError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{postDiffErr: errs.NewServiceUnavailable("itx down")}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.StartSyncDiff(ctxWithPrincipal(), "ml-1")
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
}

func TestGetSyncDiffStatus_NoLongerLinked(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.GetSyncDiffStatus(ctxWithPrincipal(), "ml-1", "job-1")
	require.Error(t, err)
	var conflict errs.Conflict
	assert.True(t, errors.As(err, &conflict))
}

func TestGetSyncDiffStatus_AccessDenied(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
		WithSyncDiffAccessChecker(&stubAccessChecker{allow: false}),
	)
	_, err := o.GetSyncDiffStatus(ctxWithPrincipal(), "ml-1", "job-1")
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}

// TestGetSyncDiffStatus_Ongoing pins that an in-progress job returns progress counts without a
// Diff, and without requiring the job to report a CommitteeID.
func TestGetSyncDiffStatus_Ongoing(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			diffJob: &port.SyncDiffJob{ID: "job-1", Status: port.SyncJobStatusOngoing, ProcessedCount: 5, TotalCount: 10},
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	status, err := o.GetSyncDiffStatus(ctxWithPrincipal(), "ml-1", "job-1")
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusOngoing, status.Status)
	assert.Equal(t, 5, status.ProcessedCount)
	assert.Equal(t, 10, status.TotalCount)
	assert.Nil(t, status.Diff)
}

// TestGetSyncDiffStatus_Completed pins that a completed job's diff is translated and attached.
func TestGetSyncDiffStatus_Completed(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			diffJob: &port.SyncDiffJob{
				ID:        "job-1",
				Status:    port.SyncJobStatusCompleted,
				Direction: port.SyncJobDirectionCommitteeToMailingList,
				ToAdd: []port.SyncJobMember{
					{MemberID: "cm-2", Email: "bob@example.com", FullName: "Bob"},
				},
			},
		}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	status, err := o.GetSyncDiffStatus(ctxWithPrincipal(), "ml-1", "job-1")
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusCompleted, status.Status)
	require.NotNil(t, status.Diff)
	require.Len(t, status.Diff.Adds, 1)
	assert.Equal(t, "bob@example.com", status.Diff.Adds[0].Email)
	assert.Equal(t, "committee-1", status.Diff.CommitteeUID)
}

func TestGetSyncDiffStatus_GetSyncDiffJobError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{diffJobErr: errors.New("itx unreachable")}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	_, err := o.GetSyncDiffStatus(ctxWithPrincipal(), "ml-1", "job-1")
	require.Error(t, err)
}

// TestGetSyncDiffStatus_RejectsPathTraversalJobID pins that jobID is validated with
// isSafePathSegment before it is proxied into an ITX URL path segment — sync-diff has no
// server-side job binding (unlike sync-apply) to catch a cross-mailing-list job_id, so this guard
// is the only thing standing between a malicious job_id and itxSyncClient.GetSyncDiffJob reaching
// another mailing list's job (per @copilot-pull-request-reviewer).
func TestGetSyncDiffStatus_RejectsPathTraversalJobID(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{diffJob: &port.SyncDiffJob{ID: "job-1", Status: port.SyncJobStatusCompleted}}),
		WithSyncDiffAccessChecker(mock.NewMockAccessChecker()),
	)
	for _, jobID := range []string{
		"../../other-list/sync-diff/known-job",
		"..",
		"foo/bar",
		"foo%2f..%2fbar",
		"",
	} {
		_, err := o.GetSyncDiffStatus(ctxWithPrincipal(), "ml-1", jobID)
		require.Error(t, err, "jobID=%q", jobID)
		var validation errs.Validation
		assert.True(t, errors.As(err, &validation), "jobID=%q", jobID)
	}
}
