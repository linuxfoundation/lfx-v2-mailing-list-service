# Domain language

Terms used when discussing the v1 data-stream sync in this service. They name the seams in
`internal/service/datastream_*` and `internal/domain/port/mapping_store.go`.

**Datastream handler** — a function in `internal/service` that turns one v1 DynamoDB KV event
(PUT, soft-delete or hard delete) for a GroupsIO entity into indexer and FGA messages. There is
one update and one delete handler per entity: service, subgroup (mailing list), member,
artifact, message.

**Mapping store** — the `v1-mappings` KV bucket (plus the `groupsio-subgroup-service-index`
bucket), accessed through `port.MappingReaderWriter`. It records which v1 entities have been
processed and the lookups between them (project SFID → project UID, group_id → subgroup UID).

**Mapping key / mapping entry** — a key in the mapping store, and the result of reading it with
`Lookup`: `Absent`, `Present` (with a value) or `Tombstoned`. A failed read is an error, never
`Absent`.

**Tombstone** — the deletion marker written over a mapping key when its entity is deleted. It
makes duplicate deletes no-ops and lets a later create re-index the entity.

**Ack outcome** — what a datastream handler decides for the message it was given: *ack*
(finished, or can never succeed) or *nak* (redeliver later). It is converted to the `bool`
contract of `port.DataEventHandler` at the handler boundary (`true` = nak).

**Permanent error** — an error an adapter has marked with `pkg/errors.MarkPermanent`, meaning a
retry cannot succeed. Only permanent errors are acked; any other failure is naked and bounded by
the consumer's `MaxDeliver`.

**Ordering NAK** — a nak issued because a parent mapping (project, service, subgroup, committee)
has not been written yet. The v1 sync does not guarantee cross-entity ordering, so children wait
for their parents by retrying.
