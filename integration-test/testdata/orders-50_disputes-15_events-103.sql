-- Seed data for test environment
-- Natural data: 50 orders, only 30% have disputes (15 disputes), 80% disputes are final, 100+ events

-- Insert 50 orders across multiple days
INSERT INTO orders (id, user_id, status, created_at, updated_at) VALUES
-- Week 1: 2024-01-15 to 2024-01-21 (35 orders)
('order_001', '550e8400-e29b-41d4-a716-446655440001', 'completed', '2024-01-15 08:00:00', '2024-01-15 08:30:00'),
('order_002', '550e8400-e29b-41d4-a716-446655440002', 'completed', '2024-01-15 10:15:00', '2024-01-15 10:45:00'),
('order_003', '550e8400-e29b-41d4-a716-446655440003', 'completed', '2024-01-15 14:30:00', '2024-01-15 15:00:00'),
('order_004', '550e8400-e29b-41d4-a716-446655440004', 'completed', '2024-01-15 16:20:00', '2024-01-15 16:50:00'),
('order_005', '550e8400-e29b-41d4-a716-446655440005', 'completed', '2024-01-15 18:45:00', '2024-01-15 19:15:00'),
('order_006', '550e8400-e29b-41d4-a716-446655440006', 'completed', '2024-01-16 09:00:00', '2024-01-16 09:30:00'),
('order_007', '550e8400-e29b-41d4-a716-446655440007', 'completed', '2024-01-16 11:30:00', '2024-01-16 12:00:00'),
('order_008', '550e8400-e29b-41d4-a716-446655440008', 'completed', '2024-01-16 13:15:00', '2024-01-16 13:45:00'),
('order_009', '550e8400-e29b-41d4-a716-446655440009', 'completed', '2024-01-16 15:45:00', '2024-01-16 16:15:00'),
('order_010', '550e8400-e29b-41d4-a716-446655440010', 'completed', '2024-01-16 17:20:00', '2024-01-16 17:50:00'),
('order_011', '550e8400-e29b-41d4-a716-446655440011', 'completed', '2024-01-17 08:30:00', '2024-01-17 09:00:00'),
('order_012', '550e8400-e29b-41d4-a716-446655440012', 'completed', '2024-01-17 12:00:00', '2024-01-17 12:30:00'),
('order_013', '550e8400-e29b-41d4-a716-446655440013', 'completed', '2024-01-17 17:30:00', '2024-01-17 18:00:00'),
('order_014', '550e8400-e29b-41d4-a716-446655440014', 'completed', '2024-01-17 19:15:00', '2024-01-17 19:45:00'),
('order_015', '550e8400-e29b-41d4-a716-446655440015', 'completed', '2024-01-18 10:15:00', '2024-01-18 10:45:00'),
('order_016', '550e8400-e29b-41d4-a716-446655440016', 'completed', '2024-01-18 14:30:00', '2024-01-18 15:00:00'),
('order_017', '550e8400-e29b-41d4-a716-446655440017', 'completed', '2024-01-18 19:00:00', '2024-01-18 19:30:00'),
('order_018', '550e8400-e29b-41d4-a716-446655440018', 'completed', '2024-01-19 09:45:00', '2024-01-19 10:15:00'),
('order_019', '550e8400-e29b-41d4-a716-446655440019', 'completed', '2024-01-19 13:20:00', '2024-01-19 13:50:00'),
('order_020', '550e8400-e29b-41d4-a716-446655440020', 'completed', '2024-01-19 16:30:00', '2024-01-19 17:00:00'),
('order_021', '550e8400-e29b-41d4-a716-446655440021', 'completed', '2024-01-19 18:45:00', '2024-01-19 19:15:00'),
('order_022', '550e8400-e29b-41d4-a716-446655440022', 'completed', '2024-01-20 08:00:00', '2024-01-20 08:30:00'),
('order_023', '550e8400-e29b-41d4-a716-446655440023', 'pending', '2024-01-20 11:30:00', '2024-01-20 11:30:00'),
('order_024', '550e8400-e29b-41d4-a716-446655440024', 'completed', '2024-01-20 15:45:00', '2024-01-20 16:15:00'),
('order_025', '550e8400-e29b-41d4-a716-446655440025', 'completed', '2024-01-20 17:30:00', '2024-01-20 18:00:00'),
('order_026', '550e8400-e29b-41d4-a716-446655440026', 'completed', '2024-01-21 10:00:00', '2024-01-21 10:30:00'),
('order_027', '550e8400-e29b-41d4-a716-446655440027', 'completed', '2024-01-21 14:15:00', '2024-01-21 14:45:00'),
('order_028', '550e8400-e29b-41d4-a716-446655440028', 'completed', '2024-01-21 18:30:00', '2024-01-21 19:00:00'),
('order_029', '550e8400-e29b-41d4-a716-446655440029', 'completed', '2024-01-21 20:15:00', '2024-01-21 20:45:00'),
('order_030', '550e8400-e29b-41d4-a716-446655440030', 'completed', '2024-01-21 21:30:00', '2024-01-21 22:00:00'),
('order_031', '550e8400-e29b-41d4-a716-446655440031', 'completed', '2024-01-21 22:45:00', '2024-01-21 23:15:00'),
('order_032', '550e8400-e29b-41d4-a716-446655440032', 'completed', '2024-01-21 23:30:00', '2024-01-22 00:00:00'),
('order_033', '550e8400-e29b-41d4-a716-446655440033', 'completed', '2024-01-22 01:15:00', '2024-01-22 01:45:00'),
('order_034', '550e8400-e29b-41d4-a716-446655440034', 'completed', '2024-01-22 03:30:00', '2024-01-22 04:00:00'),
('order_035', '550e8400-e29b-41d4-a716-446655440035', 'completed', '2024-01-22 09:15:00', '2024-01-22 09:45:00'),

-- Week 2: 2024-01-22 to 2024-01-28 (15 orders)
('order_036', '550e8400-e29b-41d4-a716-446655440036', 'completed', '2024-01-22 12:45:00', '2024-01-22 13:15:00'),
('order_037', '550e8400-e29b-41d4-a716-446655440037', 'completed', '2024-01-23 11:30:00', '2024-01-23 12:00:00'),
('order_038', '550e8400-e29b-41d4-a716-446655440038', 'completed', '2024-01-23 16:00:00', '2024-01-23 16:30:00'),
('order_039', '550e8400-e29b-41d4-a716-446655440039', 'completed', '2024-01-24 08:45:00', '2024-01-24 09:15:00'),
('order_040', '550e8400-e29b-41d4-a716-446655440040', 'pending', '2024-01-24 17:30:00', '2024-01-24 17:30:00'),
('order_041', '550e8400-e29b-41d4-a716-446655440041', 'completed', '2024-01-25 10:15:00', '2024-01-25 10:45:00'),
('order_042', '550e8400-e29b-41d4-a716-446655440042', 'completed', '2024-01-25 14:30:00', '2024-01-25 15:00:00'),
('order_043', '550e8400-e29b-41d4-a716-446655440043', 'completed', '2024-01-26 09:00:00', '2024-01-26 09:30:00'),
('order_044', '550e8400-e29b-41d4-a716-446655440044', 'completed', '2024-01-26 13:15:00', '2024-01-26 13:45:00'),
('order_045', '550e8400-e29b-41d4-a716-446655440045', 'completed', '2024-01-27 11:20:00', '2024-01-27 11:50:00'),
('order_046', '550e8400-e29b-41d4-a716-446655440046', 'completed', '2024-01-27 15:30:00', '2024-01-27 16:00:00'),
('order_047', '550e8400-e29b-41d4-a716-446655440047', 'completed', '2024-01-28 08:45:00', '2024-01-28 09:15:00'),
('order_048', '550e8400-e29b-41d4-a716-446655440048', 'completed', '2024-01-28 12:30:00', '2024-01-28 13:00:00'),
('order_049', '550e8400-e29b-41d4-a716-446655440049', 'completed', '2024-01-28 16:45:00', '2024-01-28 17:15:00'),
('order_050', '550e8400-e29b-41d4-a716-446655440050', 'pending', '2024-01-28 19:20:00', '2024-01-28 19:20:00')
    ON CONFLICT (id) DO NOTHING;

-- Insert 15 disputes (30% of 50 orders), 12 in final state (80%)
INSERT INTO disputes (id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at) VALUES
-- Final state disputes (12 total - 80%)
('dispute_001', 'order_003', 'sub_001', 'won', 'Product not received', 99.99, 'USD', '2024-01-16 09:00:00', '2024-01-23 23:59:59', '2024-01-18 14:30:00', '2024-01-25 16:45:00'),
('dispute_002', 'order_007', 'sub_002', 'lost', 'Product damaged', 149.50, 'USD', '2024-01-17 11:30:00', '2024-01-24 23:59:59', '2024-01-19 10:20:00', '2024-01-26 09:15:00'),
('dispute_003', 'order_012', 'sub_003', 'won', 'Unauthorized transaction', 75.25, 'EUR', '2024-01-18 15:20:00', '2024-01-25 23:59:59', '2024-01-20 11:00:00', '2024-01-27 13:30:00'),
('dispute_004', 'order_015', 'sub_004', 'closed', 'Service not provided', 200.00, 'GBP', '2024-01-19 08:15:00', '2024-01-26 23:59:59', '2024-01-21 16:45:00', '2024-01-28 10:20:00'),
('dispute_005', 'order_018', 'sub_005', 'won', 'Product defective', 89.99, 'USD', '2024-01-20 14:45:00', '2024-01-27 23:59:59', '2024-01-22 09:30:00', '2024-01-29 11:15:00'),
('dispute_006', 'order_024', 'sub_006', 'lost', 'Duplicate charge', 45.50, 'USD', '2024-01-21 10:30:00', '2024-01-28 23:59:59', '2024-01-23 15:15:00', '2024-01-30 14:20:00'),
('dispute_007', 'order_026', 'sub_007', 'won', 'Product not as described', 125.75, 'EUR', '2024-01-22 16:00:00', '2024-01-29 23:59:59', '2024-01-24 12:30:00', '2024-01-31 17:45:00'),
('dispute_008', 'order_031', 'sub_008', 'canceled', 'Billing error', 67.80, 'USD', '2024-01-23 09:45:00', '2024-01-30 23:59:59', NULL, '2024-01-25 08:30:00'),
('dispute_009', 'order_035', 'sub_009', 'won', 'Service cancelled', 180.00, 'GBP', '2024-01-24 13:30:00', '2024-01-31 23:59:59', '2024-01-26 11:20:00', '2024-02-02 15:45:00'),
('dispute_010', 'order_037', 'sub_010', 'lost', 'Wrong amount charged', 55.25, 'USD', '2024-01-25 17:15:00', '2024-02-01 23:59:59', '2024-01-27 14:45:00', '2024-02-03 10:30:00'),
('dispute_011', 'order_041', 'sub_011', 'won', 'Product quality issue', 95.40, 'EUR', '2024-01-26 08:30:00', '2024-02-02 23:59:59', '2024-01-28 16:20:00', '2024-02-04 12:15:00'),
('dispute_012', 'order_045', 'sub_012', 'closed', 'Refund not processed', 120.00, 'USD', '2024-01-27 12:45:00', '2024-02-03 23:59:59', '2024-01-29 09:30:00', '2024-02-05 11:45:00'),

-- Active disputes (3 total - 20%)
('dispute_013', 'order_047', 'sub_013', 'open', 'Delivery never received', 78.90, 'USD', '2024-01-28 11:20:00', '2024-02-04 23:59:59', NULL, NULL),
('dispute_014', 'order_049', 'sub_014', 'submitted', 'Incorrect item shipped', 165.30, 'GBP', '2024-01-29 15:30:00', '2024-02-05 23:59:59', '2024-01-31 10:45:00', NULL),
('dispute_015', 'order_050', 'sub_015', 'under_review', 'Late delivery penalty', 35.00, 'GBP', '2024-01-30 09:30:00', '2024-02-06 23:59:59', '2024-02-01 14:20:00', NULL)
    ON CONFLICT (id) DO NOTHING;

-- Insert 100+ dispute events using proper domain event kinds
INSERT INTO dispute_events (id, dispute_id, kind, provider_event_id, data, created_at) VALUES
-- Events for dispute_001 (won dispute - complete lifecycle)
('event_001', 'dispute_001', 'webhook_opened', 'chb_opened_001', '{"amount": 99.99, "currency": "USD", "reason": "Product not received", "status": "opened"}', '2024-01-16 09:00:00'),
('event_002', 'dispute_001', 'webhook_updated', 'chb_updated_001_1', '{"evidence_due_at": "2024-01-23T23:59:59Z", "required_documents": ["receipt", "shipping_proof"]}', '2024-01-16 09:05:00'),
('event_003', 'dispute_001', 'evidence_added', 'evidence_001_1', '{"document_type": "receipt", "file_id": "file_001"}', '2024-01-18 10:30:00'),
('event_004', 'dispute_001', 'evidence_added', 'evidence_001_2', '{"document_type": "shipping_proof", "file_id": "file_002"}', '2024-01-18 14:20:00'),
('event_005', 'dispute_001', 'evidence_submitted', 'submit_001', '{"submission_id": "sub_001", "documents_count": 2}', '2024-01-18 14:30:00'),
('event_006', 'dispute_001', 'webhook_updated', 'chb_updated_001_2', '{"status": "under_review", "review_started_at": "2024-01-19T08:00:00Z"}', '2024-01-19 08:00:00'),
('event_007', 'dispute_001', 'provider_decision', 'decision_001', '{"outcome": "won", "resolution": "Merchant provided sufficient evidence", "amount_recovered": 99.99}', '2024-01-25 16:45:00'),

-- Events for dispute_002 (lost dispute - complete lifecycle)
('event_008', 'dispute_002', 'webhook_opened', 'chb_opened_002', '{"amount": 149.50, "currency": "USD", "reason": "Product damaged", "status": "opened"}', '2024-01-17 11:30:00'),
('event_009', 'dispute_002', 'webhook_updated', 'chb_updated_002_1', '{"evidence_due_at": "2024-01-24T23:59:59Z", "required_documents": ["photos", "receipt"]}', '2024-01-17 11:35:00'),
('event_010', 'dispute_002', 'evidence_added', 'evidence_002_1', '{"document_type": "photos", "file_id": "file_003"}', '2024-01-19 09:15:00'),
('event_011', 'dispute_002', 'evidence_added', 'evidence_002_2', '{"document_type": "receipt", "file_id": "file_004"}', '2024-01-19 10:00:00'),
('event_012', 'dispute_002', 'evidence_submitted', 'submit_002', '{"submission_id": "sub_002", "documents_count": 2}', '2024-01-19 10:20:00'),
('event_013', 'dispute_002', 'webhook_updated', 'chb_updated_002_2', '{"status": "under_review"}', '2024-01-20 12:00:00'),
('event_014', 'dispute_002', 'provider_decision', 'decision_002', '{"outcome": "lost", "resolution": "Evidence insufficient - product damage confirmed", "amount_charged": 149.50}', '2024-01-26 09:15:00'),

-- Events for dispute_003 (won dispute - complete lifecycle)
('event_015', 'dispute_003', 'webhook_opened', 'chb_opened_003', '{"amount": 75.25, "currency": "EUR", "reason": "Unauthorized transaction", "status": "opened"}', '2024-01-18 15:20:00'),
('event_016', 'dispute_003', 'webhook_updated', 'chb_updated_003_1', '{"evidence_due_at": "2024-01-25T23:59:59Z", "required_documents": ["bank_statement", "authorization_proof"]}', '2024-01-18 15:25:00'),
('event_017', 'dispute_003', 'evidence_added', 'evidence_003_1', '{"document_type": "bank_statement", "file_id": "file_005"}', '2024-01-20 10:30:00'),
('event_018', 'dispute_003', 'evidence_added', 'evidence_003_2', '{"document_type": "authorization_proof", "file_id": "file_006"}', '2024-01-20 10:45:00'),
('event_019', 'dispute_003', 'evidence_submitted', 'submit_003', '{"submission_id": "sub_003", "documents_count": 2}', '2024-01-20 11:00:00'),
('event_020', 'dispute_003', 'webhook_updated', 'chb_updated_003_2', '{"status": "under_review"}', '2024-01-21 09:00:00'),
('event_021', 'dispute_003', 'provider_decision', 'decision_003', '{"outcome": "won", "resolution": "Transaction properly authorized", "amount_recovered": 75.25}', '2024-01-27 13:30:00'),

-- Events for dispute_004 (closed dispute - minimal lifecycle)
('event_022', 'dispute_004', 'webhook_opened', 'chb_opened_004', '{"amount": 200.00, "currency": "GBP", "reason": "Service not provided", "status": "opened"}', '2024-01-19 08:15:00'),
('event_023', 'dispute_004', 'webhook_updated', 'chb_updated_004_1', '{"evidence_due_at": "2024-01-26T23:59:59Z", "required_documents": ["service_records", "contract"]}', '2024-01-19 08:20:00'),
('event_024', 'dispute_004', 'evidence_added', 'evidence_004_1', '{"document_type": "service_records", "file_id": "file_007"}', '2024-01-21 16:00:00'),
('event_025', 'dispute_004', 'evidence_added', 'evidence_004_2', '{"document_type": "contract", "file_id": "file_008"}', '2024-01-21 16:30:00'),
('event_026', 'dispute_004', 'evidence_submitted', 'submit_004', '{"submission_id": "sub_004", "documents_count": 2}', '2024-01-21 16:45:00'),
('event_027', 'dispute_004', 'provider_decision', 'decision_004', '{"outcome": "closed", "resolution": "Case closed without specific outcome"}', '2024-01-28 10:20:00'),

-- Events for dispute_005 (won dispute - complete lifecycle)
('event_028', 'dispute_005', 'webhook_opened', 'chb_opened_005', '{"amount": 89.99, "currency": "USD", "reason": "Product defective", "status": "opened"}', '2024-01-20 14:45:00'),
('event_029', 'dispute_005', 'webhook_updated', 'chb_updated_005_1', '{"evidence_due_at": "2024-01-27T23:59:59Z", "required_documents": ["warranty", "photos", "repair_records"]}', '2024-01-20 14:50:00'),
('event_030', 'dispute_005', 'evidence_added', 'evidence_005_1', '{"document_type": "warranty", "file_id": "file_009"}', '2024-01-22 08:15:00'),
('event_031', 'dispute_005', 'evidence_added', 'evidence_005_2', '{"document_type": "photos", "file_id": "file_010"}', '2024-01-22 08:45:00'),
('event_032', 'dispute_005', 'evidence_added', 'evidence_005_3', '{"document_type": "repair_records", "file_id": "file_011"}', '2024-01-22 09:15:00'),
('event_033', 'dispute_005', 'evidence_submitted', 'submit_005', '{"submission_id": "sub_005", "documents_count": 3}', '2024-01-22 09:30:00'),
('event_034', 'dispute_005', 'webhook_updated', 'chb_updated_005_2', '{"status": "under_review"}', '2024-01-23 10:00:00'),
('event_035', 'dispute_005', 'provider_decision', 'decision_005', '{"outcome": "won", "resolution": "Product defect not covered by warranty", "amount_recovered": 89.99}', '2024-01-29 11:15:00'),

-- Events for dispute_006 (lost dispute - complete lifecycle)
('event_036', 'dispute_006', 'webhook_opened', 'chb_opened_006', '{"amount": 45.50, "currency": "USD", "reason": "Duplicate charge", "status": "opened"}', '2024-01-21 10:30:00'),
('event_037', 'dispute_006', 'webhook_updated', 'chb_updated_006_1', '{"evidence_due_at": "2024-01-28T23:59:59Z", "required_documents": ["transaction_logs", "bank_statement"]}', '2024-01-21 10:35:00'),
('event_038', 'dispute_006', 'evidence_added', 'evidence_006_1', '{"document_type": "transaction_logs", "file_id": "file_012"}', '2024-01-23 14:30:00'),
('event_039', 'dispute_006', 'evidence_added', 'evidence_006_2', '{"document_type": "bank_statement", "file_id": "file_013"}', '2024-01-23 15:00:00'),
('event_040', 'dispute_006', 'evidence_submitted', 'submit_006', '{"submission_id": "sub_006", "documents_count": 2}', '2024-01-23 15:15:00'),
('event_041', 'dispute_006', 'webhook_updated', 'chb_updated_006_2', '{"status": "under_review"}', '2024-01-24 11:00:00'),
('event_042', 'dispute_006', 'provider_decision', 'decision_006', '{"outcome": "lost", "resolution": "Duplicate charge confirmed by bank records", "amount_charged": 45.50}', '2024-01-30 14:20:00'),

-- Events for dispute_007 (won dispute - complete lifecycle)
('event_043', 'dispute_007', 'webhook_opened', 'chb_opened_007', '{"amount": 125.75, "currency": "EUR", "reason": "Product not as described", "status": "opened"}', '2024-01-22 16:00:00'),
('event_044', 'dispute_007', 'webhook_updated', 'chb_updated_007_1', '{"evidence_due_at": "2024-01-29T23:59:59Z", "required_documents": ["product_description", "photos", "specifications"]}', '2024-01-22 16:05:00'),
('event_045', 'dispute_007', 'evidence_added', 'evidence_007_1', '{"document_type": "product_description", "file_id": "file_014"}', '2024-01-24 11:30:00'),
('event_046', 'dispute_007', 'evidence_added', 'evidence_007_2', '{"document_type": "photos", "file_id": "file_015"}', '2024-01-24 12:00:00'),
('event_047', 'dispute_007', 'evidence_added', 'evidence_007_3', '{"document_type": "specifications", "file_id": "file_016"}', '2024-01-24 12:15:00'),
('event_048', 'dispute_007', 'evidence_submitted', 'submit_007', '{"submission_id": "sub_007", "documents_count": 3}', '2024-01-24 12:30:00'),
('event_049', 'dispute_007', 'webhook_updated', 'chb_updated_007_2', '{"status": "under_review"}', '2024-01-25 09:00:00'),
('event_050', 'dispute_007', 'provider_decision', 'decision_007', '{"outcome": "won", "resolution": "Product matched description and specifications", "amount_recovered": 125.75}', '2024-01-31 17:45:00'),

-- Events for dispute_008 (canceled dispute - short lifecycle)
('event_051', 'dispute_008', 'webhook_opened', 'chb_opened_008', '{"amount": 67.80, "currency": "USD", "reason": "Billing error", "status": "opened"}', '2024-01-23 09:45:00'),
('event_052', 'dispute_008', 'webhook_updated', 'chb_updated_008_1', '{"evidence_due_at": "2024-01-30T23:59:59Z", "required_documents": ["invoice", "payment_records"]}', '2024-01-23 09:50:00'),
('event_053', 'dispute_008', 'provider_decision', 'decision_008', '{"outcome": "canceled", "resolution": "Dispute canceled by cardholder"}', '2024-01-25 08:30:00'),

-- Events for dispute_009 (won dispute - complete lifecycle)
('event_054', 'dispute_009', 'webhook_opened', 'chb_opened_009', '{"amount": 180.00, "currency": "GBP", "reason": "Service cancelled", "status": "opened"}', '2024-01-24 13:30:00'),
('event_055', 'dispute_009', 'webhook_updated', 'chb_updated_009_1', '{"evidence_due_at": "2024-01-31T23:59:59Z", "required_documents": ["service_agreement", "cancellation_policy", "usage_logs"]}', '2024-01-24 13:35:00'),
('event_056', 'dispute_009', 'evidence_added', 'evidence_009_1', '{"document_type": "service_agreement", "file_id": "file_017"}', '2024-01-26 10:00:00'),
('event_057', 'dispute_009', 'evidence_added', 'evidence_009_2', '{"document_type": "cancellation_policy", "file_id": "file_018"}', '2024-01-26 10:30:00'),
('event_058', 'dispute_009', 'evidence_added', 'evidence_009_3', '{"document_type": "usage_logs", "file_id": "file_019"}', '2024-01-26 11:00:00'),
('event_059', 'dispute_009', 'evidence_submitted', 'submit_009', '{"submission_id": "sub_009", "documents_count": 3}', '2024-01-26 11:20:00'),
('event_060', 'dispute_009', 'webhook_updated', 'chb_updated_009_2', '{"status": "under_review"}', '2024-01-27 14:00:00'),
('event_061', 'dispute_009', 'provider_decision', 'decision_009', '{"outcome": "won", "resolution": "Service was provided according to agreement", "amount_recovered": 180.00}', '2024-02-02 15:45:00'),

-- Events for dispute_010 (lost dispute - complete lifecycle)
('event_062', 'dispute_010', 'webhook_opened', 'chb_opened_010', '{"amount": 55.25, "currency": "USD", "reason": "Wrong amount charged", "status": "opened"}', '2024-01-25 17:15:00'),
('event_063', 'dispute_010', 'webhook_updated', 'chb_updated_010_1', '{"evidence_due_at": "2024-02-01T23:59:59Z", "required_documents": ["pricing_agreement", "invoice", "payment_authorization"]}', '2024-01-25 17:20:00'),
('event_064', 'dispute_010', 'evidence_added', 'evidence_010_1', '{"document_type": "pricing_agreement", "file_id": "file_020"}', '2024-01-27 13:30:00'),
('event_065', 'dispute_010', 'evidence_added', 'evidence_010_2', '{"document_type": "invoice", "file_id": "file_021"}', '2024-01-27 14:00:00'),
('event_066', 'dispute_010', 'evidence_added', 'evidence_010_3', '{"document_type": "payment_authorization", "file_id": "file_022"}', '2024-01-27 14:30:00'),
('event_067', 'dispute_010', 'evidence_submitted', 'submit_010', '{"submission_id": "sub_010", "documents_count": 3}', '2024-01-27 14:45:00'),
('event_068', 'dispute_010', 'webhook_updated', 'chb_updated_010_2', '{"status": "under_review"}', '2024-01-28 16:00:00'),
('event_069', 'dispute_010', 'provider_decision', 'decision_010', '{"outcome": "lost", "resolution": "Amount charged was incorrect according to agreement", "amount_charged": 55.25}', '2024-02-03 10:30:00'),

-- Events for dispute_011 (won dispute - complete lifecycle)
('event_070', 'dispute_011', 'webhook_opened', 'chb_opened_011', '{"amount": 95.40, "currency": "EUR", "reason": "Product quality issue", "status": "opened"}', '2024-01-26 08:30:00'),
('event_071', 'dispute_011', 'webhook_updated', 'chb_updated_011_1', '{"evidence_due_at": "2024-02-02T23:59:59Z", "required_documents": ["quality_standards", "testing_reports", "photos"]}', '2024-01-26 08:35:00'),
('event_072', 'dispute_011', 'evidence_added', 'evidence_011_1', '{"document_type": "quality_standards", "file_id": "file_023"}', '2024-01-28 15:30:00'),
('event_073', 'dispute_011', 'evidence_added', 'evidence_011_2', '{"document_type": "testing_reports", "file_id": "file_024"}', '2024-01-28 16:00:00'),
('event_074', 'dispute_011', 'evidence_added', 'evidence_011_3', '{"document_type": "photos", "file_id": "file_025"}', '2024-01-28 16:15:00'),
('event_075', 'dispute_011', 'evidence_submitted', 'submit_011', '{"submission_id": "sub_011", "documents_count": 3}', '2024-01-28 16:20:00'),
('event_076', 'dispute_011', 'webhook_updated', 'chb_updated_011_2', '{"status": "under_review"}', '2024-01-29 10:00:00'),
('event_077', 'dispute_011', 'provider_decision', 'decision_011', '{"outcome": "won", "resolution": "Product met all quality standards", "amount_recovered": 95.40}', '2024-02-04 12:15:00'),

-- Events for dispute_012 (closed dispute - complete lifecycle)
('event_078', 'dispute_012', 'webhook_opened', 'chb_opened_012', '{"amount": 120.00, "currency": "USD", "reason": "Refund not processed", "status": "opened"}', '2024-01-27 12:45:00'),
('event_079', 'dispute_012', 'webhook_updated', 'chb_updated_012_1', '{"evidence_due_at": "2024-02-03T23:59:59Z", "required_documents": ["refund_policy", "processing_records", "bank_records"]}', '2024-01-27 12:50:00'),
('event_080', 'dispute_012', 'evidence_added', 'evidence_012_1', '{"document_type": "refund_policy", "file_id": "file_026"}', '2024-01-29 08:45:00'),
('event_081', 'dispute_012', 'evidence_added', 'evidence_012_2', '{"document_type": "processing_records", "file_id": "file_027"}', '2024-01-29 09:15:00'),
('event_082', 'dispute_012', 'evidence_added', 'evidence_012_3', '{"document_type": "bank_records", "file_id": "file_028"}', '2024-01-29 09:25:00'),
('event_083', 'dispute_012', 'evidence_submitted', 'submit_012', '{"submission_id": "sub_012", "documents_count": 3}', '2024-01-29 09:30:00'),
('event_084', 'dispute_012', 'webhook_updated', 'chb_updated_012_2', '{"status": "under_review"}', '2024-01-30 11:00:00'),
('event_085', 'dispute_012', 'provider_decision', 'decision_012', '{"outcome": "closed", "resolution": "Case resolved through direct communication"}', '2024-02-05 11:45:00'),

-- Events for dispute_013 (open dispute - ongoing)
('event_086', 'dispute_013', 'webhook_opened', 'chb_opened_013', '{"amount": 78.90, "currency": "USD", "reason": "Delivery never received", "status": "opened"}', '2024-01-28 11:20:00'),
('event_087', 'dispute_013', 'webhook_updated', 'chb_updated_013_1', '{"evidence_due_at": "2024-02-04T23:59:59Z", "required_documents": ["shipping_records", "tracking_info", "delivery_confirmation"]}', '2024-01-28 11:25:00'),
('event_088', 'dispute_013', 'evidence_added', 'evidence_013_1', '{"document_type": "shipping_records", "file_id": "file_029"}', '2024-01-30 14:30:00'),
('event_089', 'dispute_013', 'evidence_added', 'evidence_013_2', '{"document_type": "tracking_info", "file_id": "file_030"}', '2024-01-30 15:00:00'),

-- Events for dispute_014 (submitted dispute - awaiting decision)
('event_090', 'dispute_014', 'webhook_opened', 'chb_opened_014', '{"amount": 165.30, "currency": "GBP", "reason": "Incorrect item shipped", "status": "opened"}', '2024-01-29 15:30:00'),
('event_091', 'dispute_014', 'webhook_updated', 'chb_updated_014_1', '{"evidence_due_at": "2024-02-05T23:59:59Z", "required_documents": ["order_details", "shipping_manifest", "photos"]}', '2024-01-29 15:35:00'),
('event_092', 'dispute_014', 'evidence_added', 'evidence_014_1', '{"document_type": "order_details", "file_id": "file_031"}', '2024-01-31 09:30:00'),
('event_093', 'dispute_014', 'evidence_added', 'evidence_014_2', '{"document_type": "shipping_manifest", "file_id": "file_032"}', '2024-01-31 10:15:00'),
('event_094', 'dispute_014', 'evidence_added', 'evidence_014_3', '{"document_type": "photos", "file_id": "file_033"}', '2024-01-31 10:30:00'),
('event_095', 'dispute_014', 'evidence_submitted', 'submit_014', '{"submission_id": "sub_014", "documents_count": 3}', '2024-01-31 10:45:00'),
('event_096', 'dispute_014', 'webhook_updated', 'chb_updated_014_2', '{"status": "under_review"}', '2024-02-01 12:00:00'),

-- Events for dispute_015 (under_review dispute - awaiting decision)
('event_097', 'dispute_015', 'webhook_opened', 'chb_opened_015', '{"amount": 35.00, "currency": "GBP", "reason": "Late delivery penalty", "status": "opened"}', '2024-01-30 09:30:00'),
('event_098', 'dispute_015', 'webhook_updated', 'chb_updated_015_1', '{"evidence_due_at": "2024-02-06T23:59:59Z", "required_documents": ["delivery_policy", "shipping_records", "communication_logs"]}', '2024-01-30 09:35:00'),
('event_099', 'dispute_015', 'evidence_added', 'evidence_015_1', '{"document_type": "delivery_policy", "file_id": "file_034"}', '2024-02-01 13:45:00'),
('event_100', 'dispute_015', 'evidence_added', 'evidence_015_2', '{"document_type": "shipping_records", "file_id": "file_035"}', '2024-02-01 14:00:00'),
('event_101', 'dispute_015', 'evidence_added', 'evidence_015_3', '{"document_type": "communication_logs", "file_id": "file_036"}', '2024-02-01 14:15:00'),
('event_102', 'dispute_015', 'evidence_submitted', 'submit_015', '{"submission_id": "sub_015", "documents_count": 3}', '2024-02-01 14:20:00'),
('event_103', 'dispute_015', 'webhook_updated', 'chb_updated_015_2', '{"status": "under_review"}', '2024-02-02 10:00:00')
    ON CONFLICT (id) DO NOTHING;