// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a Client pointing at the given base URL, bypassing Auth0.
func newTestClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{},
		config:     Config{BaseURL: baseURL},
	}
}

// ---- mapHTTPError ----

func TestMapHTTPError(t *testing.T) {
	c := newTestClient("http://unused")
	tests := []struct {
		status  int
		errType domain.ErrorType
	}{
		{http.StatusNotFound, domain.ErrorTypeNotFound},
		{http.StatusBadRequest, domain.ErrorTypeValidation},
		{http.StatusConflict, domain.ErrorTypeConflict},
		{http.StatusServiceUnavailable, domain.ErrorTypeUnavailable},
		{http.StatusBadGateway, domain.ErrorTypeUnavailable},
		{http.StatusGatewayTimeout, domain.ErrorTypeUnavailable},
		{http.StatusInternalServerError, domain.ErrorTypeInternal},
		{http.StatusUnprocessableEntity, domain.ErrorTypeInternal},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			err := c.mapHTTPError(tt.status, []byte("detail"))
			var domErr *domain.DomainError
			require.True(t, errors.As(err, &domErr))
			assert.Equal(t, tt.errType, domErr.Type)
		})
	}
}

// ---- service endpoints ----

func TestClient_ListServices(t *testing.T) {
	t.Run("success with no filter", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/groupsio_service", r.URL.Path)
			assert.Empty(t, r.URL.RawQuery)
			assert.Equal(t, itxScope, r.Header.Get("x-scope"))
			writeJSON(w, models.GroupsioServiceListResponse{
				Items: []*models.GroupsioService{{ID: "svc-1", ProjectID: "proj-sfid"}},
				Total: 1,
			})
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		resp, err := c.ListServices(context.Background(), "")
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		assert.Equal(t, "svc-1", resp.Items[0].ID)
	})

	t.Run("success with project_id filter", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "proj-sfid-001", r.URL.Query().Get("project_id"))
			writeJSON(w, models.GroupsioServiceListResponse{Items: []*models.GroupsioService{}, Total: 0})
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		resp, err := c.ListServices(context.Background(), "proj-sfid-001")
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Total)
	})

	t.Run("404 returns not found error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		_, err := c.ListServices(context.Background(), "")
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
	})
}

func TestClient_CreateService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/groupsio_service", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req models.GroupsioServiceRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "proj-sfid", req.ProjectID)

		w.WriteHeader(http.StatusCreated)
		writeJSON(w, models.GroupsioService{ID: "new-svc", ProjectID: req.ProjectID})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.CreateService(context.Background(), &models.GroupsioServiceRequest{ProjectID: "proj-sfid"})
	require.NoError(t, err)
	assert.Equal(t, "new-svc", resp.ID)
}

func TestClient_GetService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/groupsio_service/svc-42", r.URL.Path)
		writeJSON(w, models.GroupsioService{ID: "svc-42"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GetService(context.Background(), "svc-42")
	require.NoError(t, err)
	assert.Equal(t, "svc-42", resp.ID)
}

func TestClient_UpdateService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/groupsio_service/svc-42", r.URL.Path)
		writeJSON(w, models.GroupsioService{ID: "svc-42", Status: "active"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.UpdateService(context.Background(), "svc-42", &models.GroupsioServiceRequest{Status: "active"})
	require.NoError(t, err)
	assert.Equal(t, "active", resp.Status)
}

func TestClient_DeleteService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/groupsio_service/svc-42", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.DeleteService(context.Background(), "svc-42")
	require.NoError(t, err)
}

func TestClient_GetProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groupsio_service/_projects", r.URL.Path)
		writeJSON(w, models.GroupsioServiceProjectsResponse{Projects: []string{"proj-a", "proj-b"}})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GetProjects(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"proj-a", "proj-b"}, resp.Projects)
}

func TestClient_FindParentService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groupsio_service_find_parent", r.URL.Path)
		assert.Equal(t, "proj-sfid-001", r.URL.Query().Get("project_id"))
		writeJSON(w, models.GroupsioService{ID: "parent-svc"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.FindParentService(context.Background(), "proj-sfid-001")
	require.NoError(t, err)
	assert.Equal(t, "parent-svc", resp.ID)
}

// ---- subgroup endpoints ----

func TestClient_ListSubgroups(t *testing.T) {
	t.Run("both filters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/groupsio_subgroup", r.URL.Path)
			assert.Equal(t, "proj-sfid", r.URL.Query().Get("project_id"))
			assert.Equal(t, "comm-sfid", r.URL.Query().Get("committee_id"))
			writeJSON(w, models.GroupsioSubgroupListResponse{
				Items: []*models.GroupsioSubgroup{{ID: "sg-1"}},
				Total: 1,
			})
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		resp, err := c.ListSubgroups(context.Background(), "proj-sfid", "comm-sfid")
		require.NoError(t, err)
		assert.Len(t, resp.Items, 1)
	})

	t.Run("no filters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.URL.RawQuery)
			writeJSON(w, models.GroupsioSubgroupListResponse{})
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		_, err := c.ListSubgroups(context.Background(), "", "")
		require.NoError(t, err)
	})

	t.Run("project filter only", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "proj-sfid", r.URL.Query().Get("project_id"))
			assert.Empty(t, r.URL.Query().Get("committee_id"))
			writeJSON(w, models.GroupsioSubgroupListResponse{})
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		_, err := c.ListSubgroups(context.Background(), "proj-sfid", "")
		require.NoError(t, err)
	})
}

func TestClient_CreateSubgroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/groupsio_subgroup", r.URL.Path)
		writeJSON(w, models.GroupsioSubgroup{ID: "sg-new", Name: "test-list"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.CreateSubgroup(context.Background(), &models.GroupsioSubgroupRequest{Name: "test-list"})
	require.NoError(t, err)
	assert.Equal(t, "sg-new", resp.ID)
}

func TestClient_GetSubgroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groupsio_subgroup/sg-42", r.URL.Path)
		writeJSON(w, models.GroupsioSubgroup{ID: "sg-42"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GetSubgroup(context.Background(), "sg-42")
	require.NoError(t, err)
	assert.Equal(t, "sg-42", resp.ID)
}

func TestClient_UpdateSubgroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/groupsio_subgroup/sg-42", r.URL.Path)
		writeJSON(w, models.GroupsioSubgroup{ID: "sg-42", Name: "updated"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.UpdateSubgroup(context.Background(), "sg-42", &models.GroupsioSubgroupRequest{Name: "updated"})
	require.NoError(t, err)
	assert.Equal(t, "updated", resp.Name)
}

func TestClient_DeleteSubgroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/groupsio_subgroup/sg-42", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.DeleteSubgroup(context.Background(), "sg-42")
	require.NoError(t, err)
}

func TestClient_GetSubgroupCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groupsio/subgroup_count", r.URL.Path)
		assert.Equal(t, "proj-sfid", r.URL.Query().Get("project"))
		writeJSON(w, models.GroupsioSubgroupCountResponse{Count: 5})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GetSubgroupCount(context.Background(), "proj-sfid")
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Count)
}

func TestClient_GetMemberCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groupsio_subgroup/sg-42/member_count", r.URL.Path)
		writeJSON(w, models.GroupsioMemberCountResponse{Count: 12})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GetMemberCount(context.Background(), "sg-42")
	require.NoError(t, err)
	assert.Equal(t, 12, resp.Count)
}

// ---- member endpoints ----

func TestClient_ListMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groupsio_subgroup/sg-42/members", r.URL.Path)
		writeJSON(w, models.GroupsioMemberListResponse{
			Items: []*models.GroupsioMember{{ID: "m-1", Email: "a@example.com"}},
			Total: 1,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.ListMembers(context.Background(), "sg-42")
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "a@example.com", resp.Items[0].Email)
}

func TestClient_AddMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/groupsio_subgroup/sg-42/members", r.URL.Path)
		writeJSON(w, models.GroupsioMember{ID: "m-new", Email: "new@example.com"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.AddMember(context.Background(), "sg-42", &models.GroupsioMemberRequest{Email: "new@example.com"})
	require.NoError(t, err)
	assert.Equal(t, "m-new", resp.ID)
}

func TestClient_GetMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groupsio_subgroup/sg-42/members/m-7", r.URL.Path)
		writeJSON(w, models.GroupsioMember{ID: "m-7"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GetMember(context.Background(), "sg-42", "m-7")
	require.NoError(t, err)
	assert.Equal(t, "m-7", resp.ID)
}

func TestClient_UpdateMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/groupsio_subgroup/sg-42/members/m-7", r.URL.Path)
		writeJSON(w, models.GroupsioMember{ID: "m-7", Name: "Updated Name"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.UpdateMember(context.Background(), "sg-42", "m-7", &models.GroupsioMemberRequest{Name: "Updated Name"})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.Name)
}

func TestClient_DeleteMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/groupsio_subgroup/sg-42/members/m-7", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.DeleteMember(context.Background(), "sg-42", "m-7")
	require.NoError(t, err)
}

func TestClient_InviteMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/groupsio_subgroup/sg-42/invitemembers", r.URL.Path)

		var req models.GroupsioInviteMembersRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, []string{"a@example.com", "b@example.com"}, req.Emails)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.InviteMembers(context.Background(), "sg-42", &models.GroupsioInviteMembersRequest{
		Emails: []string{"a@example.com", "b@example.com"},
	})
	require.NoError(t, err)
}

func TestClient_CheckSubscriber(t *testing.T) {
	t.Run("subscribed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/groupsio_checksubscriber", r.URL.Path)
			writeJSON(w, models.GroupsioCheckSubscriberResponse{Subscribed: true})
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		resp, err := c.CheckSubscriber(context.Background(), &models.GroupsioCheckSubscriberRequest{
			Email: "a@example.com", SubgroupID: "sg-42",
		})
		require.NoError(t, err)
		assert.True(t, resp.Subscribed)
	})

	t.Run("not subscribed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, models.GroupsioCheckSubscriberResponse{Subscribed: false})
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		resp, err := c.CheckSubscriber(context.Background(), &models.GroupsioCheckSubscriberRequest{
			Email: "b@example.com", SubgroupID: "sg-42",
		})
		require.NoError(t, err)
		assert.False(t, resp.Subscribed)
	})
}

func TestClient_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GetService(context.Background(), "svc-1")
	require.Error(t, err)
	assert.Equal(t, domain.ErrorTypeInternal, domain.GetErrorType(err))
	assert.True(t, strings.Contains(err.Error(), "failed to parse response"))
}

// writeJSON is a test helper that writes v as JSON to w.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}
