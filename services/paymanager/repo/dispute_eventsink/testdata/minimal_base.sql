-- Minimal test data for integration tests
-- Contains 5 orders, 3 disputes, and 12 events - just enough for basic CRUD testing

-- Insert minimal set of orders
INSERT INTO orders (id, user_id, status, on_hold, hold_reason, created_at, updated_at) VALUES
('order_001', '550e8400-e29b-41d4-a716-446655440001', 'completed', false, NULL, '2024-01-15 10:00:00', '2024-01-15 10:30:00'),
('order_002', '550e8400-e29b-41d4-a716-446655440002', 'completed', false, NULL, '2024-01-15 14:00:00', '2024-01-15 14:30:00'),
('order_003', '550e8400-e29b-41d4-a716-446655440003', 'completed', false, NULL, '2024-01-16 09:00:00', '2024-01-16 09:30:00'),
('order_004', '550e8400-e29b-41d4-a716-446655440004', 'completed', false, NULL, '2024-01-16 15:00:00', '2024-01-16 15:30:00'),
('order_005', '550e8400-e29b-41d4-a716-446655440005', 'pending', true, 'manual_review', '2024-01-17 11:00:00', '2024-01-17 11:00:00')
    ON CONFLICT (id) DO NOTHING;

-- Insert 3 disputes with different states
INSERT INTO disputes (id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at) VALUES
-- Complete dispute lifecycle (won)
('dispute_001', 'order_001', 'sub_001', 'won', 'Product not received', 99.99, 'USD', '2024-01-16 09:00:00', '2024-01-23 23:59:59', '2024-01-18 14:30:00', '2024-01-25 16:45:00'),
-- Complete dispute lifecycle (lost) 
('dispute_002', 'order_002', 'sub_002', 'lost', 'Product damaged', 149.50, 'USD', '2024-01-17 11:30:00', '2024-01-24 23:59:59', '2024-01-19 10:20:00', '2024-01-26 09:15:00'),
-- Open dispute (ongoing)
('dispute_003', 'order_003', 'sub_003', 'open', 'Service not provided', 200.00, 'EUR', '2024-01-18 15:20:00', '2024-01-25 23:59:59', NULL, NULL)
    ON CONFLICT (id) DO NOTHING;

-- Insert minimal set of dispute events covering all event kinds
INSERT INTO dispute_events (id, dispute_id, kind, provider_event_id, data, created_at) VALUES
-- Events for dispute_001 (complete lifecycle)
('event_001', 'dispute_001', 'webhook_opened', 'chb_opened_001', '{"amount": 99.99, "currency": "USD", "reason": "Product not received"}', '2024-01-16 09:00:00'),
('event_002', 'dispute_001', 'webhook_updated', 'chb_updated_001', '{"evidence_due_at": "2024-01-23T23:59:59Z"}', '2024-01-16 09:05:00'),
('event_003', 'dispute_001', 'evidence_added', 'evidence_001', '{"document_type": "receipt", "file_id": "file_001"}', '2024-01-18 10:30:00'),
('event_004', 'dispute_001', 'evidence_submitted', 'submit_001', '{"submission_id": "sub_001", "documents_count": 1}', '2024-01-18 14:30:00'),
('event_005', 'dispute_001', 'provider_decision', 'decision_001', '{"outcome": "won", "amount_recovered": 99.99}', '2024-01-25 16:45:00'),

-- Events for dispute_002 (complete lifecycle)
('event_006', 'dispute_002', 'webhook_opened', 'chb_opened_002', '{"amount": 149.50, "currency": "USD", "reason": "Product damaged"}', '2024-01-17 11:30:00'),
('event_007', 'dispute_002', 'webhook_updated', 'chb_updated_002', '{"evidence_due_at": "2024-01-24T23:59:59Z"}', '2024-01-17 11:35:00'),
('event_008', 'dispute_002', 'evidence_added', 'evidence_002', '{"document_type": "photos", "file_id": "file_002"}', '2024-01-19 09:15:00'),
('event_009', 'dispute_002', 'evidence_submitted', 'submit_002', '{"submission_id": "sub_002", "documents_count": 1}', '2024-01-19 10:20:00'),
('event_010', 'dispute_002', 'provider_decision', 'decision_002', '{"outcome": "lost", "amount_charged": 149.50}', '2024-01-26 09:15:00'),

-- Events for dispute_003 (ongoing - partial lifecycle)
('event_011', 'dispute_003', 'webhook_opened', 'chb_opened_003', '{"amount": 200.00, "currency": "EUR", "reason": "Service not provided"}', '2024-01-18 15:20:00'),
('event_012', 'dispute_003', 'webhook_updated', 'chb_updated_003', '{"evidence_due_at": "2024-01-25T23:59:59Z"}', '2024-01-18 15:25:00')
    ON CONFLICT (id, created_at) DO NOTHING;
