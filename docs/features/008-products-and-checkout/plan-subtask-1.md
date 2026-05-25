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
- [x] **Step 2: Migration** — `services/silvergate/migrations/20260518001_create_products_table.sql`
  - Колонки і constraints за [spec Data Model](spec-subtask-1.md#data-model)
  - Partial unique index на `(merchant_id, slug) WHERE slug IS NOT NULL`
  - List index на `(merchant_id, status, created_at DESC, id DESC)`
  - **Без `CHECK` на `status`** — business-invariant вже у Go (`Status` type), у схемі не дублюємо
- [x] **Step 3: Repo impl** — `internal/product/productrepo/pg.go`
  - Implements `product.Repo` interface
  - `Update` будує dynamic SQL через Squirrel: SET-и тільки для non-nil pointers у `upd.Info` та `upd.Locked`
  - Conditional WHERE `status='active'` (на write methods), `first_purchased_at IS NULL` (коли `upd.Locked != nil`)
  - При 0 rows від guarded UPDATE — `disambiguateUpdateMiss` re-fetch розпізнає `ErrArchived` / `ErrFieldsLocked` / `ErrNotFound`
  - `MarkPurchased`: idempotent. 0 rows + row existує → no-op (вже purchased); 0 rows + не існує → `ErrNotFound`
  - Cursor encoding: base64-RawURL JSON `{c: created_at, i: id}` — `EncodeCursor` / `DecodeCursor` exported для controllers
- [x] **Step 4: Middleware** — `internal/merchantauth/middleware.go`
  - Читає `X-Merchant-ID` header, abort 401 якщо порожній
  - `merchantauth.FromContext(ctx) (string, bool)` для handlers
  - `WithMerchantID(ctx, id)` exported для тестів (без реального middleware)
- [x] **Step 5: HTTP handlers** — `internal/product/productcontroller/`
  - `update.go` — PATCH /products/:id; errors → 404/409/422; `productResponse` з computed `locked_fields`; shared `errorResponse{error,code,fields}`
  - `create.go` — POST /products; ErrInvalidSlug → 422, ErrSlugConflict → 409
  - `get.go` — GET /products/:id; ErrNotFound → 404
  - `list.go` — GET /products?status=&cursor=&limit=; cursor encode/decode delegates to `productrepo.{En,De}codeCursor`
  - `archive.go` / `unarchive.go` — POST /products/:id/{archive,unarchive}; повертає оновлений productResponse; shared `resolveProductIdentity` + `writeStatusError`
- [x] **Step 6: Router wiring** — `app.go` + `router.go`
  - DI bundle `productHandlers` (Create/Get/List/Update/Archive/Unarchive) у app.go
  - `/api/v1/products` group під `merchantauth.Middleware()` з POST/GET/PATCH + POST `:id/archive`, `:id/unarchive`
- [x] **Step 7: Unit tests**
  - `update_test.go` — `NewUpdate` table-driven (empty/info/locked/mixed groups; archived; purchased+locked vs purchased+info-only; slug downgrade, invalid slug, valid slug) + `LockedAfterPurchase.FieldNames`, `Product.LockedFields`, `ValidateSlug` cases
  - `service_test.go` — hand-rolled `fakeRepo`: Create (slug pre-validation, repo error pass-through, merchant id set), List (default/max limit, filter forwarding), Update (double-fetch pre+post, snapshot error, locked rejection skips repo.Update), Archive/Unarchive (status forwarded), MarkPurchased forwarded
  - `gomock` не використано — hand-rolled fake простіше для маленького Repo interface (mirror `acquirer/mock_acquirer.go`)
- [x] **Step 8: Integration tests** — `productrepo/{integration_test.go,pg_integration_test.go}`
  - Per `.claude/rules/migrations.md`: unique slug + nil-slug, CHECK (price>0, currency length), CHECK violations surface as `pgconn.PgError` 23xxx
  - Freeze flow через `MarkPurchased` → repo.Update повертає `ErrFieldsLocked`; info-only updates після purchase усе ще проходять
  - Pagination/filter coverage: ordering DESC, cursor resume, nil/active/archived status filters
  - Cross-merchant isolation для `GetByID`/`Update`/`SetStatus`/`MarkPurchased` → `ErrNotFound`
  - testcontainers-based (`postgres:17`); кожен тест ізольований через unique `merchant_id`, no truncate

## Open Implementation Decisions

Вирішуються при написанні відповідного step:
- Cursor encoding bytes (Step 3) — base64 JSON якщо нема кращої ідеї
- Чи виносити `merchantauth` у `pkg/` (Step 4) — рекомендовано лишити в `internal/` поки не знадобиться у PayManager (Rule of Three)
- Чи генерувати mock для Repo через `mockgen` (Step 7) — додаємо `//go:generate` directive у `interfaces.go` коли пишемо тести
