CREATE TABLE IF NOT EXISTS subscriptions
(
    id                UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    repository_id     UUID        NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    email             TEXT        NOT NULL,
    confirm_token     TEXT        NOT NULL UNIQUE,
    unsubscribe_token TEXT        NOT NULL UNIQUE,
    confirmed         BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT subscriptions_email_repository_id_key UNIQUE (email, repository_id)
);