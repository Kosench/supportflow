BEGIN;

CREATE SCHEMA ticket;

CREATE TABLE ticket.users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT users_email_not_blank
        CHECK (btrim(email) <> ''),
    CONSTRAINT users_email_has_at_sign
        CHECK (position('@' IN email) > 1),
    CONSTRAINT users_password_hash_not_blank
        CHECK (btrim(password_hash) <> ''),
    CONSTRAINT users_display_name_length
        CHECK (char_length(btrim(display_name)) BETWEEN 2 AND 200),
    CONSTRAINT users_role_valid
        CHECK (role IN ('USER', 'OPERATOR', 'MANAGER', 'ADMIN')),
    CONSTRAINT users_timestamps_ordered
        CHECK (updated_at >= created_at)

);

CREATE TABLE ticket.categories (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT categories_code_format
        CHECK (code ~ '^[a-z][a-z0-9_]{1,49}$'),
    CONSTRAINT categories_name_length
        CHECK (char_length(btrim(name)) BETWEEN 2 AND 100),
    CONSTRAINT categories_description_length
        CHECK (char_length(description) <= 1000),
    CONSTRAINT categories_timestamps_ordered
        CHECK (updated_at >= created_at)
);

CREATE TABLE ticket.operator_profiles (
    user_id UUID PRIMARY KEY
    REFERENCES ticket.users (id),
    is_available BOOLEAN NOT NULL DEFAULT FALSE,
    max_active_tickets INTEGER NOT NULL DEFAULT 10,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    last_assigned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT operator_profiles_capacity_positive
        CHECK (max_active_tickets > 0),
    CONSTRAINT operator_profiles_capacity_reasonable
        CHECK (max_active_tickets <= 1000),
    CONSTRAINT operator_profiles_timezone_not_blank
        CHECK (btrim(timezone) <> ''),
    CONSTRAINT operator_profiles_timestamps_ordered
        CHECK (updated_at >= created_at)
);

CREATE TABLE ticket.operator_skills (
    operator_id UUID NOT NULL
        REFERENCES ticket.operator_profiles (user_id),
    category_id UUID NOT NULL
        REFERENCES ticket.categories (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (operator_id, category_id)
);

CREATE TABLE ticket.operator_schedules (
    operator_id UUID NOT NULL
        REFERENCES ticket.operator_profiles (user_id),
    weekday SMALLINT NOT NULL,
    starts_at TIME NOT NULL,
    ends_at TIME NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (operator_id, weekday),

    CONSTRAINT operator_schedules_weekday_valid
        CHECK (weekday BETWEEN 0 AND 6),
    CONSTRAINT operator_schedules_interval_valid
        CHECK (starts_at < ends_at),
    CONSTRAINT operator_schedules_timestamps_ordered
        CHECK (updated_at >= created_at)
);

CREATE TABLE ticket.sla_policies (
    priority TEXT PRIMARY KEY,
    first_response_interval INTERVAL NOT NULL,
    resolution_interval INTERVAL NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    updated_by UUID
        REFERENCES ticket.users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT sla_policies_priority_valid
        CHECK (priority IN ('LOW', 'NORMAL', 'HIGH', 'CRITICAL')),
    CONSTRAINT sla_policies_first_response_positive
        CHECK (first_response_interval > INTERVAL '0 seconds'),
    CONSTRAINT sla_policies_resolution_positive
        CHECK (resolution_interval > INTERVAL '0 seconds'),
    CONSTRAINT sla_policies_intervals_ordered
        CHECK (resolution_interval >= first_response_interval),
    CONSTRAINT sla_policies_version_positive
        CHECK (version >= 1),
    CONSTRAINT sla_policies_timestamps_ordered
        CHECK (updated_at >= created_at)
);

COMMIT;