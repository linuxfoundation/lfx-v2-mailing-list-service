// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

const (
	// KVBucketNameV1Mappings is the shared KV bucket used by v1 eventing consumers to track
	// processed entities (idempotency, created-vs-updated, tombstone markers for deletes).
	KVBucketNameV1Mappings = "v1-mappings"

	// KVBucketV1Objects is the NATS KV bucket that lfx-v1-sync-helper writes DynamoDB records into.
	KVBucketV1Objects = "v1-objects"

	// KVTombstoneMarker is the value written to v1-mappings after a successful delete,
	// preventing duplicate delete processing on consumer redelivery.
	KVTombstoneMarker = "!del"

	// KVMappingPrefixService is the v1-mappings key prefix for GroupsIO services.
	KVMappingPrefixService = "groupsio-service"
	// KVMappingPrefixSubgroup is the v1-mappings key prefix for GroupsIO subgroups (mailing lists).
	KVMappingPrefixSubgroup = "groupsio-subgroup"
	// KVMappingPrefixMember is the v1-mappings key prefix for GroupsIO members.
	KVMappingPrefixMember = "groupsio-member"
	// KVMappingPrefixSubgroupByGroupID is the v1-mappings reverse index: Groups.io group_id → subgroup UID.
	// Written by the subgroup handler so the member handler can resolve MailingListUID from group_id.
	KVMappingPrefixSubgroupByGroupID = "groupsio-subgroup-gid"
	// KVMappingPrefixSubgroupProject is the v1-mappings key that stores the project UID and slug for a
	// subgroup (mailing list). Written by the subgroup handler; read by the member handler so that
	// project_uid and project_slug can be included on indexed groupsio_member records.
	// Value format: "{project_uid}|{project_slug}"
	KVMappingPrefixSubgroupProject = "groupsio-subgroup-project"
	// KVMappingPrefixArtifact is the v1-mappings key prefix for GroupsIO artifacts.
	KVMappingPrefixArtifact = "groupsio-artifact"

	// KVMappingPrefixProjectBySFID is the v1-mappings forward index written by lfx-v1-sync-helper:
	// project.sfid.{sfid} → v2 project UID. Used to resolve the v1 project_id (SFID) to a v2 UID.
	KVMappingPrefixProjectBySFID = "project.sfid"

	// KVMappingPrefixCommitteeBySFID is the v1-mappings forward index written by lfx-v1-sync-helper:
	// committee.sfid.{sfid} → v2 committee UID. Used to resolve the v1 committee SFID to a v2 UID.
	KVMappingPrefixCommitteeBySFID = "committee.sfid"
)
