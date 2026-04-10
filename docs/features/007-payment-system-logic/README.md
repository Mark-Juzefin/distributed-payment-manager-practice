# Feature: Payment System Logic

**Status:** In Progress

## Overview

Build a realistic payment processing pipeline inspired by Solidgate's auth & settle model.

**Architecture direction:**
```
Banks (mocked) ← Silvergate (PSP) ← Paymanager (merchant) ← Ingest (webhook gateway)
```

- **Paymanager** (`services/paymanager`, renamed from `services/api`) — merchant side. Initiates payments, handles webhooks from PSP, manages orders/disputes.
- **Silvergate** (`services/silvergate`, new) — payment gateway/PSP. Accepts auth/capture/void/refund from merchants, communicates with bank (mocked internally), sends webhooks back.
- **Ingest** — stays as webhook ingestion gateway for highload simulation.

Disputes exist but are out of scope until auth/settle flow is solid.

## Implementation Plan

- **Subtask 1 plan:** TBD — rename api → paymanager
- **Subtask 2 plan:** [plan-subtask-2.md](plan-subtask-2.md) — Silvergate service: auth & capture
- **Subtask 3 plan:** [plan-subtask-3.md](plan-subtask-3.md) — integrate Silvergate into Paymanager
- **Subtask 4 plan:** [plan-subtask-4.md](plan-subtask-4.md) — void with capture_delay
- **Subtask 5 plan:** [plan-subtask-5.md](plan-subtask-5.md) — refund in Silvergate
- **Subtask 6 plan:** TBD — refund integration in Paymanager

## Tasks

- [x] **Subtask 1:** Rename `services/api` → `services/paymanager` (go.mod, imports, docker-compose, Makefile, configs, tests)
- [x] **Subtask 2:** Silvergate service — auth & capture flow, mocked bank, webhook callbacks
- [x] **Subtask 3:** Paymanager integration — new payment entities, auth/capture requests to Silvergate, webhook handling
- [x] **Subtask 4:** Void — capture_delay + void endpoint (Silvergate + Paymanager)
- [x] **Subtask 5:** Refund in Silvergate
- [ ] **Subtask 6:** Refund integration in Paymanager

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

**Domain model changes in Paymanager:**
- New payment/transaction entities (separate from existing orders initially)
- Existing order/dispute domain stays untouched for now

## Testing Strategy
(To be filled per subtask)

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
