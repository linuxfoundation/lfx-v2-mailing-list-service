# Indexer Contract — Mailing List Service

This document is the authoritative reference for all data the mailing list service sends to the indexer service, which makes resources searchable via the [query service](https://github.com/linuxfoundation/lfx-v2-query-service).

**Update this document in the same PR as any change to indexer message construction.**

---

## Resource Types

- [GroupsIO Service](#groupsio-service)
- [GroupsIO Service Settings](#groupsio-service-settings)
- [GroupsIO Mailing List](#groupsio-mailing-list)
- [GroupsIO Mailing List Settings](#groupsio-mailing-list-settings)
- [GroupsIO Member](#groupsio-member)
- [GroupsIO Artifact](#groupsio-artifact)

---

## GroupsIO Service

**Object type:** `groupsio_service`

**Source struct:** `internal/domain/model/grpsio_service.go` — `GroupsIOService`

**NATS subject:** `lfx.index.groupsio_service`

**Indexed on:** create, update, delete of a GroupsIO service (v1 datastream via `datastream_service_handler.go`).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Service unique identifier |
| `type` | string | Service type (`primary`, `formation`, `shared`) |
| `domain` | string | Groups.io domain (e.g. `groups.io`) |
| `group_id` | int64 (optional) | Groups.io numeric group ID |
| `status` | string | Service status; emitted as empty string when not populated |
| `source` | string | Source system identifier; always `"v1-sync"` for v1 datastream records |
| `prefix` | string | Groups.io group name prefix; emitted as empty string when not populated |
| `global_owners` | []string | Global owner list; always emitted as `null`/empty array by v1-sync (not populated by transform) |
| `parent_service_uid` | string | UID of the parent service for shared type; emitted as empty string by v1-sync |
| `project_uid` | string | v2 UID of the owning project (resolved from v1 SFID) |
| `project_slug` | string | Slug of the owning project; emitted as empty string when not populated |
| `project_name` | string | Name of the owning project; emitted as empty string when not populated |
| `url` | string | Groups.io URL for the service group; emitted as empty string when not populated |
| `group_name` | string | Groups.io group name; emitted as empty string when not populated |
| `public` | bool | Whether the service is publicly accessible; emitted as `false` when not populated |
| `created_at` | timestamp | Creation time (RFC3339) |
| `updated_at` | timestamp | Last update time (RFC3339) |
| `system_updated_at` | timestamp (optional) | Last modified by a system process |

> **v1-sync transform note:** `transformV1ToGrpsIOService` populates `uid`, `type`, `domain`, `group_id`, `prefix`, `project_uid`, `project_slug`, `source` ("v1-sync"), and timestamps. All other fields (`status`, `global_owners`, `parent_service_uid`, `project_name`, `url`, `group_name`, `public`) are at their Go zero values and will be serialized as empty strings / `false` / `null`.

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{uid}` | `abc123` | Direct lookup by UID |
| `service_uid:{uid}` | `service_uid:abc123` | Namespaced lookup by UID |
| `project_uid:{value}` | `project_uid:bb4ed8c8-...` | Find services for a project |
| `project_slug:{value}` | `project_slug:my-project` | Find services by project slug |
| `service_type:{value}` | `service_type:primary` | Find services by type |

> All tags are only emitted when the value is non-empty.

### Access Control (AccessMessage)

Published to `lfx.fga-sync.update_access` on create/update. Deleted via `lfx.fga-sync.delete_access` on delete.

| Field | Value |
|---|---|
| `object_type` | `groupsio_service` |
| `public` | value of `GroupsIOService.Public` |
| `references.project` | `[project_uid]` |
| `references.writer` | usernames from writers (when settings present) |
| `references.auditor` | usernames from auditors (when settings present) |

### Search Behavior (IndexingConfig)

| Field | Value |
|---|---|
| `object_id` | `{uid}` |
| `public` | value of `GroupsIOService.Public` |
| `access_check_object` | `groupsio_service:{uid}` |
| `access_check_relation` | `viewer` |
| `history_check_object` | `groupsio_service:{uid}` |
| `history_check_relation` | `auditor` |
| `sort_name` | `GetGroupName()` falling back to `domain` |
| `name_and_aliases` | `GetGroupName()`, `domain` (non-empty values) |
| `fulltext` | space-joined non-empty values of `GetGroupName()`, `domain`, `prefix`, `type` |

### Parent References

| Ref | Condition |
|---|---|
| `project:{project_uid}` | Only when `project_uid` is set |

---

## GroupsIO Service Settings

**Object type:** `groupsio_service_settings`

**Source struct:** `internal/domain/model/grpsio_service.go` — `GrpsIOServiceSettings`

**NATS subject:** `lfx.index.groupsio_service_settings`

**Indexed on:** create/update of a GroupsIO service when writers or auditors are present. Settings share the same UID as their parent service.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Service UID (same as the parent service) |
| `writers` | []object | Users with write access. Each object has `username` (string, holds the user ID) |
| `auditors` | []object | Users with audit access. Each object has `username` (string, holds the user ID) |
| `last_reviewed_at` | string (optional) | RFC3339 timestamp of the last membership review |
| `last_reviewed_by` | string (optional) | UID of who performed the last review |
| `last_audited_by` | string (optional) | UID of who performed the last audit |
| `last_audited_time` | string (optional) | RFC3339 timestamp of the last audit |
| `created_at` | timestamp | Creation time (RFC3339) |
| `updated_at` | timestamp | Last update time (RFC3339) |

> **v1-sync build note:** `buildServiceSettings` only populates `uid`, `writers`, and `auditors`. The optional review/audit fields and `created_at`/`updated_at` will be at Go zero values (`0001-01-01T00:00:00Z` for timestamps, `null` for optional strings).

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{uid}` | `abc123` | Direct lookup by UID |
| `service_uid:{uid}` | `service_uid:abc123` | Namespaced lookup by UID |

### Search Behavior (IndexingConfig)

| Field | Value |
|---|---|
| `object_id` | `{uid}` |
| `access_check_object` | `groupsio_service:{uid}` |
| `access_check_relation` | `auditor` |
| `history_check_object` | `groupsio_service:{uid}` |
| `history_check_relation` | `auditor` |

### Parent References

| Ref | Condition |
|---|---|
| `groupsio_service:{uid}` | Always set (uid is the parent service UID) |

---

## GroupsIO Mailing List

**Object type:** `groupsio_mailing_list`

**Source struct:** `internal/domain/model/grpsio_mailing_list.go` — `GroupsIOMailingList`

**NATS subject:** `lfx.index.groupsio_mailing_list`

**Indexed on:** create, update, delete of a GroupsIO mailing list (v1 datastream via `datastream_subgroup_handler.go`).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Mailing list unique identifier |
| `group_id` | int64 (optional) | Groups.io numeric group ID |
| `group_name` | string | Groups.io group name; emitted as empty string when not populated |
| `public` | bool | Whether the mailing list is publicly accessible |
| `audience_access` | string | Access model: `public`, `approval_required`, or `invite_only`; not populated by v1-sync transform — emitted as empty string |
| `source` | string | Source system identifier; always `"v1-sync"` for v1 datastream records |
| `type` | string | List type: `announcement`, `discussion_moderated`, or `discussion_open` |
| `subscriber_count` | int | Current number of subscribers |
| `committees` | []object (optional) | Associated committees. Each has `uid` (string) and `allowed_voting_statuses` ([]string) |
| `description` | string | Mailing list description |
| `title` | string | Mailing list title |
| `subject_tag` | string | Email subject tag; emitted as empty string when not populated |
| `service_uid` | string | UID of the parent GroupsIO service |
| `project_uid` | string | v2 UID of the owning project (resolved from v1 SFID) |
| `project_name` | string | Name of the owning project; emitted as empty string when not populated |
| `project_slug` | string | Slug of the owning project; emitted as empty string when not populated |
| `url` | string (optional) | Groups.io URL for the subgroup |
| `flags` | []string (optional) | Warning messages about unusual settings |
| `created_at` | timestamp | Creation time (RFC3339) |
| `updated_at` | timestamp | Last update time (RFC3339) |
| `system_updated_at` | timestamp (optional) | Last modified by a system process |

> **v1-sync transform note:** `transformV1ToGrpsIOMailingList` populates `uid`, `group_id`, `group_name`, `public` (from `visibility`), `type`, `description`, `title`, `subject_tag`, `url`, `flags`, `service_uid` (from `parent_id`), `project_uid`, `source` ("v1-sync"), `subscriber_count`, `committees`, and timestamps. `audience_access`, `project_name`, and `project_slug` are not set by the transform and will be emitted as empty strings.

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `groupsio_mailing_list_uid:{uid}` | `groupsio_mailing_list_uid:abc123` | Namespaced lookup by UID |
| `project_uid:{value}` | `project_uid:bb4ed8c8-...` | Find mailing lists for a project |
| `service_uid:{value}` | `service_uid:abc123` | Find mailing lists under a service |
| `type:{value}` | `type:announcement` | Find mailing lists by type |
| `public:{value}` | `public:true` | Find mailing lists by public status |
| `audience_access:{value}` | `audience_access:public` | Find mailing lists by audience access |
| `committee_uid:{value}` | `committee_uid:061a110a-...` | Find mailing lists associated with a committee (one tag per committee) |
| `committee_voting_status:{value}` | `committee_voting_status:Voting Rep` | Find mailing lists by committee voting status filter |
| `group_name:{value}` | `group_name:my-project` | Find mailing lists by Groups.io group name |

### Access Control (AccessMessage)

Published to `lfx.fga-sync.update_access` on create/update. Deleted via `lfx.fga-sync.delete_access` on delete.

| Field | Value |
|---|---|
| `object_type` | `groupsio_mailing_list` |
| `public` | value of `GroupsIOMailingList.Public` |
| `references.groupsio_service` | `[service_uid]` |
| `references.committee` | committee UIDs (one per associated committee) |
| `references.writer` | usernames from writers (when settings present) |
| `references.auditor` | usernames from auditors (when settings present) |

### Search Behavior (IndexingConfig)

| Field | Value |
|---|---|
| `object_id` | `{uid}` |
| `public` | value of `GroupsIOMailingList.Public` |
| `access_check_object` | `groupsio_mailing_list:{uid}` |
| `access_check_relation` | `viewer` |
| `history_check_object` | `groupsio_mailing_list:{uid}` |
| `history_check_relation` | `auditor` |
| `sort_name` | `title` falling back to `group_name` |
| `name_and_aliases` | `title`, `group_name` (non-empty values) |
| `fulltext` | space-joined non-empty values of `title`, `group_name`, `description` |

### Parent References

| Ref | Condition |
|---|---|
| `groupsio_service:{service_uid}` | Always set |
| `project:{project_uid}` | Only when `project_uid` is set |
| `committee:{uid}` | One per associated committee (when `committees` is non-empty) |

### Reverse Index

After a successful update, the handler writes a reverse index to `v1-mappings`:
- Key: `groupsio-subgroup-gid.{group_id}` → Value: `{uid}`

This allows the member and artifact handlers to resolve the mailing list UID from the Groups.io numeric `group_id`.

---

## GroupsIO Mailing List Settings

**Object type:** `groupsio_mailing_list_settings`

**Source struct:** `internal/domain/model/grpsio_mailing_list.go` — `GroupsIOMailingListSettings`

**NATS subject:** `lfx.index.groupsio_mailing_list_settings`

**Indexed on:** create/update of a GroupsIO mailing list when writers or auditors are present. Settings share the same UID as their parent mailing list.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Mailing list UID (same as the parent mailing list) |
| `writers` | []object | Users with write access. Each object has `username` (string, holds the user ID) |
| `auditors` | []object | Users with audit access. Each object has `username` (string, holds the user ID) |
| `last_reviewed_at` | string (optional) | RFC3339 timestamp of the last membership review |
| `last_reviewed_by` | string (optional) | UID of who performed the last review |
| `last_audited_by` | string (optional) | UID of who performed the last audit |
| `last_audited_time` | string (optional) | RFC3339 timestamp of the last audit |
| `created_at` | timestamp | Creation time (RFC3339) |
| `updated_at` | timestamp | Last update time (RFC3339) |

> **v1-sync build note:** `buildMailingListSettings` only populates `uid`, `writers`, and `auditors`. The optional review/audit fields and `created_at`/`updated_at` will be at Go zero values (`0001-01-01T00:00:00Z` for timestamps, `null` for optional strings).

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{uid}` | `abc123` | Direct lookup by UID |
| `mailing_list_uid:{uid}` | `mailing_list_uid:abc123` | Namespaced lookup by UID |

### Search Behavior (IndexingConfig)

| Field | Value |
|---|---|
| `object_id` | `{uid}` |
| `access_check_object` | `groupsio_mailing_list:{uid}` |
| `access_check_relation` | `auditor` |
| `history_check_object` | `groupsio_mailing_list:{uid}` |
| `history_check_relation` | `auditor` |

### Parent References

| Ref | Condition |
|---|---|
| `groupsio_mailing_list:{uid}` | Always set (uid is the parent mailing list UID) |

---

## GroupsIO Member

**Object type:** `groupsio_member`

**Source struct:** `internal/domain/model/grpsio_member.go` — `GrpsIOMember`

**NATS subject:** `lfx.index.groupsio_member`

**Indexed on:** create, update, delete of a GroupsIO mailing list member (v1 datastream via `datastream_member_handler.go`).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Member unique identifier |
| `mailing_list_uid` | string | UID of the parent mailing list (resolved from `group_id` reverse index) |
| `member_id` | int64 (optional) | Groups.io numeric member ID |
| `group_id` | int64 (optional) | Groups.io numeric group ID |
| `source` | string | Source system identifier; always `"v1-sync"` for v1 datastream records |
| `user_id` | string (optional) | User-service ID; omitted when empty |
| `username` | string | Groups.io username (LFID); emitted as empty string when not populated |
| `first_name` | string | First name (split from `full_name`); emitted as empty string when not populated |
| `last_name` | string | Last name (split from `full_name`); emitted as empty string when not populated |
| `email` | string | Member email address (RFC 5322); emitted as empty string when not populated |
| `organization` | string | Member's organization; emitted as empty string when not populated |
| `job_title` | string | Member's job title; emitted as empty string when not populated |
| `groups_email` | string (optional) | Lowercase email as recorded by Groups.io; omitted when empty |
| `groups_full_name` | string (optional) | Lowercase full name as recorded by Groups.io; omitted when empty |
| `committee_email` | string (optional) | Lowercase email from committee service; omitted when empty |
| `committee_full_name` | string (optional) | Lowercase full name from committee service; omitted when empty |
| `committee_id` | string (optional) | Committee UID if member belongs to a committee; omitted when empty |
| `role` | string (optional) | Role within the committee; omitted when empty |
| `voting_status` | string (optional) | Voting status (e.g. `Voting Rep`, `Non-Voting`); omitted when empty |
| `member_type` | string | `committee` or `direct`; emitted as empty string when not populated |
| `delivery_mode` | string | Email delivery preference; emitted as empty string when not populated |
| `delivery_mode_list` | string (optional) | Delivery mode as reported by Groups.io; omitted when empty |
| `mod_status` | string | Moderation status: `none`, `moderator`, or `owner`; emitted as empty string when not populated |
| `status` | string | Groups.io membership status (e.g. `normal`, `pending`); emitted as empty string when not populated |
| `last_reviewed_at` | string or null | RFC3339 timestamp of the last review; emitted as `null` when not set (not omitted) |
| `last_reviewed_by` | string or null | UID of who performed the last review; emitted as `null` when not set (not omitted) |
| `project_uid` | string (optional) | v2 UID of the owning project (inherited from parent mailing list); omitted when empty |
| `project_slug` | string (optional) | URL slug of the owning project (fetched via `lfx.projects-api.get_slug`); omitted when empty |
| `created_at` | timestamp | Creation time (RFC3339) |
| `updated_at` | timestamp | Last update time (RFC3339) |
| `system_updated_at` | timestamp (optional) | Last modified by a system process |

> **v1-sync note:** `project_uid` and `project_slug` are resolved by the subgroup handler (written to `groupsio-subgroup-project.{subgroup_uid}`) and read by the member handler before indexing. The member handler NAKs if the project mapping is absent, ensuring the subgroup is fully processed first.

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{uid}` | `abc123` | Direct lookup by UID |
| `member_uid:{uid}` | `member_uid:abc123` | Namespaced lookup by UID |
| `mailing_list_uid:{value}` | `mailing_list_uid:xyz789` | Find members of a mailing list |
| `username:{value}` | `username:jdoe` | Find members by username |
| `email:{value}` | `email:jdoe@example.com` | Find members by email |
| `status:{value}` | `status:normal` | Find members by Groups.io status |
| `project_uid:{value}` | `project_uid:bb4ed8c8-...` | Find members belonging to a project |

> Tags for `username`, `email`, `status`, and `project_uid` are only emitted when the value is non-empty.

### Access Control (AccessMessage)

When a member has a non-empty `username`, the handler also publishes an FGA membership message:
- **Put member:** `lfx.fga-sync.member_put` on create/update
- **Remove member:** `lfx.fga-sync.member_remove` on delete

The message is a `GenericFGAMessage` with `object_type: groupsio_mailing_list`, `operation: member_put` / `member_remove`, and a `FGAMemberPutData` payload containing `uid` (the mailing list UID), `username` (principal), and `relations: ["member"]`.

> **Username transform:** The `username` field in this FGA payload is **not** the raw Groups.io/LFID username. It is the principal value derived via `principal.FromUsername(member.Username)`, which produces an Auth0-style subject (e.g. `auth0|...`). Downstream FGA consumers should expect this format.

### Search Behavior (IndexingConfig)

| Field | Value |
|---|---|
| `object_id` | `{uid}` |
| `access_check_object` | `groupsio_mailing_list:{mailing_list_uid}` |
| `access_check_relation` | `viewer` |
| `history_check_object` | `groupsio_mailing_list:{mailing_list_uid}` |
| `history_check_relation` | `auditor` |
| `sort_name` | `last_name + ", " + first_name`, falling back to `last_name`, `first_name`, or `username` |
| `name_and_aliases` | full name (`first_name + " " + last_name`), `username`, `email` (non-empty values) |
| `fulltext` | space-joined non-empty values of `first_name`, `last_name`, `email`, `organization`, `job_title` |

### Parent References

| Ref | Condition |
|---|---|
| `groupsio_mailing_list:{mailing_list_uid}` | Always set |
| `project:{project_uid}` | Only when `project_uid` is set |

---

## GroupsIO Artifact

**Object type:** `groupsio_artifact`

**Source struct:** `internal/domain/model/grpsio_artifact.go` — `GroupsIOArtifact`

**NATS subject:** `lfx.index.groupsio_artifact`

**Indexed on:** create, update, delete of a GroupsIO subgroup artifact (v1 datastream via `datastream_artifact_handler.go`).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `artifact_id` | string | Artifact unique identifier |
| `group_id` | uint64 | Groups.io numeric group ID |
| `project_uid` | string (optional) | v2 UID of the owning project (resolved from v1 SFID) |
| `committee_uid` | string (optional) | v2 UID of the associated committee (resolved from v1 SFID) |
| `type` | string (optional) | Artifact type (e.g. `file`, `link`) |
| `media_type` | string (optional) | MIME type of the file |
| `filename` | string (optional) | Filename of the artifact |
| `link_url` | string (optional) | URL for link-type artifacts |
| `download_url` | string (optional) | Groups.io download URL |
| `s3_key` | string (optional) | S3 object key |
| `file_uploaded` | bool (optional) | Whether the file has been uploaded; omitted for link-type artifacts |
| `file_upload_status` | string (optional) | Upload status (e.g. `completed`) |
| `file_uploaded_at` | timestamp (optional) | When the file was uploaded |
| `message_ids` | []uint64 (optional) | IDs of associated Groups.io messages; **not populated by `transformV1ToGroupsIOArtifact`** — omitted from v1-sync payloads |
| `last_posted_at` | timestamp (optional) | When the artifact was last posted |
| `last_posted_message_id` | uint64 (optional) | ID of the last posted message; **not populated by `transformV1ToGroupsIOArtifact`** — omitted from v1-sync payloads |
| `description` | string (optional) | Artifact description |
| `created_by` | object (optional) | User who created the artifact (`id`, `username`, `name`, `email`, `profile_picture`); **not populated by `transformV1ToGroupsIOArtifact`** — omitted from v1-sync payloads |
| `last_modified_by` | object (optional) | User who last modified the artifact; **not populated by `transformV1ToGroupsIOArtifact`** — omitted from v1-sync payloads |
| `created_at` | timestamp | Creation time (RFC3339) |
| `updated_at` | timestamp | Last update time (RFC3339) |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{artifact_id}` | `a323373e-...` | Direct lookup by artifact ID |
| `group_artifact_id:{artifact_id}` | `group_artifact_id:a323373e-...` | Namespaced lookup by artifact ID |
| `group_id:{value}` | `group_id:118856` | Find artifacts for a Groups.io group |
| `project_uid:{value}` | `project_uid:bb4ed8c8-...` | Find artifacts for a project |
| `committee_uid:{value}` | `committee_uid:061a110a-...` | Find artifacts for a committee |

> `project_uid` and `committee_uid` tags are only emitted when the value is non-empty.

### Access Control (IndexingConfig)

Artifacts use a typed `IndexingConfig` (no server-side enrichers). No FGA `AccessMessage` is published — access is checked at query time via the indexing config.

| Field | Value |
|---|---|
| `object_id` | `{artifact_id}` |
| `public` | `false` (always) |
| `access_check_object` | `groupsio_mailing_list:{mailing_list_uid}` |
| `access_check_relation` | `viewer` |
| `history_check_object` | `groupsio_mailing_list:{mailing_list_uid}` |
| `history_check_relation` | `auditor` |

### Search Behavior

| Field | Value |
|---|---|
| `fulltext` | `filename` (or `link_url`) + ` ` + `description` |
| `name_and_aliases` | `filename`, `link_url` (non-empty values only) |
| `sort_name` | `filename` if set, otherwise `link_url` |
| `public` | `false` (always) |

### Parent References

| Ref | Condition |
|---|---|
| `project:{project_uid}` | Only when `project_uid` is set |
| `committee:{committee_uid}` | Only when `committee_uid` is set |
| `groupsio_mailing_list:{group_id}` | Always set (numeric Groups.io group ID — the artifact model does not carry a mailing list UID) |
