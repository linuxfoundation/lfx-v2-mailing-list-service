-- Copyright The Linux Foundation and each contributor to LFX.
-- SPDX-License-Identifier: MIT

CREATE TABLE IF NOT EXISTS mailing_list_sync_state (
    resource_kind text NOT NULL,
    source_id text NOT NULL,
    source_uid text NOT NULL,
    deleted boolean NOT NULL DEFAULT false,
    PRIMARY KEY(resource_kind, source_id)
);
CREATE TABLE IF NOT EXISTS mailing_list_member_principals (
    member_id text PRIMARY KEY,
    source_uid text NOT NULL,
    username text NOT NULL,
    mailing_list_id text NOT NULL,
    structured boolean NOT NULL,
    deleted boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS mailing_list_service_domains (
    service_id text PRIMARY KEY,
    domain_name text NOT NULL,
    deleted boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS mailing_list_group_lookup (
    group_id text PRIMARY KEY,
    mailing_list_id text NOT NULL,
    deleted boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS mailing_list_project_refs (
    mailing_list_id text PRIMARY KEY,
    project_uid text NOT NULL,
    project_slug text NOT NULL,
    deleted boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS mailing_list_invitation_guards (
    member_id text PRIMARY KEY,
    invite_id text NOT NULL,
    dispatch_pending boolean NOT NULL,
    deleted boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS mailing_list_state_cutover (
    singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
    source_last_sequence bigint NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT now()
);
