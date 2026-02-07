# План: Logger Refactoring (Tech Debt)

## Scope

Міграція на slog (Go stdlib) з трьома цілями:
1. **Structured logging API** — slog native API замість printf-style
2. **Automatic source location** — file:line в кожному лозі
3. **Context-first design** — correlation ID автоматично з context

> **TODO (out of scope):**
> - Log sampling для high-frequency events
> - Performance optimization (zerolog handler якщо потрібно)

## Поточний стан

```go
// Printf-style, окремі методи для context
l.Info("Starting server: port=%d", cfg.Port)
l.ErrorCtx(ctx, "Failed: event_id=%s error=%v", eventID, err)
```

**Проблеми:**
- Не structured — `event_id=%s` замість окремих полів
- Немає source location
- Дублювання: `Info()` vs `InfoCtx()`

## Архітектура

### slog Handler Chain

```
Request → CorrelationHandler → JSONHandler → stdout
                ↓                    ↓
         adds correlation_id    formats JSON + source
```

### Package Structure

```
pkg/logger/
├── logger.go      # Setup(), Options, level parsing
├── correlation.go # CorrelationHandler
├── gin.go         # Gin middleware (updated)
└── logger_test.go
```

## API Design

### Setup

```go
package logger

import "log/slog"

type Options struct {
    Level   string // debug, info, warn, error
    Console bool   // pretty print for dev (LOG_FORMAT=console)
}

// Setup configures global slog logger
func Setup(opts Options) {
    var handler slog.Handler

    handlerOpts := &slog.HandlerOptions{
        Level:     parseLevel(opts.Level),
        AddSource: true, // automatic file:line
    }

    if opts.Console {
        handler = slog.NewTextHandler(os.Stdout, handlerOpts)
    } else {
        handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
    }

    // Wrap with correlation handler
    handler = NewCorrelationHandler(handler)

    slog.SetDefault(slog.New(handler))
}
```

### CorrelationHandler

```go
type CorrelationHandler struct {
    inner slog.Handler
}

func (h *CorrelationHandler) Handle(ctx context.Context, r slog.Record) error {
    if corrID := correlation.FromContext(ctx); corrID != "" {
        r.AddAttrs(slog.String("correlation_id", corrID))
    }
    return h.inner.Handle(ctx, r)
}

// + Enabled, WithAttrs, WithGroup делегують до inner
```

### Usage

```go
// Startup (without context)
slog.Info("Starting server", "port", cfg.Port)

// Request handling (with context)
slog.InfoContext(ctx, "Order processed",
    slog.String("order_id", orderID),
    slog.String("status", status))

// Errors
slog.ErrorContext(ctx, "Failed to process",
    slog.String("order_id", orderID),
    slog.Any("error", err))
```

### Output

```json
{
  "time": "2026-01-27T10:00:00Z",
  "level": "INFO",
  "source": {"function": "main.HandleMessage", "file": "consumers/order.go", "line": 42},
  "msg": "Order processed",
  "correlation_id": "abc-123",
  "order_id": "ord_001",
  "status": "success"
}
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `pkg/logger/logger.go` | Rewrite: `Setup()` + level parsing |
| `pkg/logger/correlation.go` | New: `CorrelationHandler` |
| `pkg/logger/gin.go` | Update: use slog for body logging |
| `internal/api/app.go` | Migrate to slog |
| `internal/api/workers.go` | Migrate to slog |
| `internal/api/consumers/order.go` | Migrate to `slog.InfoContext()` |
| `internal/api/consumers/dispute.go` | Migrate to `slog.InfoContext()` |
| `internal/ingest/app.go` | Migrate to slog |
| `go.mod` | Remove `github.com/rs/zerolog` (якщо більше не потрібен) |

## Порядок імплементації

### Step 1: New Logger Package

1. Create `Setup()` function with `Options`
2. Implement `CorrelationHandler`
3. Add level parsing (reuse existing logic)
4. Keep old `Logger` type temporarily for compatibility

### Step 2: Migrate Consumers

```go
// Before
c.logger.InfoCtx(ctx, "Order processed: event_id=%s", env.EventID)

// After
slog.InfoContext(ctx, "Order processed", "event_id", env.EventID)
```

### Step 3: Migrate App/Workers

```go
// Before
l.Info("Starting server: port=%d", cfg.Port)

// After
slog.Info("Starting server", "port", cfg.Port)
```

### Step 4: Update Gin Middleware

- `GinBodyLogger()` → use slog
- `CorrelationMiddleware()` — keep as is (stores in context)

### Step 5: Cleanup

1. Remove old `Logger` type and methods
2. Remove zerolog dependency (if not used elsewhere)
3. Update any remaining usages

## Migration Example

### Before
```go
func (c *OrderMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
    c.logger.DebugCtx(ctx, "Processing: event_id=%s key=%s", env.EventID, string(key))

    if err := c.service.ProcessPaymentWebhook(ctx, webhook); err != nil {
        c.logger.ErrorCtx(ctx, "Failed: event_id=%s error=%v", env.EventID, err)
        return err
    }

    c.logger.InfoCtx(ctx, "Processed: event_id=%s order_id=%s", env.EventID, webhook.OrderId)
    return nil
}
```

### After
```go
func (c *OrderMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
    slog.DebugContext(ctx, "Processing order message",
        "event_id", env.EventID,
        "key", string(key))

    if err := c.service.ProcessPaymentWebhook(ctx, webhook); err != nil {
        slog.ErrorContext(ctx, "Failed to process order webhook",
            "event_id", env.EventID,
            slog.Any("error", err))
        return err
    }

    slog.InfoContext(ctx, "Order webhook processed",
        "event_id", env.EventID,
        "order_id", webhook.OrderId)
    return nil
}
```

**Зміни в структурі:**
- Немає `c.logger` — використовуємо глобальний `slog`
- correlation_id додається автоматично через handler
- source location додається автоматично через `AddSource: true`