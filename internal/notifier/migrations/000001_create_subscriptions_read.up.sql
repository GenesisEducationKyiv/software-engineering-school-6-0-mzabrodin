CREATE TABLE IF NOT EXISTS subscriptions_read
(
    email       TEXT NOT NULL,
    repo_name   TEXT NOT NULL,
    unsub_token TEXT NOT NULL,

    PRIMARY KEY (email, repo_name)
);

CREATE INDEX IF NOT EXISTS subscriptions_read_repo_name_idx ON subscriptions_read (repo_name);