-- +goose Up
-- +goose StatementBegin

-- Publication for CDC: tells PostgreSQL to include `events` table
-- changes in the logical replication stream (pgoutput protocol).
-- The CDC worker will create its own replication slot on startup.
CREATE PUBLICATION events_pub FOR TABLE events;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP PUBLICATION IF EXISTS events_pub;

-- +goose StatementEnd
