# Feature: Silvergate Transactions Deep Dive

**Status:** Planned

## Overview

Continuation of [Feature 007](../007-payment-system-logic/) — from single-row transaction safety to multi-row financial transactions. Silvergate becomes a playground for studying PostgreSQL transaction complexity: balances, double-entry ledger, concurrent load, deadlocks.

**Prerequisite:** Feature 007 Phase 1-2 done (core flow + single-row locking patterns).

**Why separate feature:** Phase 1-2 covered the basics (RepeatableRead, optimistic/pessimistic locking, CHECK constraints). This feature is a different depth — multi-row atomicity, isolation levels trade-offs, deadlock scenarios, concurrent load. Worth its own planning and focus.

## Motivation

Current Silvergate operations are simple single-row writes. Real financial systems move money between accounts, which means:

```
Auth:    card.available -= amount, card.held += amount
Capture: card.held -= amount, merchant.available += amount
Void:    card.held -= amount, card.available += amount
Refund:  merchant.available -= amount, card.available += amount
```

Every operation becomes **two atomic writes** — the core of transaction complexity.

## Tasks

- [ ] **Subtask 1:** Balances table (card + merchant accounts) — schema, repo, two-write operations
  - Atomicity: if second write fails, first must roll back
  - Isolation: two concurrent auths on same card must not overdraft
  - Lost update problem: `SELECT balance → check → UPDATE` without proper locking

- [ ] **Subtask 2:** Double-entry ledger — debit + credit per movement
  - Every money movement = two entries, sum always = 0
  - CHECK constraint on sum = 0
  - Deadlocks on concurrent writes to same account (A→B vs B→A)
  - Fintech standard pattern

- [ ] **Subtask 3:** Concurrent payments on one card — isolation level comparison
  - **Read Committed** — sees committed data, phantom reads possible
  - **Repeatable Read** — serialization errors on conflicts, retry logic needed
  - **Serializable** — full isolation, maximum retries
  - Demonstrate race conditions and how each level fixes/fails them

- [ ] **Subtask 4:** Idempotency with concurrent requests
  - Two identical capture requests arrive simultaneously
  - Compare: `INSERT ON CONFLICT` vs `SELECT FOR UPDATE + check`
  - Trade-offs: performance, clarity, edge cases

- [ ] **Subtask 5:** Concurrent load test — k6/loadtest hitting auth/capture in parallel
  - Provoke race conditions at scale
  - Measure serialization error rates under different isolation levels
  - Observe deadlock frequency

- [ ] **Subtask 6:** Deadlock scenarios — provoke and analyze
  - Ordered lock acquisition to prevent deadlocks
  - Deadlock detection in PostgreSQL logs
  - Retry strategies for deadlock victims

## Notes
- Created: 2026-04-17
- Continuation of: Feature 007 (Payment System Logic)
- All work happens in `services/silvergate/` — Paymanager is untouched
