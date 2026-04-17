# Subscription Engine with Temporal

**Status:** Not Started

## Overview

A new microservice (`cmd/subscriptions`) that implements a **subscription billing engine** on top of a payment provider (Solidgate-like API). The service owns the full subscription lifecycle, billing scheduling, retry logic (dunning), and invoice history. Long-running workflows are orchestrated by **Temporal**.

**Learning goals:**
- Temporal workflows & activities — durable execution, timers, signals, retries
- Subscription domain modeling — lifecycle state machine, billing cycles, dunning
- Payment provider integration — tokenized recurring charges, webhook reconciliation
- Saga-like coordination — multi-step billing with compensation on failure

**Key principle:** The payment provider is a **payment primitive** (charge, tokenize, refund). All subscription intelligence lives in our service.

## Architecture

```
                         Temporal Server
                              |
                    +---------+---------+
                    |                   |
              SubscriptionWf       BillingCycleWf
              (lifecycle)          (per-period)
                    |                   |
                    v                   v
            +-------------------+  +-------------------+
            | Subscription Svc  |  | Payment Activity  |
            | (API + domain)    |  | (Solidgate calls) |
            +-------------------+  +-------------------+
                    |                   |
                    v                   v
               PostgreSQL          Solidgate API
               (state, invoices)   (charge, refund, tokenize)
                    ^
                    |
              Webhook Handler
              (payment.success, payment.failed, etc.)
```

## Solidgate API Surface (to simulate via Wiremock)

### Payments
- `POST /payments` — create a charge (one-time or recurring with token)
- `GET /payments/{id}` — get payment status
- `POST /payments/{id}/refund` — refund a settled payment
- `POST /payments/{id}/void` — cancel before settlement

Payment statuses: `created -> processing -> authorized -> settled / declined`

### Tokenization
- Token is created during the first payment (card-on-file)
- Subsequent charges use the token for recurring billing

### Subscriptions (provider-side, optional)
- `POST /subscriptions` — create
- `GET /subscriptions/{id}` — get
- `PATCH /subscriptions/{id}` — update
- `POST /subscriptions/{id}/cancel` — cancel
- `POST /subscriptions/{id}/update-token` — change payment method

### Products / Prices (optional, for billing catalog)
- `POST /products`, `POST /prices`, `GET /prices`
- Defines billing frequency, trial period, currency

### Webhooks (inbound)
- `payment.updated` — payment status change
- `subscription.updated` — subscription status change
- `chargeback` — dispute opened
- `card.updated` — card details changed (e.g., new expiry)

## Our Subscription API

```
POST   /subscriptions                          — create subscription
GET    /subscriptions/{id}                     — get subscription details
POST   /subscriptions/{id}/cancel              — cancel (end of period or immediate)
POST   /subscriptions/{id}/pause               — pause billing
POST   /subscriptions/{id}/resume              — resume billing
POST   /subscriptions/{id}/update-payment-method — update card token
GET    /subscriptions/{id}/invoices            — invoice history
```

## Domain Model

### Subscription Lifecycle State Machine

```
                  +---> Paused ---+
                  |               |
Created -> Active +---> PastDue --+--> Canceled
                  |               |
                  +---> Expired --+
```

- **Created** — subscription created, awaiting first payment
- **Active** — current billing cycle paid, service active
- **Paused** — user-initiated pause, no billing, service may be limited
- **PastDue** — payment failed, in dunning/retry period
- **Canceled** — terminal state (user-initiated or failed dunning)
- **Expired** — terminal state (fixed-term subscription ended)

### Invoice / Billing Model

```
Subscription
  |-- Plan (product, price, interval)
  |-- PaymentMethod (tokenized card)
  |-- Invoices[]
        |-- billing_period_start / end
        |-- amount, currency
        |-- status: draft / open / paid / failed / void
        |-- PaymentAttempts[]
              |-- provider_payment_id
              |-- status, error_code
              |-- attempted_at
```

## Temporal Workflows

### 1. SubscriptionWorkflow (long-running, per subscription)

Orchestrates the full subscription lifecycle:

```
start:
  create subscription record
  loop:
    schedule BillingCycleWorkflow (child workflow)
    wait for cycle result
    if paid -> advance next_billing_at, continue
    if dunning_exhausted -> cancel subscription
    listen for signals: cancel, pause, resume, update_payment_method
```

### 2. BillingCycleWorkflow (child, per billing period)

Handles a single billing period with retry logic:

```
start:
  create invoice (draft -> open)
  call POST /payments (activity)
  wait for webhook signal (payment.success / payment.failed)
  if success -> mark invoice paid, return success
  if failed -> enter dunning:
    retry in 1 day
    retry in 3 days
    retry in 5 days
    if all retries exhausted -> return dunning_failed
```

### 3. Key Temporal Features to Practice

- **Timers** — `workflow.Sleep()` for billing schedule and retry delays
- **Signals** — cancel, pause, resume sent as signals to running workflow
- **Child workflows** — BillingCycleWorkflow as child of SubscriptionWorkflow
- **Activities** — payment provider calls, DB writes, webhook sends
- **Activity retries** — automatic retry with backoff for transient failures
- **Workflow queries** — get current subscription state without DB read
- **Continue-as-new** — prevent history growth for long-running subscriptions

## Tasks

- [ ] Subtask 1: Project setup — Temporal dev server in docker-compose, new `cmd/subscriptions` service skeleton, basic domain model
- [ ] Subtask 2: Subscription CRUD — API endpoints, PostgreSQL schema, subscription creation flow
- [ ] Subtask 3: Solidgate mock — Wiremock stubs for payments API, tokenization flow
- [ ] Subtask 4: BillingCycleWorkflow — create invoice, charge payment, handle webhook result
- [ ] Subtask 5: SubscriptionWorkflow — lifecycle orchestration, billing loop, signal handling (cancel/pause/resume)
- [ ] Subtask 6: Dunning & retry — exponential retry on payment failure, past_due state, dunning exhaustion
- [ ] Subtask 7: Payment method update — update token flow, retry with new card
- [ ] Subtask 8: Invoice history & API — invoice listing, payment attempt details
- [ ] Subtask 9: Integration tests — Temporal test framework, workflow replay tests, E2E with Wiremock
- [ ] Subtask 10: Observability — Temporal metrics in Prometheus, Grafana dashboard for subscription health

## Notes

- Temporal dev server (`temporalite` or official `temporal-server` Docker image) is sufficient for learning
- Wiremock will simulate all Solidgate API responses (success, decline, timeout scenarios)
- This is a separate service from the existing API/Ingest — its own DB schema, its own domain
- Event sourcing is optional here — Temporal workflow history already provides an audit trail
- Consider whether to reuse the existing shared kernel or keep this service fully independent
