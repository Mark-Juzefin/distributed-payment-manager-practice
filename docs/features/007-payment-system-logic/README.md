# Feature: Payment System Logic

**Status:** In Progress

## Overview

Build a realistic payment processing pipeline inspired by Solidgate's auth & settle model. **Focus: Silvergate PSP and PostgreSQL transaction safety.**

**Architecture direction:**
```
Banks (mocked) ← Silvergate (PSP) ← Paymanager (merchant) ← Ingest (webhook gateway)
```

- **Silvergate** (`services/silvergate`) — payment gateway/PSP. The primary focus of this feature. Auth/capture/void/refund, mocked bank, webhook callbacks. Transaction safety practice happens here.
- **Paymanager** (`services/paymanager`) — temporary integration scaffolding to test Silvergate end-to-end. Current payment/order domains overlap and proxy Silvergate building blocks directly. This is intentional — Paymanager will be redesigned in Feature 009 after Silvergate is solid.
- **Ingest** — stays as webhook ingestion gateway for highload simulation.

**Bottom-up approach:** Get Silvergate right first (domain model, transaction safety, concurrency), then build proper business logic on top in Paymanager (Feature 009).

## Implementation Plan

- **Subtask 1 plan:** TBD — rename api → paymanager
- **Subtask 2 plan:** [plan-subtask-2.md](plan-subtask-2.md) — Silvergate service: auth & capture
- **Subtask 3 plan:** [plan-subtask-3.md](plan-subtask-3.md) — integrate Silvergate into Paymanager
- **Subtask 4 plan:** [plan-subtask-4.md](plan-subtask-4.md) — void with capture_delay
- **Subtask 5 plan:** [plan-subtask-5.md](plan-subtask-5.md) — refund in Silvergate
- **Subtask 6 plan:** TBD — refund integration in Paymanager

## Tasks

### Phase 1: Silvergate PSP — core flow (done)

- [x] **Subtask 1:** Rename `services/api` → `services/paymanager`
- [x] **Subtask 2:** Silvergate service — auth & capture flow, mocked bank, webhook callbacks
- [x] **Subtask 3:** Paymanager temp integration — payment entities, auth/capture to Silvergate, webhook handling
- [x] **Subtask 4:** Void — capture_delay + void endpoint (Silvergate + Paymanager)
- [x] **Subtask 5:** Refund in Silvergate
- [x] **Subtask 6:** Refund integration in Paymanager (temporary scaffolding)

### Phase 2: Transaction safety in Silvergate (active)

- [x] **Subtask 7:** Concurrent refund race condition — RepeatableRead transaction

  **Bug:** 3 concurrent refunds on a $50 payment all pass validation (each reads `refunded_amount = 0`).
  **Fix:** RepeatableRead wraps read + validate + reserve. PostgreSQL's first-writer-wins aborts concurrent UPDATEs.
  **Integration test:** `services/silvergate/domain/transaction/service_integration_test.go`

- [x] **Subtask 8:** Capture — RepeatableRead transaction (same pattern as Subtask 7)
- [x] **Subtask 9:** settleAsync — optimistic locking via `WHERE status = 'capture_pending'` + RowsAffected
- [ ] **Subtask 10:** Void vs Capture race — SELECT FOR UPDATE to protect concurrent state transitions
- [ ] **Subtask 11:** CHECK constraint `refunded_amount >= 0` on transactions table
- [ ] **Subtask 12:** ReleaseRefundAmount — atomic amount + status update in one transaction
- [ ] **Subtask 13:** refundAsync — retry on failed ReleaseRefundAmount (compensating transaction)

## Architecture Notes

**Auth & Settle flow (Solidgate model):**
1. Merchant creates payment → sends auth request to PSP
2. PSP authorizes with bank (hold funds) → responds sync (authorized/declined)
3. Merchant sends capture request → PSP settles with bank
4. PSP sends webhook when capture is settled

**Silvergate internal design:**
- Bank integration = interface with mock implementation (approve/decline with configurable probability)
- Own PostgreSQL schema for transactions
- Webhook sender to notify merchant about async results

**Paymanager (temporary, will be redesigned in Feature 009):**
- `domain/payment/` — proxies Silvergate building blocks (auth, capture, void, refund) as API endpoints
- `domain/order/` — legacy webhook-based order tracking, has duplicate capture logic
- `domain/dispute/` — legacy chargeback handling, tied to old order model
- These overlap and will be unified in Feature 009

## Expansion Ideas: PostgreSQL Transactions Deep Dive

Silvergate is an ideal playground for studying PostgreSQL transactions. Currently operations are simple (single row writes). Adding financial balances makes every operation a multi-row atomic change — the core of transaction complexity.

### 1. Balances

Add `balances` table (card balance, merchant balance). Every payment operation becomes two atomic writes:

```
Auth:    card.available -= amount, card.held += amount
Capture: card.held -= amount, merchant.available += amount
Void:    card.held -= amount, card.available += amount
Refund:  merchant.available -= amount, card.available += amount
```

This introduces:
- **Atomicity** — if balance update fails, transaction record must not change
- **Isolation** — two concurrent auths on the same card must not overdraft
- **Lost update problem** — `SELECT balance → check → UPDATE` without proper locking
- **SELECT FOR UPDATE** vs optimistic locking

### 2. Double-entry ledger

Every money movement = two entries (debit + credit). Sum of all entries always = 0. Fintech standard. Great for:
- Transactional consistency
- CHECK constraints that sum = 0
- Deadlocks on concurrent writes to the same account

### 3. Concurrent payments on one card

Two auths at the same time → both see sufficient balance → both deduct → overdraft. Classic race condition for studying isolation levels:
- **Read Committed** — sees committed data, but phantom reads
- **Repeatable Read** — serialization errors on conflicts
- **Serializable** — full isolation, maximum retries

### 4. Idempotency with concurrent requests

Two identical capture requests arrive simultaneously. INSERT ON CONFLICT vs SELECT FOR UPDATE + check.

### Suggested implementation order

1. Balances (card + merchant accounts) — the field for transaction experiments
2. Concurrent load test — k6/loadtest hitting auth/capture in parallel
3. Isolation level experiments — demonstrate race conditions and how to fix them
4. Deadlock scenarios — provoke and analyze

Could become a separate roadmap feature after Payment System Logic.

## Notes
- Created: 2026-04-09
- Inspired by: https://docs.solidgate.com/payments/payments-overview/#auth--settle
- Existing Wiremock stubs for Silvergate will be replaced by the real service
