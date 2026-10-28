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
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

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
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_ManualSyncMode(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeManual)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_EmptySyncModeTreatedAsManual(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommittee("committee-1")}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_UnrecognizedSyncMode(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", "bogus")}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_MailingListReaderError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{err: errors.New("not found")}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
}

func TestComputeSyncDiff_PostSyncDiffError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{postDiffErr: errs.NewServiceUnavailable("itx down")}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
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
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
}

func TestComputeSyncDiff_JobFailed(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffITXSyncClient(&stubITXSyncClient{
			postDiffJobID: "job-1",
			diffJob:       &port.SyncDiffJob{ID: "job-1", Status: port.SyncJobStatusFailed, ErrorMessage: "subgroup not found"},
		}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
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
	)

	diff, err := o.ComputeSyncDiff(context.Background(), "ml-1")
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
	)

	diff, err := o.ComputeSyncDiff(context.Background(), "ml-1")
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
	)

	diff, err := o.ComputeSyncDiff(context.Background(), "ml-1")
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
	)

	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var conflict errs.Conflict
	assert.True(t, errors.As(err, &conflict))
}
