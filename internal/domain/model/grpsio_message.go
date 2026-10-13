// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"time"
)

// GroupsIOMessage represents a single Groups.io mailing list message.
// One record per message; messages are grouped into threads by TopicID.
type GroupsIOMessage struct {
	MessageID      string    `json:"message_id"`
	GroupID        uint64    `json:"group_id"`
	TopicID        uint64    `json:"topic_id"`
	Subject        string    `json:"subject"`
	Snippet        string    `json:"snippet"`
	SenderName     string    `json:"sender_name"`
	MsgNum         uint64    `json:"msg_num"`
	IsReply        bool      `json:"is_reply"`
	GroupDomain    string    `json:"group_domain"`
	GroupName      string    `json:"group_name"`
	MailingListUID string    `json:"mailing_list_uid"`
	CreatedAt      time.Time `json:"created_at"`
}

// Tags returns the indexing tags for the message. The owning mailing list is tagged as
// `mailing_list:{uid}`; committee association and visibility are not stored on the message
// and are resolved from the mailing list at query time.
func (m *GroupsIOMessage) Tags() []string {
	if m == nil {
		return nil
	}
	tags := []string{
		fmt.Sprintf("topic_id:%d", m.TopicID),
		fmt.Sprintf("group_id:%d", m.GroupID),
	}
	if m.MailingListUID != "" {
		tags = append(tags, fmt.Sprintf("mailing_list:%s", m.MailingListUID))
	}
	return tags
}

// ParentRefs returns the parent resource references for indexing.
func (m *GroupsIOMessage) ParentRefs() []string {
	if m == nil {
		return nil
	}
	var refs []string
	if m.MailingListUID != "" {
		refs = append(refs, fmt.Sprintf("groupsio_mailing_list:%s", m.MailingListUID))
	}
	return refs
}

// SortName returns the subject as the primary sort name.
func (m *GroupsIOMessage) SortName() string {
	if m == nil {
		return ""
	}
	return m.Subject
}

// Fulltext returns subject and snippet concatenated for full-text search.
func (m *GroupsIOMessage) Fulltext() string {
	if m == nil {
		return ""
	}
	if m.Snippet == "" {
		return m.Subject
	}
	if m.Subject == "" {
		return m.Snippet
	}
	return m.Subject + " " + m.Snippet
}

// NameAndAliases returns the subject as the display name for search.
func (m *GroupsIOMessage) NameAndAliases() []string {
	if m == nil {
		return nil
	}
	if m.Subject == "" {
		return nil
	}
	return []string{m.Subject}
}
