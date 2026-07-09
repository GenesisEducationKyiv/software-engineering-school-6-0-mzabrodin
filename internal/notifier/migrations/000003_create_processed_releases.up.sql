CREATE TABLE IF NOT EXISTS processed_releases
(
    repo_name    TEXT        NOT NULL,
    tag          TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (repo_name, tag)
);