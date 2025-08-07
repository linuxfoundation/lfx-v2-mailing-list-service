// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"time"
)

// MailingList represents a mailing list entity
type MailingList struct {
	UID                string    `json:"uid"`
	GroupName          string    `json:"group_name"`
	Visibility         string    `json:"visibility"`
	Type               string    `json:"type"`
	MailingListID      string    `json:"mailing_list"`
	MailingListFilters []string  `json:"mailing_list_filters"`
	Flags              []string  `json:"flags"`
	Description        string    `json:"description"`
	Title              string    `json:"title"`
	SubjectTag         string    `json:"subject_tag"`
	ParentID           string    `json:"parent_id"`
	ProjectID          string    `json:"project_id"`
	GroupID            int64     `json:"group_id"`
	URL                string    `json:"url"`
	SubscriberCount    int       `json:"subscriber_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
