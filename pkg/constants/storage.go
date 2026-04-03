// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

const (
	// KVBucketNameGroupsIOServices is the name of the KV bucket for services.
	KVBucketNameGroupsIOServices = "groupsio-services"

	// KVBucketNameGroupsIOServiceSettings is the name of the KV bucket for service settings.
	KVBucketNameGroupsIOServiceSettings = "groupsio-service-settings"

	// KVBucketNameGroupsIOMailingLists is the name of the KV bucket for mailing lists.
	KVBucketNameGroupsIOMailingLists = "groupsio-mailing-lists"

	// KVBucketNameGroupsIOMailingListSettings is the name of the KV bucket for mailing list settings.
	KVBucketNameGroupsIOMailingListSettings = "groupsio-mailing-list-settings"

	// KVBucketNameGroupsIOMembers is the name of the KV bucket for members.
	KVBucketNameGroupsIOMembers = "groupsio-members"

	// Lookup key patterns for unique constraints
	// KVLookupGroupsIOServicePrefix is the key pattern for unique constraint lookups
	KVLookupGroupsIOServicePrefix = "lookup/groupsio-services/%s"

	// Service secondary index key patterns for external GroupsIO IDs
	// KVLookupGroupsIOServiceByGroupIDPrefix is the key pattern for GroupID index (lookup by Groups.io group ID)
	KVLookupGroupsIOServiceByGroupIDPrefix = "lookup/groupsio-service-groupid/%d"
	// KVLookupGroupsIOServiceByProjectUIDPrefix is the key pattern for ProjectUID index (lookup by ProjectUID)
	KVLookupGroupsIOServiceByProjectUIDPrefix = "lookup/groupsio-service-projectuid/%s"

	// Mailing list secondary index key patterns
	// KVLookupMailingListServicePrefix is the key pattern for service index
	KVLookupGroupsIOMailingListServicePrefix = "lookup/groupsio-mailing-list-service/%s"
	// KVLookupMailingListCommitteePrefix is the key pattern for committee index
	KVLookupGroupsIOMailingListCommitteePrefix = "lookup/groupsio-mailing-list-committee/%s"
	// KVLookupMailingListProjectPrefix is the key pattern for project index
	KVLookupGroupsIOMailingListProjectPrefix = "lookup/groupsio-mailing-list-project/%s"
	// KVLookupMailingListConstraintPrefix is the key pattern for uniqueness constraint (hashed service_id + group_name)
	KVLookupGroupsIOMailingListConstraintPrefix = "lookup/groupsio-mailing-list-name/%s"
	// KVLookupGroupsIOMailingListBySubgroupIDPrefix is the key pattern for SubgroupID index (lookup by Groups.io subgroup ID)
	KVLookupGroupsIOMailingListBySubgroupIDPrefix = "lookup/groupsio-mailing-list-subgroupid/%d"

	// Member lookup key patterns
	// KVLookupGroupsIOMemberPrefix is the key pattern for member unique constraint lookups (email per mailing list)
	KVLookupGroupsIOMemberPrefix = "lookup/groupsio-members/%s"
	// KVLookupGroupsIOMemberConstraintPrefix is the key pattern for member uniqueness constraint (mailing_list_uid + email)
	KVLookupGroupsIOMemberConstraintPrefix = "lookup/groupsio-member-email/%s"
	// KVLookupGroupsIOMemberByMemberIDPrefix is the key pattern for GroupsIOMemberID index (lookup by Groups.io member ID)
	KVLookupGroupsIOMemberByMemberIDPrefix = "lookup/groupsio-member-memberid/%d"
	// KVLookupGroupsIOMemberByGroupIDPrefix is the key pattern for GroupsIOGroupID index (lookup by Groups.io group ID)
	KVLookupGroupsIOMemberByGroupIDPrefix = "lookup/groupsio-member-groupid/%d"

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
	// KVMappingPrefixArtifact is the v1-mappings key prefix for GroupsIO artifacts.
	KVMappingPrefixArtifact = "groupsio-artifact"

	// KVMappingPrefixProjectBySFID is the v1-mappings forward index written by lfx-v1-sync-helper:
	// project.sfid.{sfid} → v2 project UID. Used to resolve the v1 project_id (SFID) to a v2 UID.
	KVMappingPrefixProjectBySFID = "project.sfid"

	// KVMappingPrefixCommitteeBySFID is the v1-mappings forward index written by lfx-v1-sync-helper:
	// committee.sfid.{sfid} → v2 committee UID. Used to resolve the v1 committee SFID to a v2 UID.
	KVMappingPrefixCommitteeBySFID = "committee.sfid"

	// Key prefixes for bucket detection
	// GroupsIOMailingListKeyPrefix is the common prefix for all mailing list related keys
	GroupsIOMailingListKeyPrefix = "lookup/groupsio-mailing-list/"
	// GroupsIOServiceLookupKeyPrefix is the prefix for service lookup keys
	GroupsIOServiceLookupKeyPrefix = "lookup/groupsio-services/"
	// GroupsIOMemberLookupKeyPrefix is the prefix for member lookup keys
	GroupsIOMemberLookupKeyPrefix = "lookup/groupsio-members/"
)
