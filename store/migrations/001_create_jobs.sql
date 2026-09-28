CREATE TABLE jobs (
    id            BIGSERIAL   PRIMARY KEY,
    queue         TEXT        NOT NULL DEFAULT 'default',
    kind          TEXT        NOT NULL,
    args          JSONB       NOT NULL DEFAULT '{}',
    state         TEXT        NOT NULL DEFAULT 'available'
                  CHECK (state IN ('available', 'running', 'completed', 'discarded')),
    priority      SMALLINT    NOT NULL DEFAULT 0,
    attempt       INT         NOT NULL DEFAULT 0,
    max_attempts  INT         NOT NULL DEFAULT 20 CHECK (max_attempts > 0),
    run_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_by     TEXT,
    lease_until   TIMESTAMPTZ,
    unique_key    TEXT,
    errors        JSONB       NOT NULL DEFAULT '[]',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finalized_at  TIMESTAMPTZ
);

-- The fetcher's query: next runnable jobs in a queue, highest priority first.
CREATE INDEX jobs_fetch_idx ON jobs (queue, priority DESC, run_at, id)
    WHERE state = 'available';

-- The rescuer's query: running jobs whose lease has expired.
CREATE INDEX jobs_lease_idx ON jobs (lease_until)
    WHERE state = 'running';

-- Deduplication: only one live (not yet finished) job per unique_key.
CREATE UNIQUE INDEX jobs_unique_key_idx ON jobs (unique_key)
    WHERE unique_key IS NOT NULL AND state IN ('available', 'running');
