# Transaction Safety in a Payment System — Notes

Real bugs found and fixed in Silvergate PSP (Go + PostgreSQL). Each section: what broke, why, and what fixed it.

## 1. Concurrent Refund Overdraft (lost update)

**Before:** 3 concurrent refunds ($30 + $40 + $45) on a $50 payment. All read `refunded_amount = 0`, all pass validation, all create refund records. Total refunded: $115 on a $50 payment.

```bash
curl -s -X POST localhost:3000/api/v1/payments/$ID/refund -d '{"amount":3000}' &
curl -s -X POST localhost:3000/api/v1/payments/$ID/refund -d '{"amount":4000}' &
curl -s -X POST localhost:3000/api/v1/payments/$ID/refund -d '{"amount":4500}' &
wait
# All 3 succeed. refunded_amount depends on which goroutine writes last.
```

**Root cause:** Read-modify-write without transaction. Each goroutine reads stale `refunded_amount`, validates against it, and writes independently.

**After (RepeatableRead):** First refund ($30) succeeds. Second gets serialization error (PostgreSQL detects concurrent UPDATE on same row — "first-writer-wins"). Third gets `refund amount exceeds remaining balance` ($45 > $20 remaining).

**Key insight:** PostgreSQL's RepeatableRead is NOT the SQL standard's Repeatable Read. It uses snapshot isolation with write-conflict detection — if two transactions try to UPDATE the same row, the second one is aborted.

## 2. Capture — Same Bug, Same Fix

Same pattern: two concurrent captures on one authorized transaction. Both read `status=authorized`, both mark `capture_pending`, both launch `settleAsync`. Two bank settlements for one payment.

Fix: identical — wrap in RepeatableRead transaction.

## 3. settleAsync Blind Update (optimistic locking)

**Before:** `settleAsync` returns from acquirer after 200ms and does `UPDATE transactions SET status = 'captured' WHERE id = $1`. No check on current status. If something changed the status while waiting (e.g., admin intervention, future void-from-pending) — overwritten silently.

**Reproduced by:** Starting capture, then directly updating DB to `status = 'voided'` while `settleAsync` is blocked on acquirer call. After `settleAsync` returns — status becomes `captured`, overwriting `voided`.

**After:** `UPDATE ... WHERE id = $1 AND status = 'capture_pending'`. If RowsAffected = 0 — status was changed by someone else, `settleAsync` logs error and stops. One line of SQL difference, but fundamentally changes the safety model.

**Key insight:** Every UPDATE in a background goroutine should verify its preconditions. "I read the state 200ms ago" is not the same as "the state is still what I expect."

## 4. Void vs Capture Race (pessimistic locking)

**Before:** Void reads `status=authorized`, then calls acquirer (sync HTTP, 100-500ms). During this time, Capture reads same `authorized` status and proceeds. Both succeed — money is both captured and voided.

**Why RepeatableRead isn't enough here:** Between read and write there's an *external call* to the bank. You don't want two transactions both calling the bank — even if only one will commit. The bank call is irreversible.

**After (SELECT FOR UPDATE):** Void locks the row before calling acquirer. Capture's RepeatableRead transaction tries to UPDATE the same row — waits for Void's lock, then gets serialization error. Or if Capture gets the lock first — Void reads `capture_pending` and rejects with `invalid status transition`.

**Key insight:** Use RepeatableRead when all work is inside the DB. Use SELECT FOR UPDATE when there's an external side effect between read and write.

## 5. CHECK Constraint as Safety Net

Added `CHECK (refunded_amount >= 0)` to transactions table. Not fixing a specific bug — preventing a class of bugs. If `ReleaseRefundAmount` ever gets called with wrong amount (double release, wrong refund ID, integer overflow), the DB rejects it instead of storing `-1000`.

Application code validates too, but bugs happen. The constraint catches what code misses.

## 6. Atomic Release with Status Recalculation

**Before:** When acquirer rejects a refund, `ReleaseRefundAmount` decremented `refunded_amount` but didn't touch `status`. Result: `refunded_amount = 0, status = partially_refunded` — invalid state.

**After:** Single SQL:
```sql
UPDATE transactions
SET refunded_amount = refunded_amount - $1,
    status = CASE WHEN refunded_amount - $1 <= 0 THEN 'captured' ELSE 'partially_refunded' END,
    updated_at = now()
WHERE id = $2
```

Two fields, one statement, one round-trip. No window for inconsistency.

## Decision Framework

After fixing all these, a pattern emerged for choosing the right tool:

| Situation | Tool | Why |
|-----------|------|-----|
| Fast read→validate→write, no side effects | RepeatableRead | First-writer-wins detects conflicts, loser retries or fails |
| External call between read and write | SELECT FOR UPDATE | Lock prevents concurrent operations from even starting |
| Background/async writes | WHERE clause (optimistic) | Verify preconditions at write time, not read time |
| Business invariant that must never break | CHECK constraint | DB-level, independent of application bugs |
| Multiple fields must change together | Single SQL with CASE | No window between individual UPDATEs |
