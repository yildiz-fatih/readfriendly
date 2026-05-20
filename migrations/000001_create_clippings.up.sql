CREATE TYPE task_status AS ENUM ('pending', 'completed', 'failed');

CREATE TABLE IF NOT EXISTS clippings (
    id TEXT PRIMARY KEY,
    status task_status NOT NULL DEFAULT 'pending',
    format TEXT NOT NULL,
    created TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
