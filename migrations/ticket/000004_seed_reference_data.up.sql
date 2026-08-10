BEGIN;

INSERT INTO ticket.categories (
    id,
    code,
    name,
    description
)
VALUES
    (
        '019d0000-0000-7000-8000-000000000001',
        'vpn',
        'VPN',
        'Problems with corporate VPN access'
    ),
    (
        '019d0000-0000-7000-8000-000000000002',
        'account_access',
        'Account access',
        'Authentication and account access problems'
    ),
    (
        '019d0000-0000-7000-8000-000000000003',
        'hardware',
        'Hardware',
        'Workstation and peripheral equipment requests'
    ),
    (
        '019d0000-0000-7000-8000-000000000004',
        'software',
        'Software',
        'Application installation and software incidents'
    ),
    (
        '019d0000-0000-7000-8000-000000000005',
        'other',
        'Other',
        'Requests that do not match another category'
    );

INSERT INTO ticket.sla_policies (
    priority,
    first_response_interval,
    resolution_interval
)
VALUES
    ('LOW', INTERVAL '8 hours', INTERVAL '72 hours'),
    ('NORMAL', INTERVAL '2 hours', INTERVAL '24 hours'),
    ('HIGH', INTERVAL '30 minutes', INTERVAL '8 hours'),
    ('CRITICAL', INTERVAL '10 minutes', INTERVAL '2 hours');

COMMIT;