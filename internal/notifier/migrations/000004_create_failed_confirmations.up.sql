CREATE TABLE IF NOT EXISTS failed_confirmations
(
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    saga_id     TEXT        NOT NULL,
    email       TEXT        NOT NULL,
    repo_name   TEXT        NOT NULL,
    confirm_url TEXT        NOT NULL,
    reason      TEXT        NOT NULL,
    retry_count INT         NOT NULL DEFAULT 0,
    failed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT failed_confirmations_email_repo_key UNIQUE (email, repo_name)
);

CREATE INDEX IF NOT EXISTS failed_confirmations_failed_at_idx ON failed_confirmations (failed_at);