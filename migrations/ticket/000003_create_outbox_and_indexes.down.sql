BEGIN;

DROP INDEX ticket.outbox_aggregate_idx;
DROP INDEX ticket.outbox_pending_idx;

DROP INDEX ticket.ticket_history_ticket_version_idx;
DROP INDEX ticket.attachments_ticket_created_idx;
DROP INDEX ticket.comments_ticket_created_idx;
DROP INDEX ticket.tickets_search_vector_idx;
DROP INDEX ticket.tickets_resolution_deadline_idx;
DROP INDEX ticket.tickets_first_response_deadline_idx;
DROP INDEX ticket.tickets_category_created_idx;
DROP INDEX ticket.tickets_unassigned_queue_idx;
DROP INDEX ticket.tickets_assignee_queue_idx;
DROP INDEX ticket.tickets_requester_created_idx;
DROP INDEX ticket.operator_skills_category_idx;
DROP INDEX ticket.operator_profiles_available_idx;
DROP INDEX ticket.users_email_lower_unique;

DROP TABLE ticket.outbox_events;

COMMIT;