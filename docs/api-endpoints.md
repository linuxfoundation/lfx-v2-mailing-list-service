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

### GroupsIO Members

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/groupsio/mailing-lists/{subgroup_id}/members` | JWT | List members of a mailing list |
| `POST` | `/groupsio/mailing-lists/{subgroup_id}/members` | JWT | Add a member to a mailing list |
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
| `GET` | `/openapi.json` | None | OpenAPI 2.0 (JSON) |
| `GET` | `/openapi3.json` | None | OpenAPI 3.0 (JSON) |
| `GET` | `/openapi.yaml` | None | OpenAPI 2.0 (YAML) |
| `GET` | `/openapi3.yaml` | None | OpenAPI 3.0 (YAML) |

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
```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","member_type":"committee"}' \
  "$BASE/groupsio/mailing-lists/<subgroup-id>/members"
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
