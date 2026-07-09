CREATE TABLE IF NOT EXISTS failed_notifications
(
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    saga_id     TEXT        NOT NULL,
    repo_name   TEXT        NOT NULL,
    tag         TEXT        NOT NULL,
    release_url TEXT        NOT NULL,
    email       TEXT        NOT NULL,
    reason      TEXT        NOT NULL,
    retry_count INT         NOT NULL DEFAULT 0,
    failed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT failed_notifications_repo_tag_email_key UNIQUE (repo_name, tag, email)
);

CREATE INDEX IF NOT EXISTS failed_notifications_failed_at_idx ON failed_notifications (failed_at);