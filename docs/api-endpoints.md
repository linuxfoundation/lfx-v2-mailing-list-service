# API Endpoints

All endpoints are served on port `8080` and require a JWT `Authorization: Bearer <token>` header unless noted.

Base URL (local): `http://localhost:8080`

## Endpoint Summary

### Health

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/livez` | None | Liveness probe — returns `OK` |
| `GET` | `/readyz` | None | Readiness probe — returns `OK` or `503` |

### GroupsIO Services

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/groupsio/services` | JWT | List services, optionally filtered by `?project_uid=<uuid>` |
| `POST` | `/groupsio/services` | JWT | Create a service |
| `GET` | `/groupsio/services/{service_id}` | JWT | Get a service by ID |
| `PUT` | `/groupsio/services/{service_id}` | JWT | Update a service |
| `DELETE` | `/groupsio/services/{service_id}` | JWT | Delete a service |
| `GET` | `/groupsio/services/_projects` | JWT | List projects that have GroupsIO services |
| `GET` | `/groupsio/services/find_parent?project_uid=<uuid>` | JWT | Find the parent service for a project |

### GroupsIO Mailing Lists

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/groupsio/mailing-lists` | JWT | List mailing lists, filtered by `?project_uid=<uuid>` and/or `?committee_uid=<uuid>` |
| `POST` | `/groupsio/mailing-lists` | JWT | Create a mailing list |
| `GET` | `/groupsio/mailing-lists/{subgroup_id}` | JWT | Get a mailing list by ID |
| `PUT` | `/groupsio/mailing-lists/{subgroup_id}` | JWT | Update a mailing list |
| `DELETE` | `/groupsio/mailing-lists/{subgroup_id}` | JWT | Delete a mailing list |
| `GET` | `/groupsio/mailing-lists/count?project_uid=<uuid>` | JWT | Get mailing list count for a project |
| `GET` | `/groupsio/mailing-lists/{subgroup_id}/member_count` | JWT | Get member count for a mailing list |
| `GET` | `/groupsio/mailing-lists/{subgroup_id}/sync-diff` | JWT (writer) | Compute the membership reconciliation diff between a mailing list and its linked committee (`auto_c_to_ml` or `auto_ml_to_c`) |
| `POST` | `/groupsio/mailing-lists/{subgroup_id}/sync-apply` | JWT (writer) | Apply an admin-approved subset of a sync-diff's adds/removes (`auto_c_to_ml` or `auto_ml_to_c`) |

### GroupsIO Members

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/groupsio/mailing-lists/{subgroup_id}/members` | JWT | List members of a mailing list |
| `POST` | `/groupsio/mailing-lists/{subgroup_id}/members` | JWT (writer) or JWT (self, public list only) | Add a member to a mailing list — see "Add a member" under GroupsIO Members below for the writer-vs-self-service distinction |
| `GET` | `/groupsio/mailing-lists/{subgroup_id}/members/{member_id}` | JWT | Get a member by ID |
| `PUT` | `/groupsio/mailing-lists/{subgroup_id}/members/{member_id}` | JWT | Update a member |
| `DELETE` | `/groupsio/mailing-lists/{subgroup_id}/members/{member_id}` | JWT | Remove a member |
| `POST` | `/groupsio/mailing-lists/{subgroup_id}/invitemembers` | JWT | Invite members by email |

### GroupsIO Artifacts

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/groupsio/mailing-lists/{subgroup_id}/artifacts/{artifact_id}` | JWT | Get artifact metadata |
| `GET` | `/groupsio/mailing-lists/{subgroup_id}/artifacts/{artifact_id}/download` | JWT | Get a presigned S3 download URL (expires in 15 min) |

### Utilities

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/groupsio/checksubscriber` | JWT | Check if an email is subscribed to a mailing list |

### OpenAPI Specs

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/_groupsio/openapi.json` | None | OpenAPI 2.0 (JSON) |
| `GET` | `/_groupsio/openapi3.json` | None | OpenAPI 3.0 (JSON) |
| `GET` | `/_groupsio/openapi.yaml` | None | OpenAPI 2.0 (YAML) |
| `GET` | `/_groupsio/openapi3.yaml` | None | OpenAPI 3.0 (YAML) |

---

## Examples

All examples use mock auth. Any non-empty Bearer token is accepted when `AUTH_SOURCE=mock`.

```bash
TOKEN="test-token"
BASE="http://localhost:8080"
```

### Health Checks

```bash
curl $BASE/livez
# OK

curl $BASE/readyz
# OK
```

### GroupsIO Services

**List services for a project:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/services?project_uid=<project-uuid>"
```

**Get a service:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/services/<service-id>"
```

**Find parent service for a project:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/services/find_parent?project_uid=<project-uuid>"
```

**Create a service:**
```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"project_uid":"<uuid>","type":"v2_primary","domain":"groups.io","prefix":"myorg","status":"active"}' \
  "$BASE/groupsio/services"
```

**Update a service:**
```bash
curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"inactive"}' \
  "$BASE/groupsio/services/<service-id>"
```

**Delete a service:**
```bash
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/services/<service-id>"
# 204 No Content
```

### GroupsIO Mailing Lists

**List mailing lists for a project:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists?project_uid=<project-uuid>"
```

**List mailing lists for a committee:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists?committee_uid=<committee-uuid>"
```

**Get a mailing list:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>"
```

**Get mailing list count for a project:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/count?project_uid=<project-uuid>"
```

**Get member count for a mailing list:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/member_count"
```

**Create a mailing list:**
```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"project_uid":"<uuid>","group_name":"my-list","description":"My list","type":"private","audience_access":"member"}' \
  "$BASE/groupsio/mailing-lists"
```

**Create a mailing list with automatic committee membership sync:**

`sync_mode` requires `committee_uid` to also be set, and must be one of `manual` (default),
`auto_c_to_ml`, or `auto_ml_to_c`. ITX is the system of record for this value — it is proxied
through on the wire, not persisted locally — and the sync-diff and sync-apply endpoints below use
it to decide how to reconcile membership.

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"project_uid":"<uuid>","service_id":"<service-uuid>","committee_uid":"<committee-uuid>","name":"my-list","description":"My list","type":"private","audience_access":"member","sync_mode":"auto_c_to_ml"}' \
  "$BASE/groupsio/mailing-lists"
```

**Update a mailing list:**
```bash
curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"description":"Updated description"}' \
  "$BASE/groupsio/mailing-lists/<subgroup-id>"
```

**Delete a mailing list:**
```bash
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>"
# 204 No Content
```

**Compute the sync-diff for a mailing list linked to a committee:**

Supported for mailing lists configured with `sync_mode: auto_c_to_ml` or `auto_ml_to_c`.
Delegates to ITX's sync-diff job (ITX computes and owns the reconciliation logic; this endpoint
posts the job, polls it to completion, and translates the result) and returns members missing
from the target roster (`adds`) and members who no longer belong on it (`removes`): for
`auto_c_to_ml`, `adds` are committee members and `removes` are mailing-list members; for
`auto_ml_to_c` it is the reverse. Each add/remove entry's `member_type` says which roster its
`member_id` belongs to. `conflicts` lists email collisions between the two rosters that ITX could
not confidently resolve into an add or remove. Returns `400 Bad Request` if the mailing list has
no linked committee, or its `sync_mode` is `manual`. Returns `503 Service Unavailable` if the
underlying ITX job fails or does not complete within the poll timeout.

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/sync-diff"
```

```json
{
  "mailing_list_id": "<subgroup-id>",
  "committee_uid": "<committee-uuid>",
  "sync_mode": "auto_c_to_ml",
  "adds": [
    {"email": "new-member@example.com", "member_id": "<committee-member-uuid>", "member_type": "committee_member"}
  ],
  "removes": [
    {"email": "departed-member@example.com", "member_id": "<mailing-list-member-id>", "member_type": "mailing_list_member"}
  ],
  "conflicts": [
    {"committee_email": "jane@old-co.example.com", "mailing_list_email": "jane@new-co.example.com", "name": "Jane Doe"}
  ]
}
```

**Apply an admin-approved subset of a sync-diff:**

Takes the `adds`/`removes` an admin chose to apply from a prior sync-diff response and delegates
to ITX's sync-apply job (this endpoint posts the job, polls it to completion, and translates the
per-entry results). An entry ITX could not apply is reported as `failed` with ITX's error message
rather than silently dropped. Same `sync_mode` and committee-link requirements as sync-diff.

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"adds":[{"email":"new-member@example.com","member_id":"<committee-member-uuid>","member_type":"committee_member"}],"removes":[{"email":"departed-member@example.com","member_id":"<mailing-list-member-id>","member_type":"mailing_list_member"}]}' \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/sync-apply"
```

```json
{
  "mailing_list_id": "<subgroup-id>",
  "committee_uid": "<committee-uuid>",
  "added": [
    {"email": "new-member@example.com", "member_id": "<committee-member-uuid>", "member_type": "committee_member", "outcome": "added"}
  ],
  "removed": [
    {"email": "departed-member@example.com", "member_id": "<mailing-list-member-id>", "member_type": "mailing_list_member", "outcome": "removed"}
  ]
}
```

### GroupsIO Members

**List members:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/members"
```

**Get a member:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/members/<member-id>"
```

**Add a member:**

This endpoint is reachable two ways, and the caller's permissions differ accordingly (see
`authorizeAddGroupsioMember` in `cmd/mailing-list-api/service/endpoint_member.go` and the
`public` relation notes in [`docs/fga-contract.md`](fga-contract.md#public-relation)):

- **Verified writer** — a caller holding the `writer` relation on the list can add any
  email, with any `member_type`/`mod_status`, on any list (public or private).
- **Self-service on a public list** — a caller who is not a writer is only admitted via the
  list's `public` relation, and may only add **their own authenticated email**, with no
  `member_type` or `mod_status` set (both must be empty or their non-privileged default). Any
  mismatch returns `403 Unauthorized`.

A caller that is neither a verified writer nor matches the self-service conditions above gets
`403 Unauthorized`; an fga-sync access-check failure (e.g. a timeout) returns
`503 Service Unavailable` instead, since that's a transient outage, not a denial.

```bash
# Writer adding another member with privileged fields
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","member_type":"committee"}' \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/members"

# Self-service subscription to a public list (caller's own email, no privileged fields)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com"}' \
  "$BASE/groupsio/mailing-lists/<public-subgroup-id>/members"
```

**Update a member:**
```bash
curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"delivery_mode":"digest"}' \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/members/<member-id>"
```

**Remove a member:**
```bash
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/members/<member-id>"
# 204 No Content
```

**Invite members by email:**
```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"emails":["alice@example.com","bob@example.com"]}' \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/invitemembers"
# 204 No Content
```

### GroupsIO Artifacts

**Get artifact metadata:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/artifacts/<artifact-uuid>"
```

**Get presigned download URL:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/artifacts/<artifact-uuid>/download"
# {"url":"https://s3.amazonaws.com/...?X-Amz-Expires=900&..."}
```

### Check Subscriber

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","subgroup_id":"<subgroup-id>"}' \
  "$BASE/groupsio/checksubscriber"
```
