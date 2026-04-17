-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS partman;
CREATE EXTENSION IF NOT EXISTS pg_partman WITH SCHEMA partman;

ALTER TABLE IF EXISTS public.dispute_events RENAME TO dispute_events_unpart;

CREATE TABLE IF NOT EXISTS public.dispute_events (
    id                 VARCHAR(255) NOT NULL,
    dispute_id         VARCHAR(255) NOT NULL,
    kind               VARCHAR(32)  NOT NULL,
    provider_event_id  VARCHAR(255) NOT NULL,
    data               JSONB        NOT NULL,
    created_at         TIMESTAMP    NOT NULL,
    CONSTRAINT fk_dispute_event_dispute
    FOREIGN KEY (dispute_id) REFERENCES public.disputes(id),
    CONSTRAINT dispute_events_pk PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

SELECT partman.create_parent(
       p_parent_table           => 'public.dispute_events',
       p_control                => 'created_at',
       p_interval               => '1 day',
       p_premake                => 7,
       p_default_table          => true,
       p_automatic_maintenance  => 'on',
       p_start_partition        => to_char(
               date_trunc('day', COALESCE((SELECT min(created_at) FROM public.dispute_events_unpart), now())),
               'YYYY-MM-DD'
                                   )
);

-- Pre-create upcoming partitions
SELECT partman.run_maintenance();

CREATE INDEX IF NOT EXISTS de_kind_created_at
    ON public.dispute_events (kind, created_at);

INSERT INTO public.dispute_events (id, dispute_id, kind, provider_event_id, data, created_at)
SELECT id, dispute_id, kind, provider_event_id, data, created_at
FROM   public.dispute_events_unpart;

CALL partman.partition_data_proc('public.dispute_events');

ANALYZE public.dispute_events;

DROP TABLE IF EXISTS public.dispute_events_unpart;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS public.dispute_events_unpart (
    id                 VARCHAR(255) PRIMARY KEY,
    dispute_id         VARCHAR(255) NOT NULL,
    kind               VARCHAR(32)  NOT NULL,
    provider_event_id  VARCHAR(255) NOT NULL,
    data               JSONB        NOT NULL,
    created_at         TIMESTAMP    NOT NULL,
    CONSTRAINT fk_dispute_event_dispute
    FOREIGN KEY (dispute_id) REFERENCES public.disputes(id)
    );

INSERT INTO public.dispute_events_unpart (id, dispute_id, kind, provider_event_id, data, created_at)
SELECT id, dispute_id, kind, provider_event_id, data, created_at
FROM   public.dispute_events;

CREATE INDEX IF NOT EXISTS de_kind_created_at_inc_dispute
    ON public.dispute_events_unpart (kind, created_at);

ANALYZE public.dispute_events_unpart;

DROP TABLE IF EXISTS public.dispute_events CASCADE;

ALTER TABLE IF EXISTS public.dispute_events_unpart RENAME TO dispute_events;

-- +goose StatementEnd
