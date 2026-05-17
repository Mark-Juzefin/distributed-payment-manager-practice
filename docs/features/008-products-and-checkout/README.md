# Feature: Silvergate Products

**Status:** In Progress

## Overview

Розширити Silvergate доменом продуктів. Зараз сервіс знає тільки про
низькорівневі операції з картою (auth/capture/void/refund) — треба додати
поняття "що саме купують".

**Що додаємо:**
- CRUD для продуктів (entity, repo, міграція, HTTP handlers)
- Один високорівневий endpoint типу `POST /purchase` — приймає
  `{merchant_id, order_id, product_id, card_token}`, всередині дістає
  продукт з БД, викликає вже існуючу логіку auth+capture, повертає
  `transaction_id`

Існуючі low-level endpoints стають внутрішніми будівельними блоками,
`/purchase` тільки їх композує.

## Tasks

- [ ] **Subtask 1:** CRUD для продуктів — entity, repo, міграція, HTTP handlers

## Notes
- Created: 2026-04-17
- Деталі (схема таблиці, контракти DTO, статуси продукту) — у
  `plan-subtask-N.md` коли почнемо конкретну підзадачу
- Архітектурні рішення для Subtask 1 (2026-05-17):
  - Product entity мінімальний: `ID, MerchantID, Name, Price, Currency, Status, CreatedAt`
  - Embedded price (НЕ Stripe-style окремий Price entity) — multi-currency
    лишимо до Subscriptions feature
  - `merchant_id` required (security boundary при auth)
  - Idempotency-Key header для `POST /products`
- Існуючі low-level endpoints (`/auth`, `/capture`, `/void`, `/refund`) НЕ
  змінюються — `/purchase` тільки композує auth+capture
- **Передумова виконана (2026-05-17):** канонічна структура PayManager
  (`internal/<domain>/<domainXxx>/`) дзеркалена на Silvergate. Обидва
  сервіси тепер слугують reference для нових доменів.
- **Test strategy:** pre-existing broken integration tests НЕ ревайвимо
  (залежать від відсутніх `pool` / `applyBaseFixture` helpers). Нові
  тести Silvergate пишемо bottom-up: unit для `transaction.Service` з
  mocks (Repo, Acquirer, WebhookSender) + integration для
  `transactionrepo` з testcontainers. e2e/ — окрема задача при
  поверненні до PayManager.

## Session Log

### 2026-05-17 — Phase 2 архітектура + PayManager refactor як підготовка
- Зафіксовано доменну модель Subtask 1 і форму `/purchase` endpoint
- PayManager: канонічну структуру завершено (7/8 кроків, **uncommitted**).
  Step 6 (event sink unification) відкладено — потребує schema migration
  + розширення `eventstore.Store` методом `Get`
- 008/009/010 README переписано з нагалюцинованих planів у brief intent
- Створено `/checkpoint` skill для збереження контексту між сесіями

### 2026-05-17 — Silvergate структура дзеркальна; test strategy
- Канонічну структуру застосовано до Silvergate: `acquirer/`, `webhook/`,
  `domain/transaction/`, `handlers/`, `repo/` всі переїхали під `internal/`
  з role-based підпакетами `transactioncontroller/`, `transactionrepo/`;
  `Repo` + `WebhookSender` витягнуто в `interfaces.go`
- `go build ./...` + `go vet ./...` clean для обох сервісів
- PayManager refactor закомічено окремо (commit `abd0bea`)
- **Test strategy decision:** забити на pre-existing broken тести
  (e2e + integration tests з відсутніми pool/fixture helpers). Bottom-up:
  спершу написати норм тести Silvergate, до e2e повертатися коли почнемо
  PayManager роботу
- **e2e/ cross-module + internal/ block:** e2e — окремий Go module, не
  може імпортувати з `internal/`. Розв'язується або винесенням публічних
  DTO у paymanager, або перенесенням e2e всередину модуля. Deferred.
