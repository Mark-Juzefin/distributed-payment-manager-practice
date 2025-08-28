-- Test data for filtering scenarios
-- Creates events across different time periods and event kinds for filtering tests

INSERT INTO orders (id, user_id, status, created_at, updated_at) VALUES
('filter_order_001', '550e8400-e29b-41d4-a716-446655440200', 'completed', '2024-02-01 10:00:00', '2024-02-01 10:30:00'),
('filter_order_002', '550e8400-e29b-41d4-a716-446655440201', 'completed', '2024-02-02 10:00:00', '2024-02-02 10:30:00'),
('filter_order_003', '550e8400-e29b-41d4-a716-446655440202', 'completed', '2024-02-03 10:00:00', '2024-02-03 10:30:00')
    ON CONFLICT (id) DO NOTHING;

INSERT INTO disputes (id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at) VALUES
('filter_dispute_001', 'filter_order_001', 'filter_sub_001', 'won', 'Filter test dispute 1', 150.00, 'USD', '2024-02-01 10:00:00', '2024-02-08 23:59:59', '2024-02-02 10:00:00', '2024-02-08 15:00:00'),
('filter_dispute_002', 'filter_order_002', 'filter_sub_002', 'lost', 'Filter test dispute 2', 250.00, 'EUR', '2024-02-02 10:00:00', '2024-02-09 23:59:59', '2024-02-03 10:00:00', '2024-02-09 16:00:00'),
('filter_dispute_003', 'filter_order_003', 'filter_sub_003', 'open', 'Filter test dispute 3', 350.00, 'GBP', '2024-02-03 10:00:00', '2024-02-10 23:59:59', NULL, NULL)
    ON CONFLICT (id) DO NOTHING;

-- Events for time range and kind filtering tests
INSERT INTO dispute_events (id, dispute_id, kind, provider_event_id, data, created_at) VALUES
-- Week 1 events (2024-02-01 to 2024-02-07)
('filter_event_001', 'filter_dispute_001', 'webhook_opened', 'filter_provider_001', '{"test": "week1_opened"}', '2024-02-01 09:00:00'),
('filter_event_002', 'filter_dispute_001', 'webhook_updated', 'filter_provider_002', '{"test": "week1_updated"}', '2024-02-02 09:00:00'),
('filter_event_003', 'filter_dispute_001', 'evidence_added', 'filter_provider_003', '{"test": "week1_evidence"}', '2024-02-03 09:00:00'),
('filter_event_004', 'filter_dispute_001', 'evidence_submitted', 'filter_provider_004', '{"test": "week1_submitted"}', '2024-02-04 09:00:00'),
('filter_event_005', 'filter_dispute_001', 'provider_decision', 'filter_provider_005', '{"test": "week1_decision"}', '2024-02-05 09:00:00'),

-- Week 2 events (2024-02-08 to 2024-02-14) 
('filter_event_006', 'filter_dispute_002', 'webhook_opened', 'filter_provider_006', '{"test": "week2_opened"}', '2024-02-08 09:00:00'),
('filter_event_007', 'filter_dispute_002', 'webhook_updated', 'filter_provider_007', '{"test": "week2_updated"}', '2024-02-09 09:00:00'),
('filter_event_008', 'filter_dispute_002', 'evidence_added', 'filter_provider_008', '{"test": "week2_evidence"}', '2024-02-10 09:00:00'),
('filter_event_009', 'filter_dispute_002', 'evidence_submitted', 'filter_provider_009', '{"test": "week2_submitted"}', '2024-02-11 09:00:00'),
('filter_event_010', 'filter_dispute_002', 'provider_decision', 'filter_provider_010', '{"test": "week2_decision"}', '2024-02-12 09:00:00'),

-- Week 3 events (2024-02-15 to 2024-02-21)
('filter_event_011', 'filter_dispute_003', 'webhook_opened', 'filter_provider_011', '{"test": "week3_opened"}', '2024-02-15 09:00:00'),
('filter_event_012', 'filter_dispute_003', 'webhook_updated', 'filter_provider_012', '{"test": "week3_updated"}', '2024-02-16 09:00:00'),
('filter_event_013', 'filter_dispute_003', 'evidence_added', 'filter_provider_013', '{"test": "week3_evidence"}', '2024-02-17 09:00:00'),

-- Additional events for kind filtering tests - multiple events of same kind
('filter_event_014', 'filter_dispute_001', 'evidence_added', 'filter_provider_014', '{"test": "additional_evidence_1"}', '2024-02-06 09:00:00'),
('filter_event_015', 'filter_dispute_002', 'evidence_added', 'filter_provider_015', '{"test": "additional_evidence_2"}', '2024-02-13 09:00:00'),
('filter_event_016', 'filter_dispute_003', 'evidence_added', 'filter_provider_016', '{"test": "additional_evidence_3"}', '2024-02-18 09:00:00'),

-- Multiple webhook_updated events
('filter_event_017', 'filter_dispute_001', 'webhook_updated', 'filter_provider_017', '{"test": "additional_updated_1"}', '2024-02-06 10:00:00'),
('filter_event_018', 'filter_dispute_002', 'webhook_updated', 'filter_provider_018', '{"test": "additional_updated_2"}', '2024-02-13 10:00:00'),
('filter_event_019', 'filter_dispute_003', 'webhook_updated', 'filter_provider_019', '{"test": "additional_updated_3"}', '2024-02-18 10:00:00')
    ON CONFLICT (id) DO NOTHING;