# План: Subtask 1 — Products CRUD у Silvergate

Реалізація вимог [spec-subtask-1.md](spec-subtask-1.md). План — ordered steps; вимоги, data model, архрішення — у spec.

## Прогрес

- [x] **Step 1: Domain core** — `services/silvergate/internal/product/`
  - `entity.go` (Product, Status, ValidateSlug, LockedFields)
  - `errors.go`
  - `interfaces.go` (Repo, ListFilter, Cursor)
  - `service.go` (Create, Get, List, Update, Archive, Unarchive, MarkPurchased)
  - `update.go` (UpdateRequest → Update validated by NewUpdate; LockedAfterPurchase as single source of truth)
  - Build clean (`go build`, `go vet`)
- [ ] **Step 2: Migration** — `services/silvergate/migrations/YYYYMMDD_create_products_table.sql`
  - Колонки і constraints за [spec Data Model](spec-subtask-1.md#data-model)
  - Partial unique index на `(merchant_id, slug) WHERE slug IS NOT NULL`
  - List index на `(merchant_id, status, created_at DESC, id DESC)`
- [ ] **Step 3: Repo impl** — `internal/product/productrepo/pg.go`
  - Implements `product.Repo` interface
  - `Update` будує dynamic SQL через Squirrel: SET-и тільки для non-nil pointers у `upd.Info` та `upd.Locked`
  - Conditional WHERE `status='active'` (на write methods), `first_purchased_at IS NULL` (коли `upd.Locked != nil`)
  - Cursor encoding: base64-JSON `{created_at, id}`
- [ ] **Step 4: Middleware** — `internal/merchantauth/`
  - Читає `X-Merchant-ID` header, injects у `context.Context`
  - 401 якщо header порожній
  - `merchantauth.MerchantID(ctx) (string, bool)` для handlers
- [ ] **Step 5: HTTP handlers** — `internal/product/productcontroller/`
  - Один файл на endpoint: `create.go`, `get.go`, `list.go`, `update.go`, `archive.go`, `unarchive.go`
  - Request/response DTOs з Gin binding tags
  - Mapping domain errors → HTTP statuses (404, 409, 422)
  - `productResponse` includes computed `locked_fields` через `p.LockedFields()`
- [ ] **Step 6: Router wiring** — `app.go` + `router.go`
  - Реєструємо product repo, service, handlers у DI
  - Route group `/products/*` під `merchantauth.Middleware()`
- [ ] **Step 7: Unit tests**
  - `update_test.go` — `NewUpdate` table-driven: всі error paths + happy path
  - `service_test.go` — Service методи з mock Repo (gomock)
- [ ] **Step 8: Integration tests** — `productrepo/pg_integration_test.go`
  - Per `.claude/rules/migrations.md`: covers unique slug constraint, CHECK constraints (currency length, price > 0), partial unique behavior with NULL slugs
  - Freeze flow через `MarkPurchased` → conditional UPDATE returns 0 rows
  - testcontainers-based

## Open Implementation Decisions

Вирішуються при написанні відповідного step:
- Cursor encoding bytes (Step 3) — base64 JSON якщо нема кращої ідеї
- Чи виносити `merchantauth` у `pkg/` (Step 4) — рекомендовано лишити в `internal/` поки не знадобиться у PayManager (Rule of Three)
- Чи генерувати mock для Repo через `mockgen` (Step 7) — додаємо `//go:generate` directive у `interfaces.go` коли пишемо тести
