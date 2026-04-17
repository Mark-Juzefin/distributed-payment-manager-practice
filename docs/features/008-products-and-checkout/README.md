# Feature: Products & Checkout

**Status:** Planned

## Overview

Transform Paymanager from a building-blocks proxy (exposing capture/void/refund as API) into a business orchestrator where payment operations are internal implementation details driven by product properties.

**Core idea:** The client (buyer) doesn't know what "capture" or "void" is. They know "buy", "cancel", and "where is my order". Paymanager decides _when_ and _how_ to capture/void/refund based on product configuration.

**Before (current):**
```
Client → POST /capture → Paymanager → POST /capture → Silvergate
         (building block)               (building block)
```

**After:**
```
Client → POST /checkout {product, card}
         Paymanager decides:
           - product.capture_strategy = "immediate" → capture now
           - product.capture_strategy = "delayed"   → capture after product.capture_delay
         Client never sees capture/void/refund — only business outcomes
```

## Client-facing API (target)

```
POST /api/v1/products                    # Admin: create product
GET  /api/v1/products                    # Catalog: list products
GET  /api/v1/products/:id                # Catalog: get product

POST /api/v1/checkout                    # Buyer: purchase (auth + auto-capture)
GET  /api/v1/orders/:id                  # Buyer: order status
POST /api/v1/orders/:id/cancel           # Buyer: cancel (Paymanager decides void vs refund)
```

Internal operations (capture, void, refund) are no longer exposed as API endpoints.

## Domain Model

```go
type Product struct {
    ID              string
    Name            string
    Price           int64
    Currency        string
    CaptureStrategy string        // "immediate" | "delayed"
    CaptureDelay    time.Duration // 0 for immediate, e.g. 72h for physical goods
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**Capture strategy by product type:**

| Type | CaptureStrategy | CaptureDelay | Example |
|------|-----------------|--------------|---------|
| Digital | `immediate` | 0 | Subscription, ebook, in-app purchase |
| Physical | `delayed` | 72h | Shipped goods |

## Key Architectural Decisions

1. **Product drives capture timing** — `CaptureDelay` from `payment.CreatePayment()` is no longer a client parameter, it comes from the product
2. **Cancel = smart routing** — `POST /cancel` inspects payment state: if not captured yet → void, if already captured → refund
3. **Checkout = orchestration** — single endpoint that reads product, validates stock/price, creates payment with correct capture strategy
4. **Existing payment domain reused** — `payment.CreatePayment()` already supports `captureDelay`, checkout just feeds it from product config

## Tasks

### Phase 1: Paymanager cleanup — resolve legacy domains

Current state after Feature 007: three overlapping domains in Paymanager:
- `domain/order/` — legacy webhook-based orders with duplicate `CapturePayment()` endpoint
- `domain/payment/` — newer auth/capture/void/refund that proxies Silvergate building blocks
- `domain/dispute/` — legacy chargeback handling tied to old order model

- [ ] **Subtask 1:** Audit and plan domain consolidation — decide what stays, what gets removed, what merges
- [ ] **Subtask 2:** Remove legacy order capture endpoint (`POST /orders/:order_id/capture`) and related proxy logic
- [ ] **Subtask 3:** Consolidate payment domain — single clean domain that owns the payment lifecycle internally
- [ ] **Subtask 4:** Decide dispute fate — keep as-is, migrate to new model, or defer to later feature

### Phase 2: Product catalog

- [ ] **Subtask 5:** Product domain — entity, repo interface, PostgreSQL migration, CRUD service
- [ ] **Subtask 6:** Product HTTP handlers — `POST/GET /api/v1/products`

### Phase 3: Checkout orchestration

- [ ] **Subtask 7:** Checkout domain — orchestration service that ties product + payment, capture strategy from product config
- [ ] **Subtask 8:** Checkout HTTP handler — `POST /api/v1/checkout`
- [ ] **Subtask 9:** Cancel endpoint — `POST /api/v1/orders/:id/cancel` (smart void vs refund routing)
- [ ] **Subtask 10:** Remove raw capture/void/refund from public API (keep as internal)

### Phase 4: Validation

- [ ] **Subtask 11:** E2E tests — full checkout → capture → cancel flows for digital and physical products

## Notes
- Created: 2026-04-17
- Builds on top of Feature 007 (Silvergate must be solid before this starts)
- Existing `payment/service.go` already has `captureInBackground()` and `captureWithDelay()` — reuse them
- Silvergate stays unchanged — it's still the PSP with building-block API
