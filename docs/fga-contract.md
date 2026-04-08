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

Published to `lfx.fga-sync.member_put` when a member event is processed and the member has a non-empty `Username`. The username is resolved to an Auth0 `sub` value via `principal.FromUsername` before sending.

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
| `username` | Auth0 `sub` of the member | Always (skipped if `Username` is empty) |
| `relations` | `["member"]` | Always |

### member_remove (Member Delete)

Published to `lfx.fga-sync.member_remove` when a member delete event is processed and the stored mapping contains a non-empty username. The username is resolved to an Auth0 `sub` value via `principal.FromUsername` before sending.

The object UID is the **parent mailing list UID**, recovered from the stored member mapping (`uid|username|mailingListUID`).

| Field | Value |
|---|---|
| `object_type` | `groupsio_mailing_list` |
| `uid` | `MailingListUID` (parent mailing list) |
| `username` | Auth0 `sub` of the member |
| `relations` | `[]` (empty — removes all relations for the user) |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the mailing list `uid` — all FGA tuples for `groupsio_mailing_list:{uid}` are removed by the fga-sync service.

---

## Triggers

| Operation | Object Type | Subject | Notes |
|---|---|---|---|
| Create/update GroupsIO service | `groupsio_service` | `lfx.fga-sync.update_access` | Always sent |
| Delete GroupsIO service | `groupsio_service` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update mailing list | `groupsio_mailing_list` | `lfx.fga-sync.update_access` | Always sent |
| Delete mailing list | `groupsio_mailing_list` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update member (with username) | `groupsio_mailing_list` | `lfx.fga-sync.member_put` | Skipped if `Username` is empty |
| Delete member (with username) | `groupsio_mailing_list` | `lfx.fga-sync.member_remove` | Skipped if stored mapping has no username |
