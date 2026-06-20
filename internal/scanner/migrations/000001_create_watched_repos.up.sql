CREATE TABLE IF NOT EXISTS watched_repos
(
    repo_name        TEXT    NOT NULL,
    last_seen_tag    TEXT,
    subscriber_count INTEGER NOT NULL DEFAULT 0 CHECK (subscriber_count >= 0),

    PRIMARY KEY (repo_name)
);