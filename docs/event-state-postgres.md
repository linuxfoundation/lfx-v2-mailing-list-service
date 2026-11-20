# Mailing-list-owned Postgres event state

ITX remains the source of business entities; their IDs do not change. Processing
state moves to service-owned Postgres tables, not the helper's `v1_mappings` table:

| Table | Owned state |
| --- | --- |
| `mailing_list_sync_state` | Service/subgroup/artifact/message source IDs and retained deletion state |
| `mailing_list_member_principals` | Member source UID, prior username, parent list, legacy-format distinction and deletion state required for FGA cleanup |
| `mailing_list_service_domains` | Service domains for subgroup denormalization |
| `mailing_list_group_lookup` | Groups.io group ID → existing ITX subgroup UID |
| `mailing_list_project_refs` | Subgroup → canonical project UID and slug |
| `mailing_list_invitation_guards` | Member dispatch claims and returned invite IDs |
| `mailing_list_state_cutover` | Verified offline import checkpoint |

The port retains legacy key/entry encoding at the handler boundary, but the
adapter decomposes it into named columns. No names, emails, invite payloads or
credentials are migrated. Prior usernames are required by the FGA cleanup
contract. Empty present values remain distinct from absent and deleted rows.

The **separate** `groupsio-subgroup-service-index` bucket stays in NATS for
subgroup-by-service, parent and previous-parent indexes. It is not part of this
migration. The composite adapter preserves that routing and checks live subgroup
state in PG when enumerating the native index. Canonical `project.sfid.*` and
`committee.sfid.*` reads now go through the existing helper RPC, rather than
reading shared KV directly. No canonical helper data is copied into these tables.

## Configuration and cutover

Set `MAILING_LIST_STATE_DATABASE_URL` from an approved Secret, using verified
TLS in deployed environments. `eventStateDatabase.secretName`/`secretKey` wire
an existing single-key DSN Secret in Helm; alternatively use
`app.environment.MAILING_LIST_STATE_DATABASE_URL.valueFrom`, never both. Each
operation has a five-second deadline, each process pool has at most ten
connections. Runtime eventing and subgroup-index repair require a completed
cutover checkpoint and never seed empty state or fall back to legacy KV.

1. Provision the service-owned database/schema and Secret. Use a migration role
   for DDL/checkpoint writes; limit runtime to table DML and checkpoint reads.
2. Stop/drain all legacy mappings writers and the mailing-list consumer/repair
   tools. Keep this approved writer fence through import and deployment. Do not
   stop source `v1-objects` replication; retain its stream and durable consumer.
3. Run `go run ./scripts/migrate_event_state` with `NATS_URL` and the Secret-loaded
   DSN. Dry-run validates owned source state without creating schema or writing.
   Scans use next-by-subject request/reply, not a bulk ephemeral consumer. Memory
   holds only owned latest entries; size the job for measured state volume.
4. Reconcile source/history gaps and any intentional differences from historical
   wrapper copies in the helper's PG table. Those copies are not implicitly
   authoritative and are neither overwritten nor automatically removed.
5. Run the same command with `-apply`. Create-only imports preserve agreeing
   partial rows, compare every entry semantically and reject conflicts/unmatched
   target rows. Native DEL/PURGE removes source entries; application `!del` is
   retained. Source first/last sequence and message count must stay unchanged.
   Invalid or empty source state aborts. A failed run never enables cutover.
6. Deploy the new image with the verified PG Secret while the writer fence is
   still active, then restore other still-legacy writers. Verify dev before prod:
   parent dependencies, list moves, domain/project denormalization, member
   username changes/deletion, native service-index listing, artifacts/messages,
   and invitation replay; check search documents and FGA tuples.

The importer changes no NATS data or deployed settings and refuses reimport after
the checkpoint exists, since PG may contain newer state. Do not erase that
checkpoint to force a retry. Pending invitation claims after remote errors or
failed completion writes require manual reconciliation; they are never expired
or purged automatically to permit another send.

### Finding and reconciling pending invitation claims

The policy intentionally treats unclassified send errors conservatively. A
generic error is not proof of non-delivery; automatic release would also require
an ownership-safe claim and durable retry eligibility. Pending means **claimed**,
not proof an invite was dispatched (preflight can also stop after reservation).

With approved read access, inventory only relevant pending rows:

```sql
SELECT member_id, invite_id
FROM mailing_list_invitation_guards
WHERE dispatch_pending AND NOT deleted
ORDER BY member_id;
```

For each source member ID, correlate the source member/list record with logs
`failed to send LFID invite for mailing-list member`,
`failed to update mailing-list member LFID invite sent marker`, and
`sent LFID invite for mailing-list member`. These carry `member_uid`; a confirmed
send also carries its returned invite ID. Verify the authoritative invite/delivery
record using approved support tooling before concluding whether a send occurred.
If a known successful invite exists, reconcile that exact guard through an
approved per-record repair. If non-dispatch is proven, use an explicitly approved
support/recovery action for that member. Unknown outcomes stay pending: never
bulk-clear, expire, or delete guards based only on age/error text. Merely deleting
a guard is not a reliable retry because a processed member redelivery is an update.

During shutdown, new consumption stops first. Active callbacks retain their
processing context during the graceful window, and PG stays open until callback
completion so normal rolling restarts do not manufacture failed completion writes.

Failure logs contain only sanitized categories: `import_busy`,
`already_verified`, `empty_source`, `invalid_source_state`, `target_conflict`,
`target_extra_rows`, `source_changed`, or `infrastructure_failure`. They do not
assert that a pre-existing checkpoint was removed or disabled. Busy/infrastructure
failures require checking availability; source/target discrepancies require
reconciliation. `already_verified` means do not force a reimport.

Before new PG writes, rollback can resume the old consumer against unchanged KV.
After PG writes, KV is stale: fence processing and explicitly reconcile an
approved rollback target before using an old image. No automatic dual-write or
stale-read fallback is provided. Remove obsolete shared KV/PG copies only after
verified owner cutovers and approved cleanup. Shared bucket retirement remains
gated by the other wrappers, helper and read-only tooling in ops #1649.

`backfill_subgroup_service_index` reads PG processed-state and still writes only
the native subgroup index. `migrate_event_state` is the sole explicit offline
legacy mappings reader in this repo. Live provisioning and deployed acceptance
checks are operational rollout work, not actions performed by this PR.

## Tests

Use `MAILING_LIST_STATE_TEST_DATABASE_URL` with a disposable local Postgres
database and run `make test`. Database tests create/drop only unique test schemas;
without the variable, they skip. Tests exercise typed roundtrips, empty/absent/
deleted distinctions, concurrent claims, all owned import families, interleaved
unrelated/native-index keys, sparse revisions, deletion/recreation, and refused
changed-source/conflicting-target checkpoints. Existing handler tests cover
parent ordering and prior-principal cleanup.
