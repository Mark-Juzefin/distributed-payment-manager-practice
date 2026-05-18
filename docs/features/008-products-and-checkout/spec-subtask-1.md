# Spec: Subtask 1 — Products CRUD у Silvergate

> Цей документ — **вимоги** (Jira-style ticket), не план імплементації.
> План з кодом, файлами і порядком кроків — `plan-subtask-1.md` (TBD).

## Мета

Додати у Silvergate доменну сутність **Product** з CRUD-ендпоінтами. Зараз Silvergate знає тільки про низькорівневі операції з картою (auth/capture/void/refund) і не знає "що саме купують". Products — фундамент для `/purchase` endpoint у Subtask 2.

## In Scope

- Доменна модель `Product` з мутабельністю по tier-ам (див. нижче).
- Persistence (PostgreSQL міграція + repo).
- HTTP endpoints: create, read, list, update, archive, unarchive.
- Merchant context через middleware (header-based auth).
- Захист "після першої покупки — semantic поля заморожені" (механізм готовий, активується у Subtask 2).

## Out of Scope (Subtask 1)

- **Хард DELETE** — не реалізуємо. Тільки archive.
- **Idempotency-Key для POST /products** — знято з scope. Blast radius мінімальний (дублі видно у LIST, archive чистить). Повна idempotency реалізується у Subtask 2 для `/purchase` де реально критична.
- **`/purchase` endpoint** — Subtask 2.
- **`external_id` / SKU** — потенційно додамо коли з'явиться use case sync з merchant-овою системою. Зараз тільки `slug` для людино-friendly identifier.
- **JWT/API key auth** — placeholder middleware читає `X-Merchant-ID` header як trust-based context. Справжній auth — окрема фіча.
- **Configurable sort** у LIST — тільки hardcoded `created_at DESC, id DESC`.
- **Total count** у LIST response.

## Data Model

| Колонка | Тип | Обмеження | Tier | Notes |
|---------|-----|-----------|------|-------|
| `id` | UUID | PK, `DEFAULT gen_random_uuid()` | I0 | Server-generated |
| `merchant_id` | TEXT | NOT NULL | I0 | З auth context, не з body |
| `slug` | TEXT | optional, `UNIQUE (merchant_id, slug) WHERE NOT NULL` | I2 | Kebab-case, `^[a-z0-9](-?[a-z0-9])*$`, 1–64 chars |
| `name` | TEXT | NOT NULL | I1 | Display name |
| `description` | TEXT | NOT NULL DEFAULT `''` | I1 | Marketing copy |
| `price` | BIGINT | NOT NULL, `CHECK (price > 0)` | I2 | Minor units (cents) |
| `currency` | TEXT | NOT NULL, `CHECK (length = 3)` | I2 | ISO 4217 |
| `status` | TEXT | NOT NULL DEFAULT `'active'`, `CHECK IN ('active','archived')` | I3 | State machine |
| `first_purchased_at` | TIMESTAMPTZ | NULL | derived | Set by `/purchase` (Subtask 2) на першій успішній покупці. У Subtask 1 ніде не записується, але PATCH-логіка вже його перевіряє |
| `created_at` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | I0 | |
| `updated_at` | TIMESTAMPTZ | NOT NULL DEFAULT `now()` | — | Авто на кожен write |

**Indexes:**
- `idx_products_slug` — `UNIQUE (merchant_id, slug) WHERE slug IS NOT NULL`
- `idx_products_merchant_status_created` — `(merchant_id, status, created_at DESC, id DESC)` для LIST з cursor pagination

## Mutability Tiers

| Tier | Семантика | Поля |
|------|-----------|------|
| **I0** | Frozen forever — set at creation, не змінюється ніколи | `id`, `merchant_id`, `created_at` |
| **I1** | Always mutable — інформативні, не впливають на покупку | `name`, `description` |
| **I2** | Mutable until first purchase — семантичні, після першої покупки заморожені | `slug`, `price`, `currency` |
| **I3** | State machine — окремі endpoints з валідацією transitions | `status` |

**Freeze mechanism:** при першій успішній покупці (Subtask 2) сервер виконує:
```sql
UPDATE products
SET first_purchased_at = now()
WHERE id = $1 AND first_purchased_at IS NULL;
```
Idempotent (тільки перший win-ер ставить timestamp).

**PATCH на I2 полі при frozen state:** atomic conditional update:
```sql
UPDATE products SET price = $1, ...
WHERE id = $2 AND first_purchased_at IS NULL;
```
0 rows affected → 422 з error `fields_locked`.

## API Surface

Усі endpoints вимагають `X-Merchant-ID` header. Middleware відкидає request без header (401).

```
POST   /products                    Create product
GET    /products/:id                Get by UUID
GET    /products                    List (cursor pagination, filter by status)
PATCH  /products/:id                Update I1 + I2 fields (unified endpoint)
POST   /products/:id/archive        I3 transition → archived (idempotent)
POST   /products/:id/unarchive      I3 transition → active (idempotent)
```

### POST /products
**Request:**
```json
{ "slug": "premium-plan"?, "name": "...", "description": "..."?, "price": 1500, "currency": "EUR" }
```
**Response 201:** повний `productResponse` (див. нижче).
**Errors:** 400 validation, 409 slug conflict.

### PATCH /products/:id
**Request:** partial — будь-яке підмножина `{slug?, name?, description?, price?, currency?}`.
**Behavior:**
- Якщо product archived → 422 `product_archived`.
- Якщо клієнт надіслав I2 поле, а `first_purchased_at IS NOT NULL` → 422 `fields_locked` з `{"fields": [...]}`.
- Інакше — оновлення.

### Response shape (GET / POST 201 / PATCH 200)
```json
{
  "id": "...",
  "merchant_id": "...",
  "slug": "...",
  "name": "...",
  "description": "...",
  "price": 1500,
  "currency": "EUR",
  "status": "active",
  "first_purchased_at": "2026-05-12T10:30:00Z" | null,
  "locked_fields": ["slug", "price", "currency"] | [],
  "created_at": "...",
  "updated_at": "..."
}
```

**Important:** `locked_fields` computed server-side:
- `first_purchased_at == nil` → `[]`
- `first_purchased_at != nil` → `["slug", "price", "currency"]`

Клієнт використовує `locked_fields` для UI gating; ніколи не дублює business rule на своєму боці.

### GET /products (LIST)
Cursor-based keyset pagination. Sort hardcoded: `created_at DESC, id DESC`.

**Filter:**
- `?status=active` — тільки active
- `?status=archived` — тільки archived
- absent / empty — both (no filter). Без магічного значення `all`.

Деталі cursor encoding format — implementation detail, вирішується при написанні коду.

### Update DTO (PATCH)
Partial update через **nullable pointers**:
```go
type updateProductRequest struct {
    Slug        *string `json:"slug,omitempty"`
    Name        *string `json:"name,omitempty"`
    Description *string `json:"description,omitempty"`
    Price       *int64  `json:"price,omitempty"`
    Currency    *string `json:"currency,omitempty"`
}
```
- Поле відсутнє у JSON або `null` → не чіпати (обидва означають "skip").
- Не розрізняємо `absent` vs `null` — спрощуємо до single semantic. Для очищення description: `"description": ""`.
- Slug downgrade до `null` після створення — заборонено (rename slug → none = de-facto rename назад; обробляємо як 422).

## Repo Contract

```go
// services/silvergate/internal/product/interfaces.go
type Repo interface {
    Create(ctx, p *Product) error
    GetByID(ctx, merchantID, id) (*Product, error)
    List(ctx, merchantID, filter ListFilter) ([]*Product, *Cursor, error)
    Update(ctx, merchantID, id, upd Update) error
    SetStatus(ctx, merchantID, id, status) error
    MarkPurchased(ctx, merchantID, id) error  // first_purchased_at = now() iff NULL
}
```

**Update flow (parse-don't-validate pattern):**

```go
// Raw HTTP/service input — no guarantees.
type UpdateRequest struct { Name, Description, Slug *string; Price *int64; Currency *string }

// Validated tier-grouped payload — only constructable via NewUpdate.
type Update struct {
    Info   *InfoUpdate           // I1 (name, description) — nil if no I1 changes
    Locked *LockedAfterPurchase  // I2 (slug, price, currency) — nil if no I2 changes
}

func NewUpdate(req UpdateRequest, p *Product) (Update, error)
```

`LockedAfterPurchase.FieldNames()` is the single source of truth for which fields lock after first purchase; `Product.LockedFields()` delegates to it. Repo receives a validated `Update` and builds dynamic SQL based on which group pointers are non-nil. Conditional WHERE (`status='active'`, and when `upd.Locked != nil` also `first_purchased_at IS NULL`) is defense-in-depth against fetch/apply races.

## Architectural Decisions

| # | Рішення | Чому |
|---|---------|------|
| 1 | Embedded price (не Stripe-style Price entity) | Multi-currency не потрібно у Subtask 1. Окремий Price entity — рівень folder-у Subscriptions feature. |
| 2 | Denormalized `first_purchased_at` на products | Чистий domain boundary — product сам володіє своїм freeze state. Atomic conditional UPDATE без cross-domain coupling. |
| 3 | Unified PATCH endpoint + `locked_fields` у response | Server-published capability; client gates UI без знання business rule. Зміна правила (наприклад "frozen after 30 days") не вимагає client update. |
| 4 | Server-generated UUID + optional slug | Mirror існуючого `transactions.id`. Slug для storefront URLs / debug, opt-in. |
| 5 | Slug = I2 (mutable until first purchase) | Симетрично з price; до покупки нічого "не exists" у customer-світі. |
| 6 | Auth middleware `X-Merchant-ID` → `context.Context` | Placeholder для real auth; коли з'явиться JWT, middleware міняється, handlers — ні. Mirror `pkg/correlation/` ідіоми. |
| 7 | No idempotency-key на POST /products | Blast radius мінімальний; повна idempotency машина — у Subtask 2 для `/purchase`. |
| 8 | Archive/unarchive idempotent (no-op якщо вже у target state) | Consequence-light transitions; не варті strict state-machine errors як `transaction.MarkVoided`. |
| 9 | No hard DELETE | Subtask 2 додасть FK з `transactions` → cascade неможливий; archive достатньо для UI. |
| 10 | PATCH блокується якщо status='archived' | Чіткий mental model: archived = immutable до unarchive. |
| 11 | Hardcoded sort `created_at DESC` у LIST | Single index path, predictable performance. Configurable sort — premature. |
| 12 | Cross-merchant isolation = 404 (not 403) | Hide existence для іншого merchant-а; mirror Stripe/GitHub. Стосується GET/PATCH/Archive/Unarchive. |
| 13 | Partial PATCH через nullable pointers; absent ≡ null ≡ "skip" | Простіша семантика. Для очищення description: `""`. RFC 7396 JSON Merge Patch — overkill. |
| 14 | `Repo.MarkPurchased(ctx, productID)` визначаємо у Subtask 1, використовує Subtask 2 | Стабільний контракт між subtasks; freeze-тести у Subtask 1 можуть викликати його напряму замість raw SQL fixtures. |
| 15 | `updated_at` оновлюється Go-side (не DB trigger) | Mirror існуючий `transactions` repo; trigger додає DB-level magic яка ховається від code review. |
| 16 | Last-write-wins для concurrent PATCH | Low write rate; optimistic locking premature. Якщо у проді конфлікти — додаємо `version INT`. |
| 17 | Validation: Gin binding tags для basic + domain service для slug regex / price max; currency whitelist відкладено | Whitelist прив'яжемо до acquirer-supported set у Subtask 2. Зараз — будь-який 3-char код проходить. |
| 18 | Update — validated value object built via `NewUpdate(req, p)`; Repo приймає один `Update` замість двох tier-методів | "Parse don't validate": як тільки тримаєш `Update`, він уже passed всі перевірки (empty, archived, locked, slug rules). Repo стає простіший — один UPDATE з dynamic SET. Альтернатива (Update carries callback/query) відкинута як coupling domain → persistence. |
| 19 | `LockedAfterPurchase` struct + `FieldNames()` метод як single source of truth | Усуває дублікацію між `Product.LockedFields()` (raw `[]string`) і struct fields. Назва типу явно сигналізує семантику ("ці поля locked після покупки"), не описує що це за поля (як попереднє `PricingUpdate`). |

## Open Questions (deferred до plan-subtask-1.md — implementation details)

Архітектурні рішення зафіксовані. Нижче — речі що вирішуємо при написанні коду:

- **Cursor encoding format** — base64-JSON `{created_at, id}` як стандарт; точний layout у коді.
- **Package layout** — наслідуємо `internal/transaction/` (entity/errors/interfaces/service у `internal/product/`, `productrepo/pg.go`, `productcontroller/{create,list,update,archive,unarchive,get}.go`).
- **Test strategy details** — mirror `transactionrepo` integration tests (testcontainers per `.claude/rules/migrations.md`) + unit для service з gomock.

## Explicitly deferred з Subtask 1 (можуть з'явитись потім)

- **Slug lookup endpoint** (path-based `/products/by-slug/:slug` чи `?slug=` filter) — поки storefront use case теоретичний. Додамо коли реальний клієнт попросить.
- **Optimistic locking (version column)** — додамо при першому реальному lost-update bug-у.
- **Currency whitelist** — у Subtask 2 коли інтегруємо з acquirer-supported set.
- **Configurable sort у LIST** — коли admin UI вимагатиме.
- **External_id / SKU** — коли merchant з'явиться з PIM/POS integration.

## Acceptance Criteria

- [ ] Міграція створює таблицю `products` з всіма колонками, індексами, constraints.
- [ ] Middleware `merchantauth` відкидає request без `X-Merchant-ID` (401).
- [ ] POST /products створює продукт, повертає 201 з повним response.
- [ ] GET /products/:id повертає продукт, або 404 якщо merchant-id у context не співпадає з product.merchant_id (cross-merchant isolation).
- [ ] GET /products повертає cursor-paginated список.
- [ ] PATCH /products/:id оновлює I1 поля без обмежень.
- [ ] PATCH /products/:id з I2 полями: успіх якщо `first_purchased_at IS NULL`, 422 з `fields_locked` інакше.
- [ ] PATCH /products/:id на archived → 422 `product_archived`.
- [ ] POST /products/:id/archive idempotent: 200 на active, 200 на already-archived.
- [ ] POST /products/:id/unarchive idempotent: 200 на archived, 200 на already-active.
- [ ] `locked_fields` коректно обчислюється у всіх response-ах.
- [ ] Slug unique per merchant; конфлікт → 409.
- [ ] Integration тест для constraint violations (UNIQUE slug, CHECK currency, CHECK price > 0) — відповідно `.claude/rules/migrations.md`.

## References

- Feature README: [README.md](README.md)
- Existing reference layout: `services/silvergate/internal/transaction/`
- Mutability rules pattern: `services/silvergate/internal/transaction/entity.go:97` (`MarkVoided`)
- Cursor pagination pattern: CLAUDE.md → "Cursor-based pagination" section
- Migration test rules: `.claude/rules/migrations.md`
