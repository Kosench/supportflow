BEGIN;

DROP TABLE ticket.sla_policies;
DROP TABLE ticket.operator_schedules;
DROP TABLE ticket.operator_skills;
DROP TABLE ticket.operator_profiles;
DROP TABLE ticket.categories;
DROP TABLE ticket.users;

DROP SCHEMA ticket;

COMMIT;