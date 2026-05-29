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

- [x] **Subtask 1:** CRUD для продуктів — entity, repo, міграція, HTTP handlers
  - **Spec (вимоги):** [spec-subtask-1.md](spec-subtask-1.md)
  - **План:** [plan-subtask-1.md](plan-subtask-1.md) — all 8 steps done
- [x] **Subtask 2:** `/purchase` endpoint — композиція auth+capture навколо product
  - **Spec (вимоги):** [spec-subtask-2.md](spec-subtask-2.md)
  - **План:** [plan-subtask-2.md](plan-subtask-2.md) — all 10 steps done

## Known Limitations / Future Work

Свідомо прийняті костилі та архітектурні зрізані кути у scope цієї фічі. Кожен — потенційна окрема feature після завершення 008. Деталі — у [spec-subtask-2.md §Known Limitations](spec-subtask-2.md#known-limitations--future-work).

- **F-α: Generic `idempotency_keys` table.** Зараз додаємо окрему колонку `transactions.purchase_idempotency_key` як костиль. Правильний pattern — Stripe-style таблиця `(merchant_id, key, endpoint, request_hash, response_body, ...)` з generic middleware. Виправляє також pre-existing capture overwrite bug.
- **F-β: Intent-record + reconciliation worker.** Зараз `acquirer.Authorize` викликається всередині DB tx. Якщо commit fails після bank approval → lost result. Правильний pattern — INSERT intent (`status=authorizing`) → acquirer call (idempotent) → UPDATE final → reconciliation worker для stuck rows.
- **F-γ: Compensating Void (saga).** Зараз при Capture failure після Authorize approved повертаємо `purchase_partially_persisted` 500 з tx_id, caller manually решає. Правильний pattern — outbox + worker що автоматично робить Void.

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

### 2026-05-26 — Subtask 2: `/purchase` implemented + committed
- Прогрилили дизайн крізь 11 питань → [spec-subtask-2.md](spec-subtask-2.md)
  + [plan-subtask-2.md](plan-subtask-2.md). Усі 10 кроків done, committed.
- **Idempotency двошарова:** `checkIdempotency` pre-check (швидкий шлях, економить
  acquirer-виклик на очевидному replay) + UNIQUE index `idx_transactions_purchase_idempotency`
  як справжній backstop під конкурентністю. Програш race на INSERT (23505 →
  `ErrPurchaseIdempotencyConflict`) → `resolveRace` re-fetch'ить переможця. `sameRequest`
  розрізняє replay від key-reuse (→ 409) в обох точках.
- **Окрема колонка `purchase_idempotency_key`** замість generic table — костиль
  (F-α), бо `idempotency_key` перетирається Capture. Задокументовано.
- **F-β підтверджено як реальний баг:** `acquirer.Authorize` викликається ДО
  claim'у ключа (всередині tx, перед INSERT). У програшному race req2 авторизує
  картку вдруге → orphan auth hold. Терпимо лише бо mock acquirer + вузьке вікно.
  Реальний фікс = intent-record first + idempotent acquirer + reconciliation worker.
- **Тести:** 10 unit (hand-rolled fakes) для `purchase.Service` — green. Integration
  на UNIQUE constraint + `GetByPurchaseIdempotencyKey` — compile clean, runtime
  потребує docker (`make integration-test`, ще не прогнано користувачем).
- **Code review pass:** прибрано verbose feature-ref коментарі (spec-path, F-α/β/γ,
  "Subtask 2") з коду — деталі лишились у spec/README. `SetPurchaseContext` →
  `MarkProductPurchase`. Додано Returns-докстрінги до `Purchase` + `checkIdempotency`.
- **Фіча 008: обидва сабтаски done.** Кандидат на закриття — звірити з roadmap
  при наступному старті.

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
