// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

const (
	// KVBucketNameGrpsIOServices is the name of the KV bucket for services.
	KVBucketNameGrpsIOServices = "groupsio-services"

	// KVBucketNameGrpsIOMailingLists is the name of the KV bucket for mailing lists.
	KVBucketNameGrpsIOMailingLists = "groupsio-mailing-lists"

	// Lookup key patterns for unique constraints
	// KVLookupGrpsIOServicePrefix is the key pattern for unique constraint lookups
	KVLookupGrpsIOServicePrefix = "lookup/grpsio_services/%s"

	// Mailing list secondary index key patterns
	// KVLookupMailingListParentPrefix is the key pattern for parent service index
	KVLookupMailingListParentPrefix = "mailing-list-parent-%s"
	// KVLookupMailingListCommitteePrefix is the key pattern for committee index
	KVLookupMailingListCommitteePrefix = "mailing-list-committee-%s"
	// KVLookupMailingListProjectPrefix is the key pattern for project index
	KVLookupMailingListProjectPrefix = "mailing-list-project-%s"
	// KVLookupMailingListConstraintPrefix is the key pattern for uniqueness constraint (parent_id + group_name)
	KVLookupMailingListConstraintPrefix = "mailing-list-name-%s-%s"

	// Key prefixes for bucket detection
	// MailingListKeyPrefix is the common prefix for all mailing list related keys
	MailingListKeyPrefix = "mailing-list-"
	// ServiceLookupKeyPrefix is the prefix for service lookup keys
	ServiceLookupKeyPrefix = "lookup/grpsio_services/"
)
