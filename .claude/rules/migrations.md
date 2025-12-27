# Database Migration Rules

## Test Coverage Requirement

**CRITICAL RULE: Every database migration that adds constraints, indices, or modifies schema MUST be covered by integration tests.**

### When to Add Tests

Add integration tests when your migration includes:
- ✅ UNIQUE constraints
- ✅ Foreign key constraints
- ✅ CHECK constraints
- ✅ New indices that affect query behavior
- ✅ Table partitioning
- ✅ Column type changes that affect validation

### Where to Add Tests

Tests go in the repository module that interacts with the migrated table:

```
Migration file: internal/app/migrations/20251227102937_add_idempotency_constraints.sql
Test location:  internal/repo/order_eventsink/pg_order_event_sink_integration_test.go
                internal/repo/dispute_eventsink/pg_event_sink_integration_test.go
```

### What to Test

1. **UNIQUE Constraints**
   - Verify duplicates are rejected
   - Verify error returns `apperror.ErrEventAlreadyStored` (or appropriate domain error)
   - Test with valid variations (e.g., same key different entity)

2. **Foreign Keys**
   - Verify invalid references are rejected
   - Test cascade behavior if applicable

3. **Partitioned Tables**
   - UNIQUE constraints MUST include all partitioning columns
   - Test with identical partitioning key values to trigger violation

### Example Test Structure

```go
func TestCreateEvent_IdempotencyConstraint(t *testing.T) {
    tests := []struct {
        name                 string
        firstEvent           Event
        duplicateEvent       Event
        expectDuplicateError bool
    }{
        {
            name: "Duplicate (entity_id, provider_event_id) returns ErrEventAlreadyStored",
            firstEvent: Event{
                EntityID:        "entity_001",
                ProviderEventID: "evt_123",
                CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
            },
            duplicateEvent: Event{
                EntityID:        "entity_001",
                ProviderEventID: "evt_123", // Same as first
                CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Same for partitioned tables
            },
            expectDuplicateError: true,
        },
        // Test that different combinations succeed...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create first event
            // Attempt duplicate
            // Assert error behavior
        })
    }
}
```

### Partitioned Table Gotchas

When testing UNIQUE constraints on partitioned tables:

```sql
-- ❌ This will FAIL on partitioned table:
CREATE UNIQUE INDEX idx ON partitioned_table(entity_id, provider_id);

-- ✅ Must include partition key:
CREATE UNIQUE INDEX idx ON partitioned_table(entity_id, provider_id, created_at);
```

In tests for partitioned tables, **use the SAME `created_at`** value for duplicate checks:

```go
// ❌ Wrong - different created_at means different partition, no violation
firstEvent.CreatedAt = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
duplicate.CreatedAt  = time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

// ✅ Correct - same created_at to test constraint
firstEvent.CreatedAt = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
duplicate.CreatedAt  = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
```

### Why This Matters

1. **Prevents silent failures** - Constraint might not work as expected
2. **Documents intent** - Tests show what the constraint protects against
3. **Catches regressions** - Future changes won't break critical invariants
4. **Validates error handling** - Application correctly handles DB errors

### Enforcement

Before marking migration task as complete:
- [ ] Integration test exists
- [ ] Test covers constraint violations
- [ ] Test verifies correct error type returned
- [ ] Test passes with `make integration-test`
- [ ] For partitioned tables: partition key included in constraint and test

## Migration Best Practices

### Naming Convention
```
YYYYMMDDHHMMSS_descriptive_name.sql
```

### Structure
```sql
-- +goose Up
-- +goose StatementBegin

-- Add detailed comment explaining the change
CREATE INDEX ...;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Rollback must be safe and tested
DROP INDEX ...;

-- +goose StatementEnd
```

### Safety Checks

Before creating migration:
1. **Check current schema** - Read existing migration files to understand structure
2. **Consider partitioning** - Does table use pg_partman? Constraints must include partition key
3. **Plan rollback** - Can this be safely rolled back? Add to `-- +goose Down`
4. **Check for existing data** - Will constraint fail on existing records?

### Running Locally

```bash
# Migrations run automatically on app startup
make run-dev

# Or manually via goose (for testing)
cd internal/app/migrations
goose postgres "connection-string" up
```
