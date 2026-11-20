# Backfill service-to-mailing-list mappings

This script reads existing `itx-groupsio-v2-subgroup.*` entries in the NATS JetStream `v1-objects` KV bucket and populates the dedicated `groupsio-subgroup-service-index` KV bucket used by `ListSubgroupsByService`:

- `groupsio-subgroup-service.{service_uid}.{subgroup_uid}` → subgroup UID
- `groupsio-subgroup-parent.{subgroup_uid}` → service UID
- `groupsio-subgroup-previous-service.{subgroup_uid}` → old service UID while a move's old index key awaits cleanup (maintained by the event handler)

It skips soft-deleted, absent, and unprocessed subgroups, using verified
`mailing_list_sync_state` in Postgres for processed/deleted state. Source records
can be JSON or msgpack. A parent mismatch is repaired only with a matching pending
move marker; conflicts are preserved. It never republishes events or writes to
`v1-objects` or the PG processed-state tables.

Run with Go, access to `v1-objects` and `groupsio-subgroup-service-index`, and the
Secret-loaded `MAILING_LIST_STATE_DATABASE_URL`. PG cutover must already be
verified; see [state migration](../../docs/event-state-postgres.md). No legacy
`v1-mappings` access is required. `NATS_URL` defaults to `nats://127.0.0.1:4222`.
The service chart provisions `groupsio-subgroup-service-index` by default; create the bucket before running the script outside Helm-managed environments.

```sh
# Load MAILING_LIST_STATE_DATABASE_URL from the approved Secret first.
NATS_URL=nats://localhost:4222 go run ./scripts/backfill_subgroup_service_index/
NATS_URL=nats://localhost:4222 go run ./scripts/backfill_subgroup_service_index/ -write
```

The default is dry-run; `-write` persists only missing mappings. Progress is logged per affected subgroup and final counts show listed, processed (planned or written), skipped and failed records. Failures result in a nonzero exit status. The script is safe to rerun: already complete mappings are skipped. Run it while subgroup updates are quiescent to avoid races with concurrent changes.
