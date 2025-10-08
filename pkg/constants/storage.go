// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

const (
	// KVBucketNameGroupsIOServices is the name of the KV bucket for services.
	KVBucketNameGroupsIOServices = "groupsio-services"

	// KVBucketNameGroupsIOMailingLists is the name of the KV bucket for mailing lists.
	KVBucketNameGroupsIOMailingLists = "groupsio-mailing-lists"

	// KVBucketNameGroupsIOMembers is the name of the KV bucket for members.
	KVBucketNameGroupsIOMembers = "groupsio-members"

	// Lookup key patterns for unique constraints
	// KVLookupGroupsIOServicePrefix is the key pattern for unique constraint lookups
	KVLookupGroupsIOServicePrefix = "lookup/groupsio-services/%s"

	// Service secondary index key patterns for external GroupsIO IDs
	// KVLookupGroupsIOServiceByGroupIDPrefix is the key pattern for GroupID index (lookup by Groups.io group ID)
	KVLookupGroupsIOServiceByGroupIDPrefix = "lookup/groupsio-service-groupid/%d"

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

	// Key prefixes for bucket detection
	// GroupsIOMailingListKeyPrefix is the common prefix for all mailing list related keys
	GroupsIOMailingListKeyPrefix = "lookup/groupsio-mailing-list/"
	// GroupsIOServiceLookupKeyPrefix is the prefix for service lookup keys
	GroupsIOServiceLookupKeyPrefix = "lookup/groupsio-services/"
	// GroupsIOMemberLookupKeyPrefix is the prefix for member lookup keys
	GroupsIOMemberLookupKeyPrefix = "lookup/groupsio-members/"
)
