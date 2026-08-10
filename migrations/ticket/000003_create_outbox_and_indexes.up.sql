BEGIN;

CREATE TABLE ticket.outbox_events (
                                      event_id UUID PRIMARY KEY,
                                      event_type TEXT NOT NULL,
                                      event_version SMALLINT NOT NULL,

                                      aggregate_type TEXT NOT NULL,
                                      aggregate_id UUID NOT NULL,
                                      aggregate_version BIGINT NOT NULL,

                                      correlation_id UUID NOT NULL,
                                      causation_id UUID,
                                      trace_id VARCHAR(32),

                                      payload JSONB NOT NULL,
                                      occurred_at TIMESTAMPTZ NOT NULL,
                                      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

                                      attempts INTEGER NOT NULL DEFAULT 0,
                                      next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                      published_at TIMESTAMPTZ,
                                      last_error TEXT,

                                      CONSTRAINT outbox_event_type_not_blank
                                          CHECK (btrim(event_type) <> ''),
                                      CONSTRAINT outbox_event_version_positive
                                          CHECK (event_version >= 1),
                                      CONSTRAINT outbox_aggregate_type_not_blank
                                          CHECK (btrim(aggregate_type) <> ''),
                                      CONSTRAINT outbox_aggregate_version_positive
                                          CHECK (aggregate_version >= 1),
                                      CONSTRAINT outbox_trace_id_valid
                                          CHECK (
                                              trace_id IS NULL
                                                  OR trace_id ~ '^[0-9a-f]{32}$'
),
    CONSTRAINT outbox_payload_object
        CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT outbox_attempts_non_negative
        CHECK (attempts >= 0),
    CONSTRAINT outbox_published_after_creation
        CHECK (
            published_at IS NULL
            OR published_at >= created_at
        ),
    CONSTRAINT outbox_business_event_unique
        UNIQUE (
            aggregate_type,
            aggregate_id,
            aggregate_version,
            event_type
        )
);

CREATE UNIQUE INDEX users_email_lower_unique
    ON ticket.users (lower(email));

CREATE INDEX operator_profiles_available_idx
    ON ticket.operator_profiles (
                                 last_assigned_at ASC NULLS FIRST,
                                 user_id
        )
    WHERE is_available = TRUE;

CREATE INDEX operator_skills_category_idx
    ON ticket.operator_skills (category_id, operator_id);

CREATE INDEX tickets_requester_created_idx
    ON ticket.tickets (requester_id, created_at DESC, id DESC);

CREATE INDEX tickets_assignee_queue_idx
    ON ticket.tickets (
                       assignee_id,
                       status,
                       priority,
                       created_at,
                       id
        )
    WHERE assignee_id IS NOT NULL
      AND status IN (
          'ASSIGNED',
          'IN_PROGRESS',
          'WAITING_CUSTOMER',
          'REOPENED'
      );

CREATE INDEX tickets_unassigned_queue_idx
    ON ticket.tickets (priority, created_at, id)
    WHERE assignee_id IS NULL
      AND status IN ('NEW', 'REOPENED');

CREATE INDEX tickets_category_created_idx
    ON ticket.tickets (category_id, created_at DESC, id DESC);

CREATE INDEX tickets_first_response_deadline_idx
    ON ticket.tickets (first_response_deadline_at, id)
    WHERE first_responded_at IS NULL
      AND status NOT IN ('CLOSED', 'CANCELLED');

CREATE INDEX tickets_resolution_deadline_idx
    ON ticket.tickets (resolution_deadline_at, id)
    WHERE resolved_at IS NULL
      AND status NOT IN ('CLOSED', 'CANCELLED');

CREATE INDEX tickets_search_vector_idx
    ON ticket.tickets
    USING GIN (search_vector);

CREATE INDEX comments_ticket_created_idx
    ON ticket.comments (ticket_id, created_at, id);

CREATE INDEX attachments_ticket_created_idx
    ON ticket.attachments (ticket_id, created_at, id);

CREATE INDEX ticket_history_ticket_version_idx
    ON ticket.ticket_history (
                              ticket_id,
                              ticket_version,
                              created_at,
                              id
        );

CREATE INDEX outbox_pending_idx
    ON ticket.outbox_events (next_attempt_at, occurred_at, event_id)
    WHERE published_at IS NULL;

CREATE INDEX outbox_aggregate_idx
    ON ticket.outbox_events (
                             aggregate_type,
                             aggregate_id,
                             aggregate_version
        );

COMMIT;