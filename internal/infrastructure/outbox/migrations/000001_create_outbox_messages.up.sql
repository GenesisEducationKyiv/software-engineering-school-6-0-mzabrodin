CREATE TABLE IF NOT EXISTS outbox_messages
(
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subject    TEXT        NOT NULL,
    payload    BYTEA       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
