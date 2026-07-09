CREATE TABLE IF NOT EXISTS sagas
(
    id         UUID PRIMARY KEY,
    type       TEXT        NOT NULL,
    state      TEXT        NOT NULL,
    email      TEXT        NOT NULL,
    repo_name  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (type, email, repo_name)
);

CREATE INDEX IF NOT EXISTS sagas_state_idx ON sagas (state);