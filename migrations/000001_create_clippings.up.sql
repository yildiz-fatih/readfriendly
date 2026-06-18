CREATE TYPE clipping_status AS ENUM ('pending', 'completed', 'failed');
CREATE TYPE clipping_format AS ENUM ('pdf', 'epub', 'html');

CREATE TABLE IF NOT EXISTS clippings (
    id TEXT PRIMARY KEY,
    status clipping_status NOT NULL DEFAULT 'pending',
    format clipping_format NOT NULL,
    created TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
