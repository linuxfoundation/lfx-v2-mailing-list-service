# Backfill service-to-mailing-list mappings

This script reads existing `itx-groupsio-v2-subgroup.*` entries in the NATS JetStream `v1-objects` KV bucket and populates the dedicated `groupsio-subgroup-service-index` KV bucket used by `ListSubgroupsByService`:

- `groupsio-subgroup-service.{service_uid}.{subgroup_uid}` → subgroup UID
- `groupsio-subgroup-parent.{subgroup_uid}` → service UID
- `groupsio-subgroup-previous-service.{subgroup_uid}` → old service UID while a move's old index key awaits cleanup (maintained by the event handler)

It skips soft-deleted, absent, and unprocessed subgroups (those without a live `groupsio-subgroup.{uid}` mapping in `v1-mappings`). A live subgroup without `parent_id` is skipped only when its stored, published, and pending parent pointers are cleared **and** `groupsio-subgroup-unassociated.{uid}` confirms completed UID-based document/access cleanup. `groupsio-subgroup-unassociation-pending.{uid}` is written **before** any destructive unassociation publication and must be absent for any backfill skip or write. A missing completion marker may indicate a legacy indexed list with no service-index pointers; the script reports it as unresolved rather than guessing. An **associated** source record with either marker is also unresolved: restoring its index without replaying the subgroup access event can leave private lists inaccessible. Reprocess unresolved records from their latest source through the subgroup event handler, verify their document and access, and rerun the dry run **before** pausing event processing for the write run. Replaying only the service is not sufficient. It supports JSON and msgpack records. A parent mismatch is only repaired when a pending move marker identifies that old parent; otherwise the script reports the conflict rather than overwriting it. It never republishes indexer events or writes to `v1-objects`. Write mode tombstones `groupsio-service-domain-indexed.{service_uid}` in `v1-mappings` **before** adding or repairing that service's index mappings, so an unchanged-domain service replay will reindex its lists.

Run from the repository root with Go and access to all three NATS KV buckets (`v1-objects`, `v1-mappings`, and `groupsio-subgroup-service-index`). `NATS_URL` defaults to `nats://127.0.0.1:4222`.
The service chart provisions `groupsio-subgroup-service-index` by default; create the bucket before running the script outside Helm-managed environments.

```sh
NATS_URL=nats://localhost:4222 go run ./scripts/backfill_subgroup_service_index/
NATS_URL=nats://localhost:4222 go run ./scripts/backfill_subgroup_service_index/ -write
```

The default is dry-run; `-write` persists missing mappings and invalidates the affected services' propagation checkpoints. It **never** clears an unassociation completion marker: only successful subgroup event processing can confirm access has been restored. Progress is logged per affected subgroup and final counts show listed, processed (planned or written), skipped and failed records. Failures result in a nonzero exit status. The script is safe to rerun: already complete mappings are skipped.

For an existing deployment, pause service **and** subgroup event processing across all replicas before the write run; keep it paused until backfill completes successfully. Resume processing and redeliver/reprocess the latest `itx-groupsio-v2-service.<service_uid>` record for **each affected service**, even if its domain has not changed. The invalidated checkpoint causes a full mailing-list refresh. Confirm `groupsio-service-domain-indexed.<service_uid>` matches the current domain after replay. If backfill fails, repair and rerun it before replaying services; do not let event processing checkpoint a partially populated index.
