// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This fixture follows getGroupsioSubgroups' committee response, including numeric
// group_id (the mailing list UID) and the distinct query/record committee field names.
const linkedSubgroupsResponse = `{"data":[
	{"group_id":101,"group_name":"test-auto","project_id":"project-v1","parent_id":"service-1","committee":"committee-v1","sync_mode":"auto_c_to_ml","committee_filters":["voting_rep"],"visibility":"public"},
	{"group_id":102,"group_name":"test-manual","project_id":"other-project-v1","committee":"committee-v1","sync_mode":"manual"}
],"meta":{"total_results":2,"total_pages":1,"per_page":2,"page_token":""}}`

func newMailingListTestProxy(t *testing.T, handler http.HandlerFunc) *itx {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &itx{
		config:     Config{BaseURL: server.URL},
		httpClient: httpclient.NewClientWithHTTPClient(httpclient.Config{}, server.Client()),
	}
}

func TestITXListMailingLists_SelectorsAndResponse(t *testing.T) {
	tests := []struct {
		name, projectID, committeeID string
		query                        url.Values
		wantUIDs                     []string
		wantTotal                    int
	}{
		{"committee only", "", "committee-v1", url.Values{"committee_id": {"committee-v1"}}, []string{"101", "102"}, 2},
		{"project only", "project-v1", "", url.Values{"project_id": {"project-v1"}}, []string{"101", "102"}, 2},
		{"both filters", "project-v1", "committee-v1", url.Values{"committee_id": {"committee-v1"}}, []string{"101"}, 1},
		{"project mismatch", "absent-project", "committee-v1", url.Values{"committee_id": {"committee-v1"}}, []string{}, 0},
		{"escaped committee", "", "committee +/&", url.Values{"committee_id": {"committee +/&"}}, []string{"101", "102"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := newMailingListTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/v2/groupsio_subgroup", r.URL.Path)
				assert.Equal(t, tt.query, r.URL.Query(), "ITX requires exactly one supported selector")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(linkedSubgroupsResponse))
			})
			items, total, err := client.ListMailingLists(context.Background(), tt.projectID, tt.committeeID)
			require.NoError(t, err)
			assert.Equal(t, 1, calls)
			assert.Equal(t, tt.wantTotal, total)
			uids := make([]string, 0, len(items))
			for _, item := range items {
				uids = append(uids, item.UID)
			}
			assert.Equal(t, tt.wantUIDs, uids)
			if len(items) > 0 {
				assert.Equal(t, "project-v1", items[0].ProjectUID)
				assert.Equal(t, "service-1", items[0].ServiceUID)
				require.Len(t, items[0].Committees, 1)
				assert.Equal(t, "committee-v1", items[0].Committees[0].UID)
				assert.Equal(t, "auto_c_to_ml", items[0].Committees[0].SyncMode)
				assert.Equal(t, []string{"voting_rep"}, items[0].Committees[0].AllowedVotingStatuses)
			}
		})
	}
}

func TestITXListMailingLists_NoSelector(t *testing.T) {
	client := newMailingListTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("missing selectors must be rejected before making an HTTP request")
	})
	items, total, err := client.ListMailingLists(context.Background(), "", "")
	require.ErrorContains(t, err, "a project or committee filter is required")
	assert.Nil(t, items)
	assert.Zero(t, total)
}

func TestITXListMailingLists_ResponseContract(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		wantErr    string
	}{
		{"unlinked committee", `{"data":[],"meta":{"total_results":0}}`, http.StatusOK, ""},
		{"unknown envelope", `{"items":[{"group_id":101}],"total":1}`, http.StatusOK, "expected data and meta.total_results"},
		{"wrong metadata key", `{"data":[{"group_id":101}],"metadata":{"total_results":1}}`, http.StatusOK, "expected data and meta.total_results"},
		{"missing metadata", `{"data":[]}`, http.StatusOK, "expected data and meta.total_results"},
		{"missing count", `{"data":[],"meta":{}}`, http.StatusOK, "expected data and meta.total_results"},
		{"null data", `{"data":null,"meta":{"total_results":0}}`, http.StatusOK, "expected data and meta.total_results"},
		{"null subgroup", `{"data":[null],"meta":{"total_results":1}}`, http.StatusOK, "null subgroup"},
		{"malformed json", `not json`, http.StatusOK, "failed to parse response"},
		{"upstream validation", `{"message":"invalid committee"}`, http.StatusBadRequest, "bad request"},
		{"upstream failure", `{"message":"unavailable"}`, http.StatusServiceUnavailable, "ITX service unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newMailingListTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			items, total, err := client.ListMailingLists(context.Background(), "", "committee-v1")
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, items)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, items)
				assert.Empty(t, items)
			}
			assert.Zero(t, total)
		})
	}
}

type mailingListContractTranslator struct{}

func (*mailingListContractTranslator) MapID(_ context.Context, subject, direction, id string) (string, error) {
	for _, mapping := range []struct{ subject, v1, v2 string }{
		{constants.TranslationSubjectCommittee, "committee-v1", "committee-v2"},
		{constants.TranslationSubjectProject, "project-v1", "project-v2"},
		{constants.TranslationSubjectProject, "other-project-v1", "other-project-v2"},
	} {
		if subject == mapping.subject {
			if direction == constants.TranslationDirectionV2ToV1 && id == mapping.v2 {
				return mapping.v1, nil
			}
			if direction == constants.TranslationDirectionV1ToV2 && id == mapping.v1 {
				return mapping.v2, nil
			}
		}
	}
	return "", fmt.Errorf("unexpected mapping: %s %s %s", subject, direction, id)
}

func TestITXListMailingLists_ReaderTranslationContract(t *testing.T) {
	client := newMailingListTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, url.Values{"committee_id": {"committee-v1"}}, r.URL.Query())
		_, _ = w.Write([]byte(linkedSubgroupsResponse))
	})
	reader := service.NewGroupsIOMailingListReaderOrchestrator(
		service.WithMailingListReader(client),
		service.WithMailingListReaderTranslator(&mailingListContractTranslator{}),
	)
	items, total, err := reader.ListMailingLists(context.Background(), "", "committee-v2")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, 2, total)
	assert.Equal(t, "101", items[0].UID)
	assert.Equal(t, "project-v2", items[0].ProjectUID)
	assert.Equal(t, "committee-v2", items[0].Committees[0].UID)
	assert.Equal(t, "auto_c_to_ml", items[0].Committees[0].SyncMode)
}
