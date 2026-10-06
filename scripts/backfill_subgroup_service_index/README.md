# Backfill service-to-mailing-list mappings

This script reads existing `itx-groupsio-v2-subgroup.*` entries in the NATS JetStream `v1-objects` KV bucket and populates the dedicated `groupsio-subgroup-service-index` KV bucket used by `ListSubgroupsByService`:

- `groupsio-subgroup-service.{service_uid}.{subgroup_uid}` → subgroup UID
- `groupsio-subgroup-parent.{subgroup_uid}` → service UID
- `groupsio-subgroup-previous-service.{subgroup_uid}` → old service UID while a move's old index key awaits cleanup (maintained by the event handler)

It skips soft-deleted, absent, and unprocessed subgroups (those without a live `groupsio-subgroup.{uid}` mapping in `v1-mappings`). It supports JSON and msgpack records. A parent mismatch is only repaired when a pending move marker identifies that old parent; otherwise the script reports the conflict rather than overwriting it. It never republishes indexer events or writes to `v1-objects`.

Run from the repository root with Go and access to all three NATS KV buckets (`v1-objects`, `v1-mappings`, and `groupsio-subgroup-service-index`). `NATS_URL` defaults to `nats://127.0.0.1:4222`.
The service chart provisions `groupsio-subgroup-service-index` by default; create the bucket before running the script outside Helm-managed environments.

```sh
NATS_URL=nats://localhost:4222 go run ./scripts/backfill_subgroup_service_index/
NATS_URL=nats://localhost:4222 go run ./scripts/backfill_subgroup_service_index/ -write
```

The default is dry-run; `-write` persists only missing mappings. Progress is logged per affected subgroup and final counts show listed, processed (planned or written), skipped and failed records. Failures result in a nonzero exit status. The script is safe to rerun: already complete mappings are skipped. Run it while subgroup updates are quiescent to avoid races with concurrent changes.
