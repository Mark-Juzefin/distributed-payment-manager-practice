-- Test data for pagination scenarios
-- Creates 25 events across 5 disputes with predictable timestamps for testing cursor pagination

INSERT INTO orders (id, user_id, status, created_at, updated_at) VALUES
('page_order_001', '550e8400-e29b-41d4-a716-446655440100', 'completed', '2024-03-01 10:00:00', '2024-03-01 10:30:00'),
('page_order_002', '550e8400-e29b-41d4-a716-446655440101', 'completed', '2024-03-01 11:00:00', '2024-03-01 11:30:00'),
('page_order_003', '550e8400-e29b-41d4-a716-446655440102', 'completed', '2024-03-01 12:00:00', '2024-03-01 12:30:00'),
('page_order_004', '550e8400-e29b-41d4-a716-446655440103', 'completed', '2024-03-01 13:00:00', '2024-03-01 13:30:00'),
('page_order_005', '550e8400-e29b-41d4-a716-446655440104', 'completed', '2024-03-01 14:00:00', '2024-03-01 14:30:00')
    ON CONFLICT (id) DO NOTHING;

INSERT INTO disputes (id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at) VALUES
('page_dispute_001', 'page_order_001', 'page_sub_001', 'won', 'Test dispute 1', 100.00, 'USD', '2024-03-01 10:00:00', '2024-03-08 23:59:59', '2024-03-02 10:00:00', '2024-03-08 15:00:00'),
('page_dispute_002', 'page_order_002', 'page_sub_002', 'lost', 'Test dispute 2', 200.00, 'USD', '2024-03-01 11:00:00', '2024-03-08 23:59:59', '2024-03-02 11:00:00', '2024-03-08 16:00:00'),
('page_dispute_003', 'page_order_003', 'page_sub_003', 'won', 'Test dispute 3', 300.00, 'USD', '2024-03-01 12:00:00', '2024-03-08 23:59:59', '2024-03-02 12:00:00', '2024-03-08 17:00:00'),
('page_dispute_004', 'page_order_004', 'page_sub_004', 'open', 'Test dispute 4', 400.00, 'USD', '2024-03-01 13:00:00', '2024-03-08 23:59:59', NULL, NULL),
('page_dispute_005', 'page_order_005', 'page_sub_005', 'submitted', 'Test dispute 5', 500.00, 'USD', '2024-03-01 14:00:00', '2024-03-08 23:59:59', '2024-03-02 14:00:00', NULL)
    ON CONFLICT (id) DO NOTHING;

-- Create 25 events with predictable timestamps for pagination testing
-- Events every 10 minutes starting from 2024-03-01 08:00:00
INSERT INTO dispute_events (id, dispute_id, kind, provider_event_id, data, created_at) VALUES
-- Dispute 1 events (5 events)
('page_event_001', 'page_dispute_001', 'webhook_opened', 'page_provider_001', '{"amount": 100.00, "currency": "USD"}', '2024-03-01 08:00:00'),
('page_event_002', 'page_dispute_001', 'webhook_updated', 'page_provider_002', '{"status": "evidence_required"}', '2024-03-01 08:10:00'),
('page_event_003', 'page_dispute_001', 'evidence_added', 'page_provider_003', '{"document_type": "receipt"}', '2024-03-01 08:20:00'),
('page_event_004', 'page_dispute_001', 'evidence_submitted', 'page_provider_004', '{"submission_id": "sub_001"}', '2024-03-01 08:30:00'),
('page_event_005', 'page_dispute_001', 'provider_decision', 'page_provider_005', '{"outcome": "won"}', '2024-03-01 08:40:00'),

-- Dispute 2 events (5 events)
('page_event_006', 'page_dispute_002', 'webhook_opened', 'page_provider_006', '{"amount": 200.00, "currency": "USD"}', '2024-03-01 08:50:00'),
('page_event_007', 'page_dispute_002', 'webhook_updated', 'page_provider_007', '{"status": "evidence_required"}', '2024-03-01 09:00:00'),
('page_event_008', 'page_dispute_002', 'evidence_added', 'page_provider_008', '{"document_type": "photos"}', '2024-03-01 09:10:00'),
('page_event_009', 'page_dispute_002', 'evidence_submitted', 'page_provider_009', '{"submission_id": "sub_002"}', '2024-03-01 09:20:00'),
('page_event_010', 'page_dispute_002', 'provider_decision', 'page_provider_010', '{"outcome": "lost"}', '2024-03-01 09:30:00'),

-- Dispute 3 events (5 events)
('page_event_011', 'page_dispute_003', 'webhook_opened', 'page_provider_011', '{"amount": 300.00, "currency": "USD"}', '2024-03-01 09:40:00'),
('page_event_012', 'page_dispute_003', 'webhook_updated', 'page_provider_012', '{"status": "evidence_required"}', '2024-03-01 09:50:00'),
('page_event_013', 'page_dispute_003', 'evidence_added', 'page_provider_013', '{"document_type": "contract"}', '2024-03-01 10:00:00'),
('page_event_014', 'page_dispute_003', 'evidence_submitted', 'page_provider_014', '{"submission_id": "sub_003"}', '2024-03-01 10:10:00'),
('page_event_015', 'page_dispute_003', 'provider_decision', 'page_provider_015', '{"outcome": "won"}', '2024-03-01 10:20:00'),

-- Dispute 4 events (5 events)
('page_event_016', 'page_dispute_004', 'webhook_opened', 'page_provider_016', '{"amount": 400.00, "currency": "USD"}', '2024-03-01 10:30:00'),
('page_event_017', 'page_dispute_004', 'webhook_updated', 'page_provider_017', '{"status": "evidence_required"}', '2024-03-01 10:40:00'),
('page_event_018', 'page_dispute_004', 'evidence_added', 'page_provider_018', '{"document_type": "invoice"}', '2024-03-01 10:50:00'),
('page_event_019', 'page_dispute_004', 'evidence_added', 'page_provider_019', '{"document_type": "shipping"}', '2024-03-01 11:00:00'),
('page_event_020', 'page_dispute_004', 'evidence_submitted', 'page_provider_020', '{"submission_id": "sub_004"}', '2024-03-01 11:10:00'),

-- Dispute 5 events (5 events)
('page_event_021', 'page_dispute_005', 'webhook_opened', 'page_provider_021', '{"amount": 500.00, "currency": "USD"}', '2024-03-01 11:20:00'),
('page_event_022', 'page_dispute_005', 'webhook_updated', 'page_provider_022', '{"status": "evidence_required"}', '2024-03-01 11:30:00'),
('page_event_023', 'page_dispute_005', 'evidence_added', 'page_provider_023', '{"document_type": "warranty"}', '2024-03-01 11:40:00'),
('page_event_024', 'page_dispute_005', 'evidence_added', 'page_provider_024', '{"document_type": "correspondence"}', '2024-03-01 11:50:00'),
('page_event_025', 'page_dispute_005', 'evidence_submitted', 'page_provider_025', '{"submission_id": "sub_005"}', '2024-03-01 12:00:00')
    ON CONFLICT (id, created_at) DO NOTHING;
