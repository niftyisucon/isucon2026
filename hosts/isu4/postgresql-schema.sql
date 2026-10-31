--
-- PostgreSQL database dump
--

\restrict z9qVZJV8P9b0X229pJ12bYQFzP69kyzvdewe6oT6qRWm9ZodM8INPoefu5E8dBm

-- Dumped from database version 16.15 (Ubuntu 16.15-0ubuntu0.24.04.1)
-- Dumped by pg_dump version 16.15 (Ubuntu 16.15-0ubuntu0.24.04.1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: administrator_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.administrator_sessions (
    id character varying(36) NOT NULL,
    administrator_id character varying(36) NOT NULL,
    token_hash text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    invalidated_at timestamp with time zone
);


--
-- Name: administrators; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.administrators (
    id character varying(36) NOT NULL,
    username character varying(64) NOT NULL,
    password_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: block_relations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.block_relations (
    id character varying(36) NOT NULL,
    blocker_id character varying(36) NOT NULL,
    blocked_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: comments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comments (
    id character varying(36) NOT NULL,
    post_id character varying(36) NOT NULL,
    author_id character varying(36) NOT NULL,
    body text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: follows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.follows (
    id character varying(36) NOT NULL,
    follower_id character varying(36) NOT NULL,
    followee_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: friend_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.friend_requests (
    id character varying(36) NOT NULL,
    requester_id character varying(36) NOT NULL,
    recipient_id character varying(36) NOT NULL,
    status text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


--
-- Name: friendships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.friendships (
    id character varying(36) NOT NULL,
    user_low_id character varying(36) NOT NULL,
    user_high_id character varying(36) NOT NULL,
    source_request_id character varying(36),
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
)
PARTITION BY RANGE (created_at);


--
-- Name: image_objects_202409; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202409 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202410; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202410 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202411; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202411 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202412; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202412 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202501; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202501 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202502; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202502 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202503; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202503 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202504; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202504 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202505; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202505 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202506; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202506 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202507; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202507 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202508; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202508 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202509; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202509 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202510; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202510 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202511; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202511 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202512; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202512 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202601; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202601 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202602; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202602 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202603; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202603 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202604; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202604 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202605; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202605 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202606; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202606 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202607; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202607 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202608; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202608 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202609; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202609 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202610; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202610 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_20261028; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_20261028 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202611; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202611 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202612; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202612 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202701; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202701 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202702; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202702 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202703; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202703 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202704; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202704 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202705; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202705 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202706; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_202706 (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_default; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.image_objects_default (
    id character varying(36) NOT NULL,
    owner_user_id character varying(36) NOT NULL,
    data bytea NOT NULL,
    media_type text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: initial_block_relations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.initial_block_relations (
    id character varying(36),
    blocker_id character varying(36),
    blocked_id character varying(36),
    created_at timestamp with time zone
);


--
-- Name: initial_data_boundary; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.initial_data_boundary (
    created_at timestamp with time zone
);


--
-- Name: initial_follows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.initial_follows (
    id character varying(36),
    follower_id character varying(36),
    followee_id character varying(36),
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: initial_friend_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.initial_friend_requests (
    id character varying(36),
    requester_id character varying(36),
    recipient_id character varying(36),
    status text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: initial_friendships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.initial_friendships (
    id character varying(36),
    user_low_id character varying(36),
    user_high_id character varying(36),
    source_request_id character varying(36),
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: initial_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.initial_users (
    id character varying(36),
    username character varying(64),
    secret_key_hash text,
    display_name text,
    bio text,
    profile_image_id character varying(36),
    region text,
    account_type text,
    status text,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: ip_bans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ip_bans (
    ip text NOT NULL,
    banned_until timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: login_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.login_sessions (
    id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    token_hash text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    invalidated_at timestamp with time zone,
    reason text
);


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    posting_window_id character varying(36) NOT NULL,
    created_at timestamp with time zone NOT NULL
);


--
-- Name: posting_windows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posting_windows (
    id character varying(36) NOT NULL,
    region text NOT NULL,
    local_date date NOT NULL,
    starts_at timestamp with time zone NOT NULL,
    on_time_deadline timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posts (
    id character varying(36) NOT NULL,
    author_id character varying(36) NOT NULL,
    posting_window_id character varying(36) NOT NULL,
    image_id character varying(36) NOT NULL,
    caption text DEFAULT ''::text NOT NULL,
    visibility text NOT NULL,
    location_kind text NOT NULL,
    latitude numeric,
    longitude numeric,
    locality text,
    posted_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: realmojis; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.realmojis (
    id character varying(36) NOT NULL,
    post_id character varying(36) NOT NULL,
    reactor_id character varying(36) NOT NULL,
    image_id character varying(36) NOT NULL,
    reaction_type text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    replaced_at timestamp with time zone,
    deleted_at timestamp with time zone
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id character varying(36) NOT NULL,
    username character varying(64) NOT NULL,
    secret_key_hash text NOT NULL,
    display_name text NOT NULL,
    bio text DEFAULT ''::text NOT NULL,
    profile_image_id character varying(36),
    region text NOT NULL,
    account_type text DEFAULT 'regular'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: image_objects_202409; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202409 FOR VALUES FROM ('2024-09-01 00:00:00+00') TO ('2024-10-01 00:00:00+00');


--
-- Name: image_objects_202410; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202410 FOR VALUES FROM ('2024-10-01 00:00:00+00') TO ('2024-11-01 00:00:00+00');


--
-- Name: image_objects_202411; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202411 FOR VALUES FROM ('2024-11-01 00:00:00+00') TO ('2024-12-01 00:00:00+00');


--
-- Name: image_objects_202412; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202412 FOR VALUES FROM ('2024-12-01 00:00:00+00') TO ('2025-01-01 00:00:00+00');


--
-- Name: image_objects_202501; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202501 FOR VALUES FROM ('2025-01-01 00:00:00+00') TO ('2025-02-01 00:00:00+00');


--
-- Name: image_objects_202502; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202502 FOR VALUES FROM ('2025-02-01 00:00:00+00') TO ('2025-03-01 00:00:00+00');


--
-- Name: image_objects_202503; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202503 FOR VALUES FROM ('2025-03-01 00:00:00+00') TO ('2025-04-01 00:00:00+00');


--
-- Name: image_objects_202504; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202504 FOR VALUES FROM ('2025-04-01 00:00:00+00') TO ('2025-05-01 00:00:00+00');


--
-- Name: image_objects_202505; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202505 FOR VALUES FROM ('2025-05-01 00:00:00+00') TO ('2025-06-01 00:00:00+00');


--
-- Name: image_objects_202506; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202506 FOR VALUES FROM ('2025-06-01 00:00:00+00') TO ('2025-07-01 00:00:00+00');


--
-- Name: image_objects_202507; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202507 FOR VALUES FROM ('2025-07-01 00:00:00+00') TO ('2025-08-01 00:00:00+00');


--
-- Name: image_objects_202508; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202508 FOR VALUES FROM ('2025-08-01 00:00:00+00') TO ('2025-09-01 00:00:00+00');


--
-- Name: image_objects_202509; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202509 FOR VALUES FROM ('2025-09-01 00:00:00+00') TO ('2025-10-01 00:00:00+00');


--
-- Name: image_objects_202510; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202510 FOR VALUES FROM ('2025-10-01 00:00:00+00') TO ('2025-11-01 00:00:00+00');


--
-- Name: image_objects_202511; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202511 FOR VALUES FROM ('2025-11-01 00:00:00+00') TO ('2025-12-01 00:00:00+00');


--
-- Name: image_objects_202512; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202512 FOR VALUES FROM ('2025-12-01 00:00:00+00') TO ('2026-01-01 00:00:00+00');


--
-- Name: image_objects_202601; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202601 FOR VALUES FROM ('2026-01-01 00:00:00+00') TO ('2026-02-01 00:00:00+00');


--
-- Name: image_objects_202602; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202602 FOR VALUES FROM ('2026-02-01 00:00:00+00') TO ('2026-03-01 00:00:00+00');


--
-- Name: image_objects_202603; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202603 FOR VALUES FROM ('2026-03-01 00:00:00+00') TO ('2026-04-01 00:00:00+00');


--
-- Name: image_objects_202604; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202604 FOR VALUES FROM ('2026-04-01 00:00:00+00') TO ('2026-05-01 00:00:00+00');


--
-- Name: image_objects_202605; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202605 FOR VALUES FROM ('2026-05-01 00:00:00+00') TO ('2026-06-01 00:00:00+00');


--
-- Name: image_objects_202606; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202606 FOR VALUES FROM ('2026-06-01 00:00:00+00') TO ('2026-07-01 00:00:00+00');


--
-- Name: image_objects_202607; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202607 FOR VALUES FROM ('2026-07-01 00:00:00+00') TO ('2026-08-01 00:00:00+00');


--
-- Name: image_objects_202608; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202608 FOR VALUES FROM ('2026-08-01 00:00:00+00') TO ('2026-09-01 00:00:00+00');


--
-- Name: image_objects_202609; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202609 FOR VALUES FROM ('2026-09-01 00:00:00+00') TO ('2026-10-01 00:00:00+00');


--
-- Name: image_objects_202610; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202610 FOR VALUES FROM ('2026-10-01 00:00:00+00') TO ('2026-10-28 06:00:00+00');


--
-- Name: image_objects_20261028; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_20261028 FOR VALUES FROM ('2026-10-28 06:00:00+00') TO ('2026-11-01 00:00:00+00');


--
-- Name: image_objects_202611; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202611 FOR VALUES FROM ('2026-11-01 00:00:00+00') TO ('2026-12-01 00:00:00+00');


--
-- Name: image_objects_202612; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202612 FOR VALUES FROM ('2026-12-01 00:00:00+00') TO ('2027-01-01 00:00:00+00');


--
-- Name: image_objects_202701; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202701 FOR VALUES FROM ('2027-01-01 00:00:00+00') TO ('2027-02-01 00:00:00+00');


--
-- Name: image_objects_202702; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202702 FOR VALUES FROM ('2027-02-01 00:00:00+00') TO ('2027-03-01 00:00:00+00');


--
-- Name: image_objects_202703; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202703 FOR VALUES FROM ('2027-03-01 00:00:00+00') TO ('2027-04-01 00:00:00+00');


--
-- Name: image_objects_202704; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202704 FOR VALUES FROM ('2027-04-01 00:00:00+00') TO ('2027-05-01 00:00:00+00');


--
-- Name: image_objects_202705; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202705 FOR VALUES FROM ('2027-05-01 00:00:00+00') TO ('2027-06-01 00:00:00+00');


--
-- Name: image_objects_202706; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_202706 FOR VALUES FROM ('2027-06-01 00:00:00+00') TO ('2027-07-01 00:00:00+00');


--
-- Name: image_objects_default; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects ATTACH PARTITION public.image_objects_default DEFAULT;


--
-- Name: administrator_sessions administrator_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.administrator_sessions
    ADD CONSTRAINT administrator_sessions_pkey PRIMARY KEY (id);


--
-- Name: administrators administrators_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.administrators
    ADD CONSTRAINT administrators_pkey PRIMARY KEY (id);


--
-- Name: block_relations block_relations_blocker_id_blocked_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.block_relations
    ADD CONSTRAINT block_relations_blocker_id_blocked_id_key UNIQUE (blocker_id, blocked_id);


--
-- Name: block_relations block_relations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.block_relations
    ADD CONSTRAINT block_relations_pkey PRIMARY KEY (id);


--
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);


--
-- Name: follows follows_follower_id_followee_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_follower_id_followee_id_key UNIQUE (follower_id, followee_id);


--
-- Name: follows follows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_pkey PRIMARY KEY (id);


--
-- Name: friend_requests friend_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.friend_requests
    ADD CONSTRAINT friend_requests_pkey PRIMARY KEY (id);


--
-- Name: friendships friendships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_pkey PRIMARY KEY (id);


--
-- Name: friendships friendships_user_low_id_user_high_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_user_low_id_user_high_id_key UNIQUE (user_low_id, user_high_id);


--
-- Name: image_objects image_objects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects
    ADD CONSTRAINT image_objects_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202409 image_objects_202409_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202409
    ADD CONSTRAINT image_objects_202409_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202410 image_objects_202410_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202410
    ADD CONSTRAINT image_objects_202410_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202411 image_objects_202411_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202411
    ADD CONSTRAINT image_objects_202411_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202412 image_objects_202412_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202412
    ADD CONSTRAINT image_objects_202412_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202501 image_objects_202501_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202501
    ADD CONSTRAINT image_objects_202501_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202502 image_objects_202502_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202502
    ADD CONSTRAINT image_objects_202502_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202503 image_objects_202503_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202503
    ADD CONSTRAINT image_objects_202503_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202504 image_objects_202504_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202504
    ADD CONSTRAINT image_objects_202504_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202505 image_objects_202505_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202505
    ADD CONSTRAINT image_objects_202505_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202506 image_objects_202506_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202506
    ADD CONSTRAINT image_objects_202506_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202507 image_objects_202507_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202507
    ADD CONSTRAINT image_objects_202507_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202508 image_objects_202508_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202508
    ADD CONSTRAINT image_objects_202508_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202509 image_objects_202509_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202509
    ADD CONSTRAINT image_objects_202509_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202510 image_objects_202510_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202510
    ADD CONSTRAINT image_objects_202510_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202511 image_objects_202511_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202511
    ADD CONSTRAINT image_objects_202511_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202512 image_objects_202512_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202512
    ADD CONSTRAINT image_objects_202512_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202601 image_objects_202601_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202601
    ADD CONSTRAINT image_objects_202601_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202602 image_objects_202602_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202602
    ADD CONSTRAINT image_objects_202602_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202603 image_objects_202603_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202603
    ADD CONSTRAINT image_objects_202603_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202604 image_objects_202604_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202604
    ADD CONSTRAINT image_objects_202604_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202605 image_objects_202605_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202605
    ADD CONSTRAINT image_objects_202605_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202606 image_objects_202606_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202606
    ADD CONSTRAINT image_objects_202606_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202607 image_objects_202607_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202607
    ADD CONSTRAINT image_objects_202607_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202608 image_objects_202608_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202608
    ADD CONSTRAINT image_objects_202608_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202609 image_objects_202609_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202609
    ADD CONSTRAINT image_objects_202609_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_20261028 image_objects_20261028_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_20261028
    ADD CONSTRAINT image_objects_20261028_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202610 image_objects_202610_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202610
    ADD CONSTRAINT image_objects_202610_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202611 image_objects_202611_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202611
    ADD CONSTRAINT image_objects_202611_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202612 image_objects_202612_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202612
    ADD CONSTRAINT image_objects_202612_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202701 image_objects_202701_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202701
    ADD CONSTRAINT image_objects_202701_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202702 image_objects_202702_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202702
    ADD CONSTRAINT image_objects_202702_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202703 image_objects_202703_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202703
    ADD CONSTRAINT image_objects_202703_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202704 image_objects_202704_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202704
    ADD CONSTRAINT image_objects_202704_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202705 image_objects_202705_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202705
    ADD CONSTRAINT image_objects_202705_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_202706 image_objects_202706_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_202706
    ADD CONSTRAINT image_objects_202706_pkey PRIMARY KEY (id, created_at);


--
-- Name: image_objects_default image_objects_default_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.image_objects_default
    ADD CONSTRAINT image_objects_default_pkey PRIMARY KEY (id, created_at);


--
-- Name: ip_bans ip_bans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ip_bans
    ADD CONSTRAINT ip_bans_pkey PRIMARY KEY (ip);


--
-- Name: login_sessions login_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.login_sessions
    ADD CONSTRAINT login_sessions_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_user_id_posting_window_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_posting_window_id_key UNIQUE (user_id, posting_window_id);


--
-- Name: posting_windows posting_windows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posting_windows
    ADD CONSTRAINT posting_windows_pkey PRIMARY KEY (id);


--
-- Name: posting_windows posting_windows_region_local_date_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posting_windows
    ADD CONSTRAINT posting_windows_region_local_date_key UNIQUE (region, local_date);


--
-- Name: posts posts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);


--
-- Name: realmojis realmojis_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realmojis
    ADD CONSTRAINT realmojis_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: comments_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX comments_post_id ON public.comments USING btree (post_id);


--
-- Name: posting_windows_one_active_region; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX posting_windows_one_active_region ON public.posting_windows USING btree (region) WHERE (ended_at IS NULL);


--
-- Name: posts_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_author_id ON public.posts USING btree (author_id);


--
-- Name: posts_posting_window_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_posting_window_id ON public.posts USING btree (posting_window_id);


--
-- Name: realmojis_one_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX realmojis_one_active ON public.realmojis USING btree (post_id, reactor_id) WHERE ((replaced_at IS NULL) AND (deleted_at IS NULL));


--
-- Name: realmojis_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX realmojis_post_id ON public.realmojis USING btree (post_id);


--
-- Name: image_objects_202409_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202409_pkey;


--
-- Name: image_objects_202410_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202410_pkey;


--
-- Name: image_objects_202411_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202411_pkey;


--
-- Name: image_objects_202412_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202412_pkey;


--
-- Name: image_objects_202501_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202501_pkey;


--
-- Name: image_objects_202502_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202502_pkey;


--
-- Name: image_objects_202503_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202503_pkey;


--
-- Name: image_objects_202504_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202504_pkey;


--
-- Name: image_objects_202505_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202505_pkey;


--
-- Name: image_objects_202506_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202506_pkey;


--
-- Name: image_objects_202507_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202507_pkey;


--
-- Name: image_objects_202508_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202508_pkey;


--
-- Name: image_objects_202509_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202509_pkey;


--
-- Name: image_objects_202510_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202510_pkey;


--
-- Name: image_objects_202511_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202511_pkey;


--
-- Name: image_objects_202512_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202512_pkey;


--
-- Name: image_objects_202601_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202601_pkey;


--
-- Name: image_objects_202602_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202602_pkey;


--
-- Name: image_objects_202603_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202603_pkey;


--
-- Name: image_objects_202604_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202604_pkey;


--
-- Name: image_objects_202605_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202605_pkey;


--
-- Name: image_objects_202606_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202606_pkey;


--
-- Name: image_objects_202607_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202607_pkey;


--
-- Name: image_objects_202608_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202608_pkey;


--
-- Name: image_objects_202609_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202609_pkey;


--
-- Name: image_objects_20261028_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_20261028_pkey;


--
-- Name: image_objects_202610_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202610_pkey;


--
-- Name: image_objects_202611_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202611_pkey;


--
-- Name: image_objects_202612_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202612_pkey;


--
-- Name: image_objects_202701_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202701_pkey;


--
-- Name: image_objects_202702_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202702_pkey;


--
-- Name: image_objects_202703_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202703_pkey;


--
-- Name: image_objects_202704_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202704_pkey;


--
-- Name: image_objects_202705_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202705_pkey;


--
-- Name: image_objects_202706_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_202706_pkey;


--
-- Name: image_objects_default_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.image_objects_pkey ATTACH PARTITION public.image_objects_default_pkey;


--
-- Name: administrator_sessions administrator_sessions_administrator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.administrator_sessions
    ADD CONSTRAINT administrator_sessions_administrator_id_fkey FOREIGN KEY (administrator_id) REFERENCES public.administrators(id) ON DELETE RESTRICT;


--
-- Name: block_relations block_relations_blocked_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.block_relations
    ADD CONSTRAINT block_relations_blocked_id_fkey FOREIGN KEY (blocked_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: block_relations block_relations_blocker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.block_relations
    ADD CONSTRAINT block_relations_blocker_id_fkey FOREIGN KEY (blocker_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: comments comments_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: comments comments_post_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.posts(id) ON DELETE RESTRICT;


--
-- Name: follows follows_followee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_followee_id_fkey FOREIGN KEY (followee_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: follows follows_follower_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_follower_id_fkey FOREIGN KEY (follower_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: friend_requests friend_requests_recipient_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.friend_requests
    ADD CONSTRAINT friend_requests_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: friend_requests friend_requests_requester_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.friend_requests
    ADD CONSTRAINT friend_requests_requester_id_fkey FOREIGN KEY (requester_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: friendships friendships_user_high_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_user_high_id_fkey FOREIGN KEY (user_high_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: friendships friendships_user_low_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_user_low_id_fkey FOREIGN KEY (user_low_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: image_objects image_objects_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.image_objects
    ADD CONSTRAINT image_objects_owner_fk FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: login_sessions login_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.login_sessions
    ADD CONSTRAINT login_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: notifications notifications_posting_window_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_posting_window_id_fkey FOREIGN KEY (posting_window_id) REFERENCES public.posting_windows(id) ON DELETE RESTRICT;


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: posts posts_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT posts_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: posts posts_posting_window_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT posts_posting_window_id_fkey FOREIGN KEY (posting_window_id) REFERENCES public.posting_windows(id) ON DELETE RESTRICT;


--
-- Name: realmojis realmojis_post_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realmojis
    ADD CONSTRAINT realmojis_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.posts(id) ON DELETE RESTRICT;


--
-- Name: realmojis realmojis_reactor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realmojis
    ADD CONSTRAINT realmojis_reactor_id_fkey FOREIGN KEY (reactor_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- PostgreSQL database dump complete
--

\unrestrict z9qVZJV8P9b0X229pJ12bYQFzP69kyzvdewe6oT6qRWm9ZodM8INPoefu5E8dBm

