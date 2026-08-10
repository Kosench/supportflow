BEGIN;

CREATE TABLE ticket.tickets (
                                id UUID PRIMARY KEY,
                                requester_id UUID NOT NULL
                                    REFERENCES ticket.users (id),
                                category_id UUID NOT NULL
                                    REFERENCES ticket.categories (id),
                                assignee_id UUID
                                    REFERENCES ticket.users (id),

                                title VARCHAR(200) NOT NULL,
                                description TEXT NOT NULL,
                                priority TEXT NOT NULL,
                                status TEXT NOT NULL,
                                waiting_reason VARCHAR(1000) NOT NULL DEFAULT '',
                                resolution VARCHAR(4000) NOT NULL DEFAULT '',

                                version BIGINT NOT NULL,

                                created_at TIMESTAMPTZ NOT NULL,
                                updated_at TIMESTAMPTZ NOT NULL,
                                resolution_started_at TIMESTAMPTZ NOT NULL,
                                first_response_deadline_at TIMESTAMPTZ NOT NULL,
                                resolution_deadline_at TIMESTAMPTZ NOT NULL,
                                first_responded_at TIMESTAMPTZ,
                                resolved_at TIMESTAMPTZ,
                                closed_at TIMESTAMPTZ,

                                search_vector TSVECTOR GENERATED ALWAYS AS (
                                    setweight(
                                            to_tsvector('simple', coalesce(title, '')),
                                            'A'
                                    )
                                        ||
                                    setweight(
                                            to_tsvector('simple', coalesce(description, '')),
                                            'B'
                                    )
                                    ) STORED,

                                CONSTRAINT tickets_title_length
                                    CHECK (char_length(btrim(title)) BETWEEN 5 AND 200),
                                CONSTRAINT tickets_description_length
                                    CHECK (char_length(btrim(description)) BETWEEN 10 AND 10000),
                                CONSTRAINT tickets_priority_valid
                                    CHECK (priority IN ('LOW', 'NORMAL', 'HIGH', 'CRITICAL')),
                                CONSTRAINT tickets_status_valid
                                    CHECK (
                                        status IN (
                                                   'NEW',
                                                   'ASSIGNED',
                                                   'IN_PROGRESS',
                                                   'WAITING_CUSTOMER',
                                                   'RESOLVED',
                                                   'CLOSED',
                                                   'REOPENED',
                                                   'CANCELLED'
                                            )
                                        ),
                                CONSTRAINT tickets_version_positive
                                    CHECK (version >= 1),
                                CONSTRAINT tickets_active_assignee_required
                                    CHECK (
                                        status NOT IN (
                                                       'ASSIGNED',
                                                       'IN_PROGRESS',
                                                       'WAITING_CUSTOMER'
                                            )
                                            OR assignee_id IS NOT NULL
                                        ),
                                CONSTRAINT tickets_new_cancelled_without_assignee
                                    CHECK (
                                        status NOT IN ('NEW', 'CANCELLED')
                                            OR assignee_id IS NULL
                                        ),
                                CONSTRAINT tickets_waiting_reason_shape
                                    CHECK (
                                        (
                                            status = 'WAITING_CUSTOMER'
                                                AND char_length(btrim(waiting_reason)) BETWEEN 3 AND 1000
                                            )
                                            OR
                                        (
                                            status <> 'WAITING_CUSTOMER'
                                                AND waiting_reason = ''
                                            )
                                        ),
                                CONSTRAINT tickets_resolution_shape
                                    CHECK (
                                        (
                                            status IN ('RESOLVED', 'CLOSED')
                                                AND resolved_at IS NOT NULL
                                                AND char_length(btrim(resolution)) BETWEEN 3 AND 4000
                                            )
                                            OR
                                        (
                                            status NOT IN ('RESOLVED', 'CLOSED')
                                                AND resolved_at IS NULL
                                                AND resolution = ''
                                            )
                                        ),
                                CONSTRAINT tickets_closed_shape
                                    CHECK (
                                        (
                                            status = 'CLOSED'
                                                AND closed_at IS NOT NULL
                                            )
                                            OR
                                        (
                                            status <> 'CLOSED'
                                                AND closed_at IS NULL
                                            )
                                        ),
                                CONSTRAINT tickets_timestamps_ordered
                                    CHECK (
                                        updated_at >= created_at
                                            AND resolution_started_at >= created_at
                                            AND first_response_deadline_at > created_at
                                            AND resolution_deadline_at > resolution_started_at
                                            AND (
                                            first_responded_at IS NULL
                                                OR first_responded_at >= created_at
                                            )
                                            AND (
                                            resolved_at IS NULL
                                                OR resolved_at >= created_at
                                            )
                                            AND (
                                            closed_at IS NULL
                                                OR (
                                                resolved_at IS NOT NULL
                                                    AND closed_at >= resolved_at
                                                )
                                            )
                                        )
);

CREATE TABLE ticket.comments (
                                 id UUID PRIMARY KEY,
                                 ticket_id UUID NOT NULL
                                     REFERENCES ticket.tickets (id),
                                 author_id UUID NOT NULL
                                     REFERENCES ticket.users (id),
                                 visibility TEXT NOT NULL,
                                 body TEXT NOT NULL,
                                 created_at TIMESTAMPTZ NOT NULL,

                                 CONSTRAINT comments_id_ticket_unique
                                     UNIQUE (id, ticket_id),
                                 CONSTRAINT comments_visibility_valid
                                     CHECK (visibility IN ('PUBLIC', 'INTERNAL')),
                                 CONSTRAINT comments_body_length
                                     CHECK (char_length(btrim(body)) BETWEEN 1 AND 10000)
);

CREATE TABLE ticket.attachments (
                                    id UUID PRIMARY KEY,
                                    ticket_id UUID NOT NULL
                                        REFERENCES ticket.tickets (id),
                                    comment_id UUID,
                                    uploaded_by UUID NOT NULL
                                        REFERENCES ticket.users (id),

                                    object_key TEXT NOT NULL UNIQUE,
                                    file_name TEXT NOT NULL,
                                    content_type TEXT NOT NULL,
                                    size_bytes BIGINT NOT NULL,
                                    checksum_sha256 CHAR(64),
                                    status TEXT NOT NULL,

                                    created_at TIMESTAMPTZ NOT NULL,
                                    uploaded_at TIMESTAMPTZ,

                                    CONSTRAINT attachments_comment_fk
                                        FOREIGN KEY (comment_id, ticket_id)
                                            REFERENCES ticket.comments (id, ticket_id),
                                    CONSTRAINT attachments_object_key_not_blank
                                        CHECK (btrim(object_key) <> ''),
                                    CONSTRAINT attachments_file_name_length
                                        CHECK (char_length(btrim(file_name)) BETWEEN 1 AND 255),
                                    CONSTRAINT attachments_content_type_length
                                        CHECK (char_length(btrim(content_type)) BETWEEN 1 AND 255),
                                    CONSTRAINT attachments_size_valid
                                        CHECK (size_bytes BETWEEN 1 AND 52428800),
                                    CONSTRAINT attachments_status_valid
                                        CHECK (status IN ('PENDING', 'READY', 'FAILED')),
                                    CONSTRAINT attachments_ready_shape
                                        CHECK (
                                            (
                                                status = 'READY'
                                                    AND uploaded_at IS NOT NULL
                                                    AND checksum_sha256 ~ '^[0-9a-f]{64}$'
                                                )
                                                OR
                                            (
                                                status <> 'READY'
                                                    AND uploaded_at IS NULL
                                                    AND checksum_sha256 IS NULL
                                                )
                                            ),
                                    CONSTRAINT attachments_uploaded_after_creation
                                        CHECK (
                                            uploaded_at IS NULL
                                                OR uploaded_at >= created_at
                                            )
);

CREATE TABLE ticket.ticket_history (
                                       id UUID PRIMARY KEY,
                                       ticket_id UUID NOT NULL
                                           REFERENCES ticket.tickets (id),
                                       action_type TEXT NOT NULL,
                                       actor_id UUID,
                                       actor_role TEXT NOT NULL,
                                       ticket_version BIGINT NOT NULL,
                                       old_values JSONB NOT NULL DEFAULT '{}'::jsonb,
                                       new_values JSONB NOT NULL DEFAULT '{}'::jsonb,
                                       reason TEXT,
                                       correlation_id UUID NOT NULL,
                                       created_at TIMESTAMPTZ NOT NULL,

                                       CONSTRAINT ticket_history_action_not_blank
                                           CHECK (btrim(action_type) <> ''),
                                       CONSTRAINT ticket_history_actor_role_valid
                                           CHECK (
                                               actor_role IN (
                                                              'SYSTEM',
                                                              'USER',
                                                              'OPERATOR',
                                                              'MANAGER',
                                                              'ADMIN'
                                                   )
                                               ),
                                       CONSTRAINT ticket_history_actor_shape
                                           CHECK (
                                               (
                                                   actor_role = 'SYSTEM'
                                                       AND actor_id IS NULL
                                                   )
                                                   OR
                                               (
                                                   actor_role <> 'SYSTEM'
                                                       AND actor_id IS NOT NULL
                                                   )
                                               ),
                                       CONSTRAINT ticket_history_version_positive
                                           CHECK (ticket_version >= 1),
                                       CONSTRAINT ticket_history_old_values_object
                                           CHECK (jsonb_typeof(old_values) = 'object'),
                                       CONSTRAINT ticket_history_new_values_object
                                           CHECK (jsonb_typeof(new_values) = 'object')
);

COMMIT;