# Feature: Payment System Logic

**Status:** Done

## Overview

Build a realistic payment processing pipeline inspired by Solidgate's auth & settle model. **Focus: Silvergate PSP and PostgreSQL transaction safety.**

```
Banks (mocked) ← Silvergate (PSP) ← Paymanager (merchant) ← Ingest (webhook gateway)
```

- **Silvergate** (`services/silvergate`) — payment gateway/PSP. Auth/capture/void/refund, mocked bank, webhook callbacks.
- **Paymanager** (`services/paymanager`) — temporary scaffolding, will be redesigned in Feature 008.

## Tasks

### Phase 1: Core flow (done)

- [x] **Subtask 1:** Rename `services/api` → `services/paymanager`
- [x] **Subtask 2:** Silvergate service — auth & capture flow, mocked bank, webhook callbacks
- [x] **Subtask 3:** Paymanager integration — payment entities, auth/capture to Silvergate, webhook handling
- [x] **Subtask 4:** Void — capture_delay + void endpoint
- [x] **Subtask 5:** Refund in Silvergate
- [x] **Subtask 6:** Refund integration in Paymanager

### Phase 2: Transaction safety (done)

- [x] **Subtask 7:** Concurrent refund — RepeatableRead transaction (first-writer-wins)
- [x] **Subtask 8:** Concurrent capture — RepeatableRead transaction
- [x] **Subtask 9:** settleAsync — optimistic locking (`WHERE status = 'capture_pending'` + RowsAffected)
- [x] **Subtask 10:** Void vs Capture — pessimistic locking (SELECT FOR UPDATE)
- [x] **Subtask 11:** CHECK constraint `refunded_amount >= 0`
- [x] **Subtask 12:** ReleaseRefundAmount — atomic amount + status in one SQL
- [x] **Subtask 13:** ReleaseRefundAmount — retry on failure

**Concepts practiced:**

| Concept | Where | How |
|---------|-------|-----|
| RepeatableRead + first-writer-wins | Refund, Capture | `InTransaction(ctx, pgx.RepeatableRead, ...)` |
| Optimistic locking | settleAsync | `CompareAndUpdateStatus(id, expected, next)` |
| Pessimistic locking | Void | `GetByIDForUpdate` inside ReadCommitted tx |
| CHECK constraint | transactions table | `CHECK (refunded_amount >= 0)` |
| Atomicity | ReleaseRefundAmount | Single SQL with CASE for status |
| Compensating retry | refundAsync | 3 retries with backoff |

**When to use which:**
- **RepeatableRead** — fast read→validate→write, no external calls between read and write
- **SELECT FOR UPDATE** — external call (bank, HTTP) between read and write
- **Optimistic locking (WHERE)** — async operations verifying preconditions at write time
- **CHECK constraint** — invariant that must hold regardless of application bugs

**Integration tests:** `services/silvergate/domain/transaction/service_integration_test.go`

> Multi-row transactions deep dive (balances, double-entry ledger, concurrent auths, load tests) continues in [Feature 010: Silvergate Transactions Deep Dive](../010-silvergate-transactions-deep-dive/).

## Notes
- Created: 2026-04-09
- Phase 1-2 completed: 2026-04-17
- Inspired by: https://docs.solidgate.com/payments/payments-overview/#auth--settle
