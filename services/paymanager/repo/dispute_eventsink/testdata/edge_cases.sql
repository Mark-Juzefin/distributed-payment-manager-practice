-- Test data for edge cases and boundary conditions
-- Contains minimal data for testing edge cases like empty results, single events, etc.

INSERT INTO orders (id, user_id, status, on_hold, hold_reason, created_at, updated_at) VALUES
('edge_order_001', '550e8400-e29b-41d4-a716-446655440300', 'completed', false, NULL, '2024-04-01 10:00:00', '2024-04-01 10:30:00'),
('edge_order_002', '550e8400-e29b-41d4-a716-446655440301', 'completed', false, NULL, '2024-04-01 11:00:00', '2024-04-01 11:30:00')
    ON CONFLICT (id) DO NOTHING;

INSERT INTO disputes (id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at) VALUES
('edge_dispute_001', 'edge_order_001', 'edge_sub_001', 'open', 'Edge case dispute - single event', 99.99, 'USD', '2024-04-01 10:00:00', '2024-04-08 23:59:59', NULL, NULL),
('edge_dispute_002', 'edge_order_002', 'edge_sub_002', 'open', 'Edge case dispute - no events', 199.99, 'USD', '2024-04-01 11:00:00', '2024-04-08 23:59:59', NULL, NULL)
    ON CONFLICT (id) DO NOTHING;

-- Single event for testing single result scenarios
INSERT INTO dispute_events (id, dispute_id, kind, provider_event_id, data, created_at) VALUES
('edge_event_001', 'edge_dispute_001', 'webhook_opened', 'edge_provider_001', '{"test": "single_event", "amount": 99.99}', '2024-04-01 10:00:00')
    ON CONFLICT (id, created_at) DO NOTHING;

-- Note: edge_dispute_002 deliberately has no events for testing empty result scenarios