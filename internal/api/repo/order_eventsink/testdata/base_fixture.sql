-- Base fixture for order_eventsink integration tests
-- Creates test orders that events can reference

INSERT INTO orders (id, user_id, status, created_at, updated_at, on_hold, hold_reason)
VALUES
    ('order_001', '00000000-0000-0000-0000-000000000001', 'created', '2024-01-15 09:00:00', '2024-01-15 09:00:00', false, NULL),
    ('order_002', '00000000-0000-0000-0000-000000000002', 'created', '2024-01-15 09:30:00', '2024-01-15 09:30:00', false, NULL),
    ('order_003', '00000000-0000-0000-0000-000000000001', 'updated', '2024-01-15 10:00:00', '2024-01-15 10:30:00', false, NULL);
