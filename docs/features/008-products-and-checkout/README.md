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
  - **Spec (вимоги):** [spec-subtask-1.md](spec-subtask-1.md)
  - **План:** [plan-subtask-1.md](plan-subtask-1.md) — step 1/8 done (domain core), 2–8 pending

## Notes
- Created: 2026-04-17
- Subtask 1 scope, data model, API surface і архрішення — у
  [spec-subtask-1.md](spec-subtask-1.md). Цей README — high-level картина фічі.
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

### 2026-05-18 — Subtask 1: grilling → spec → domain core scaffolded
- Прогрилили дизайн крізь дерево рішень (mutability tiers, freeze detection,
  Update API shape, slug semantics, idempotency, lifecycle, listing). Усі
  рішення зафіксовано у [spec-subtask-1.md](spec-subtask-1.md). **Idempotency-Key
  для POST /products знято з scope** як premature — blast radius мінімальний,
  справжня idempotency піде у Subtask 2 для `/purchase`.
- Створено `services/silvergate/internal/product/`: entity, errors, interfaces,
  service, update — 5 файлів. `go build` + `go vet` clean.
- **Key design pattern (decisions #18, #19 у spec):** Update — validated value
  object, конструюється тільки через `NewUpdate(req, p)` ("parse don't validate").
  Repo приймає один `Update{Info, Locked}` замість двох tier-методів. Альтернативу
  з callback-based update (Update carries query/setter func) відкинуто як
  coupling domain → persistence без реального виграшу.
- **`LockedAfterPurchase` struct + `FieldNames()`** — назва типу несе семантику,
  метод — single source of truth для списку locked полів; `Product.LockedFields()`
  делегує туди замість хардкодного `[]string`.
- Створено [plan-subtask-1.md](plan-subtask-1.md) з 8 кроками; Step 1 (domain
  core) done, Steps 2–8 pending (migration, repo, middleware, handlers, wiring,
  tests).

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
