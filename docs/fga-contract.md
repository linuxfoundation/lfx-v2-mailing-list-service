# FGA Contract — Mailing List Service

This document is the authoritative reference for all messages the mailing list service sends to the fga-sync service, which writes and deletes [OpenFGA](https://openfga.dev/) relationship tuples to enforce access control.

The full OpenFGA type definitions (relations, schema) for all object types are defined in the [platform model](https://github.com/linuxfoundation/lfx-v2-helm/blob/main/charts/lfx-platform/templates/openfga/model.yaml).

**Update this document in the same PR as any change to FGA message construction.**

---

## Object Types

- [GroupsIO Service](#groupsio-service)
- [GroupsIO Mailing List](#groupsio-mailing-list)

---

## Message Format

This service uses four FGA operation types:

| Subject | Operation | Used for |
|---|---|---|
| `lfx.fga-sync.update_access` | `update_access` | Create and update — sets object-level access config and references |
| `lfx.fga-sync.member_put` | `member_put` | Adds a user to one or more relations on an object |
| `lfx.fga-sync.member_remove` | `member_remove` | Removes a user from an object; sent on member delete. An empty `relations` array removes all relations for that user on the object |
| `lfx.fga-sync.delete_access` | `delete_access` | Delete — removes all FGA tuples for the object |

---

## GroupsIO Service

**Source struct:** `internal/domain/model/` — `GroupsIOService` (base + settings)

**Synced on:** create, update, delete of a GroupsIO service.

### update_access

Published to `lfx.fga-sync.update_access` on service create or update.

#### Message Envelope

| Field | Value |
|---|---|
| `object_type` | `groupsio_service` |
| `operation` | `update_access` |

#### Data Fields

These fields are carried inside the message `data` object.

| Field | Value |
|---|---|
| `uid` | Service UID |
| `public` | `GroupsIOService.Public` (passed through directly) |

#### Relations

| Relation | Value | Condition |
|---|---|---|
| `writer` | Usernames from `GroupsIOServiceSettings.Writers` | Only when `Writers` is non-empty |
| `auditor` | Usernames from `GroupsIOServiceSettings.Auditors` | Only when `Auditors` is non-empty |

> Usernames are extracted from the `Username` pointer of each `UserInfo` entry. Users with a nil or empty `Username` are skipped.

#### References

| Reference | Value | Condition |
|---|---|---|
| `project` | `GroupsIOService.ProjectUID` | Always |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the service `uid` — all FGA tuples for `groupsio_service:{uid}` are removed by the fga-sync service.

---

## GroupsIO Mailing List

**Source struct:** `internal/domain/model/` — `GroupsIOMailingList` (base + settings)

**Synced on:** create, update, delete of a GroupsIO mailing list (subgroup). Member changes are synced separately via `member_put` and `member_remove`.

### update_access

Published to `lfx.fga-sync.update_access` on mailing list create or update.

#### Message Envelope

| Field | Value |
|---|---|
| `object_type` | `groupsio_mailing_list` |
| `operation` | `update_access` |

#### Data Fields

These fields are carried inside the message `data` object.

| Field | Value |
|---|---|
| `uid` | Mailing list UID |
| `public` | `GroupsIOMailingList.Public` (passed through directly) |

#### Relations

| Relation | Value | Condition |
|---|---|---|
| `writer` | Usernames from `GroupsIOMailingListSettings.Writers` | Only when `Writers` is non-empty |
| `auditor` | Usernames from `GroupsIOMailingListSettings.Auditors` | Only when `Auditors` is non-empty |

> Usernames are extracted from the `Username` pointer of each `UserInfo` entry. Users with a nil or empty `Username` are skipped.

#### References

| Reference | Value | Condition |
|---|---|---|
| `groupsio_service` | `GroupsIOMailingList.ServiceUID` | Always |
| `committee` | `CommitteeUID` per committee | One entry per committee with a non-empty `UID` |

#### Exclude Relations

`exclude_relations: ["member"]` — always set. Individual mailing list members are managed via `member_put` and `member_remove` and must not be overwritten by the `update_access` handler.

### member_put (Member Create/Update)

Published to `lfx.fga-sync.member_put` when a member event is processed and the member has a non-empty `Username`.

The object UID is the **parent mailing list UID**, not the member UID. The parent is resolved from the `group_id` → mailing list reverse index.

#### Message Envelope

| Field | Value |
|---|---|
| `object_type` | `groupsio_mailing_list` |
| `operation` | `member_put` |

#### Data Fields

| Field | Value | Condition |
|---|---|---|
| `uid` | `MailingListUID` (parent mailing list) | Always |
| `username` | LFX username (from v1 `username` field) | Always (skipped if `Username` is empty) |
| `relations` | `["member"]` | Always |

### member_remove (Member Delete)

Published to `lfx.fga-sync.member_remove` when a member delete event is processed and the stored mapping contains a non-empty username.

The object UID is the **parent mailing list UID**, recovered from the stored member mapping (`uid|username|mailingListUID`).

| Field | Value |
|---|---|
| `object_type` | `groupsio_mailing_list` |
| `uid` | `MailingListUID` (parent mailing list) |
| `username` | LFX username (from stored member mapping) |
| `relations` | `[]` (empty — removes all relations for the user) |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the mailing list `uid` — all FGA tuples for `groupsio_mailing_list:{uid}` are removed by the fga-sync service.

---

## Access Checks

In addition to the publish-only operations above, the mailing list API makes synchronous
ad hoc relationship checks against fga-sync for authorization decisions it can't make from
Heimdall's routing alone (e.g. distinguishing a verified writer from a public-branch caller
in `authorizeAddGroupsioMember`, see `cmd/mailing-list-api/service/endpoint_member.go`).

**Subject:** `lfx.access_check.request` (`pkg/constants.AccessCheckSubject`), NATS request/reply.

**Request:** a single line `{object}#{relation}@{user}` (e.g.
`groupsio_mailing_list:ml-1#writer@user:alice`), terminated with `\n`. `object`, `relation`,
and `user` must each be non-empty and must not contain `#`, `@`, a tab, or a newline —
`NATSAccessChecker.HasRelation` validates this before sending, since any of those characters
appearing in a caller-supplied value (e.g. `object`'s subgroup ID) could split or alter the
line fga-sync parses.

**Reply:** the request line echoed back, followed by a tab and `true` or `false`.

**Failure handling:** a request error, a malformed reply, or a timeout (5s) is treated as a
service failure, not a denial — callers map it to `ServiceUnavailable` (HTTP 503), not
`Unauthorized` (HTTP 403), so a transient fga-sync outage is distinguishable from a real
access denial.

**Local/dev:** `ACCESS_CHECK_SOURCE` selects the backend — `nats` (default) talks to
fga-sync; `mock` always returns `true` and requires no live NATS reply subscriber, matching
the `mock` option already available for `AUTH_SOURCE`, `TRANSLATOR_SOURCE`, and
`REPOSITORY_SOURCE`.

### `public` Relation

The `public` relation (see `pkg/constants.RelationPublic`) marks an object as having
public, self-service access, independent of `writer`/`auditor`. It is set via the `public`
data field documented above and is never implied by `writer` or `auditor` — callers that
need to gate a self-service action (e.g. self-subscription) on public access must check
`public` directly via an access check rather than inferring it from the other relations.

**Rollout note:** this relation is only published for a `groupsio_mailing_list` object when
it is created or updated (see `internal/service/datastream_subgroup_handler.go`). Existing
public mailing lists synced before the OpenFGA model gained the `public` relation (see
`linuxfoundation/lfx-v2-helm#189`) do not get it retroactively — their access message must
be republished after that model deploy. This is tracked for a `scripts/reindex_mailing_lists`
extension; see `linuxfoundation/lfx-self-serve-ops#954` (issues are disabled on this repo).

## Triggers

| Operation | Object Type | Subject | Notes |
|---|---|---|---|
| Create/update GroupsIO service | `groupsio_service` | `lfx.fga-sync.update_access` | Always sent |
| Delete GroupsIO service | `groupsio_service` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update mailing list | `groupsio_mailing_list` | `lfx.fga-sync.update_access` | Always sent |
| Delete mailing list | `groupsio_mailing_list` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update member (with username) | `groupsio_mailing_list` | `lfx.fga-sync.member_put` | Skipped if `Username` is empty |
| Delete member (with username) | `groupsio_mailing_list` | `lfx.fga-sync.member_remove` | Skipped if stored mapping has no username |
