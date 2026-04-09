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
- **Subtask 3 plan:** TBD — integrate Silvergate into Paymanager (new payment entities)
- **Subtask 4 plan:** TBD — void & refund in Silvergate
- **Subtask 5 plan:** TBD — void & refund integration in Paymanager

## Tasks

- [x] **Subtask 1:** Rename `services/api` → `services/paymanager` (go.mod, imports, docker-compose, Makefile, configs, tests)
- [x] **Subtask 2:** Silvergate service — auth & capture flow, mocked bank, webhook callbacks
- [ ] **Subtask 3:** Paymanager integration — new payment entities, auth/capture requests to Silvergate, webhook handling
- [ ] **Subtask 4:** Void & refund in Silvergate
- [ ] **Subtask 5:** Void & refund integration in Paymanager

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

## Notes
- Created: 2026-04-09
- Inspired by: https://docs.solidgate.com/payments/payments-overview/#auth--settle
- Existing Wiremock stubs for Silvergate will be replaced by the real service
