# GitHub Release Notifier

A service that monitors GitHub repositories for new releases and delivers email notifications to subscribers.

## Features

- Subscribe an email to any public GitHub repository
- Email confirmation flows before notifications begin
- Background scanner detects new releases and dispatches batch emails
- Redis caching of GitHub API responses (10-minute TTL)
- Prometheus metrics at `/metrics`
- API key authentication for writing endpoints
- GitHub Actions CI - lint and race-detected tests on every push

## Architecture

The service is a single Go monolith with four independently testable parts.

**Layer responsibilities:**

| Layer         | Package               | Responsibility                           |
|---------------|-----------------------|------------------------------------------|
| Domain        | `internal/domain`     | Entities, errors - no dependencies       |
| Repository    | `internal/repository` | SQL queries, error mapping               |
| Service       | `internal/service`    | Business rules, token generation         |
| GitHub client | `internal/github`     | REST calls, rate-limit handling, caching |
| Cache         | `internal/cache`      | Redis interface + implementation         |
| Mailer        | `internal/mailer`     | SMTP delivery, HTML templates            |
| Scanner       | `internal/scanner`    | Background polling goroutine             |
| API           | `internal/api`        | HTTP routing, request validation, auth   |
| Metrics       | `internal/metrics`    | Prometheus metric registration           |

## Subscription flow

**Subscribe** (`POST /api/subscribe`):
- Validate the repository name format - `400` if invalid
- Verify the repository exists via GitHub API - `404` if not found
- Create or reuse the repository record in the database
- Create the subscription with cryptographically random confirm and unsubscribe tokens - `409` if already subscribed
- Send a confirmation email in a background goroutine (non-blocking)

**Confirm** (`GET /api/confirm/{token}`):
- Mark the subscription as confirmed – the scanner will start notifying this email

**Unsubscribe** (`GET /api/unsubscribe/{token}`):
- Delete the subscription record – the unsubscribe link is included in every release email

**List subscriptions** (`GET /api/subscriptions?email=`):
- Return all subscriptions for the given email – both confirmed and pending, including confirmation status and last seen release tag

Confirmation and unsubscribe endpoints are intentionally **public** (no API key) because they are opened directly from email links in a browser, which cannot set custom headers.

## Scanner

A goroutine starts alongside the HTTP server and runs on a configurable interval (`SCAN_INTERVAL`, default `10m`):

1. Fetch all repositories that have at least one confirmed subscription
2. For each repository:
    - Call `GetLatestRelease` (served from Redis cache when warm)
    - Skip if `tag == last_seen_tag` - no new release
    - Fetch all confirmed subscribers for this repository
    - Build notification objects (release URL + per-subscriber unsubscribe URL)
    - Send batch emails over a single reused SMTP connection
    - Update `last_seen_tag` in the database
3. Record metrics: scan duration, notifications sent, GitHub API error types

Errors on individual repositories are logged and skipped – a broken repository never stops the scan of others.

## GitHub API caching

The `Client` struct carries an optional `cache.Cache`. Attach it with:

```go
github.NewClient(token).WithCache(redisCache, 10*time.Minute)
```

Cache entries:

| Situation      | Key                                | Value           |
|----------------|------------------------------------|-----------------|
| Repo exists    | `github:repo_exists:owner/repo`    | `"1"`           |
| Repo not found | `github:repo_exists:owner/repo`    | `"0"`           |
| Release found  | `github:latest_release:owner/repo` | JSON of release |
| No releases    | `github:latest_release:owner/repo` | `"none"`        |

Caching the `"none"` sentinel avoids repeated GitHub API calls for repositories that have no releases yet, which would
otherwise consume rate-limit quota on every scan. The TTL (10 min) means a newly published first release is picked up
within one scan cycle – the same delay as any other release.

On any Redis error, the client falls through to the GitHub API and logs a warning. The service continues operating
correctly without the cache.

## Rate limiting

GitHub allows 60 unauthenticated requests/hour and 5 000 with a token. The client handles both rate-limit signals:

- HTTP `429 Too Many Requests` - always rate limited
- HTTP `403 Forbidden` + `X-RateLimit-Remaining: 0` - secondary rate limit

The `Retry-After` response header is parsed (seconds) and included in the returned error. The scanner logs the error and
skips the current cycle, it does not crash or block.

## API

All endpoints return `application/json`. Protected endpoints require `X-API-Key` header.

| Method | Path                        | Auth    | Description                     |
|--------|-----------------------------|---------|---------------------------------|
| `POST` | `/api/subscribe`            | API key | Subscribe email to a repository |
| `GET`  | `/api/confirm/{token}`      | -       | Confirm subscription            |
| `GET`  | `/api/unsubscribe/{token}`  | -       | Unsubscribe                     |
| `GET`  | `/api/subscriptions?email=` | API key | List subscriptions for an email |
| `GET`  | `/health`                   | -       | Health check                    |
| `GET`  | `/metrics`                  | -       | Prometheus metrics              |

## Metrics

| Metric                          | Type      | Description                                              |
|---------------------------------|-----------|----------------------------------------------------------|
| `http_requests_total`           | Counter   | Requests by method, path, status code                    |
| `http_request_duration_seconds` | Histogram | Latency by method, path                                  |
| `scanner_runs_total`            | Counter   | Completed scan cycles                                    |
| `scanner_duration_seconds`      | Histogram | Time per scan cycle                                      |
| `notifications_sent_total`      | Counter   | Emails dispatched                                        |
| `github_api_errors_total`       | Counter   | API errors by type (rate_limited / unauthorized / other) |

## Running locally

### With Docker Compose

```bash
cp .env.example .env
# Fill in GITHUB_TOKEN, SMTP_*, REDIS_PASSWORD, DB_PASSWORD, API_KEY

docker compose up --build
```

The app, PostgreSQL, and Redis start in dependency order. Database migrations run automatically on startup.

### Without Docker

```bash
cp .env.example .env
# Start PostgreSQL and Redis separately, update .env accordingly

go run ./cmd/server
```

## Environment variables

| Variable         | Default                   | Description                                                            |
|------------------|---------------------------|------------------------------------------------------------------------|
| `PORT`           | `8080`                    | HTTP listen port                                                       |
| `BASE_URL`       | `http://localhost:8080`   | Used in confirmation/unsubscribe links                                 |
| `GITHUB_TOKEN`   | -                         | GitHub personal access token (optional, raises rate limit to 5 000/hr) |
| `SCAN_INTERVAL`  | `10m`                     | How often the scanner checks for new releases                          |
| `DB_HOST`        | `localhost`               | PostgreSQL host                                                        |
| `DB_PORT`        | `5432`                    | PostgreSQL port                                                        |
| `DB_USER`        | `postgres`                | PostgreSQL user                                                        |
| `DB_PASSWORD`    | -                         | PostgreSQL password                                                    |
| `DB_NAME`        | `github_release_notifier` | PostgreSQL database name                                               |
| `DB_SSL_MODE`    | `disable`                 | PostgreSQL SSL mode                                                    |
| `REDIS_HOST`     | `localhost`               | Redis host                                                             |
| `REDIS_PORT`     | `6379`                    | Redis port                                                             |
| `REDIS_PASSWORD` | -                         | Redis password                                                         |
| `SMTP_HOST`      | -                         | SMTP server host                                                       |
| `SMTP_PORT`      | `587`                     | SMTP server port                                                       |
| `SMTP_USER`      | -                         | SMTP username                                                          |
| `SMTP_PASSWORD`  | -                         | SMTP password                                                          |
| `SMTP_FROM`      | -                         | From address for outgoing emails                                       |
| `API_KEY`        | -                         | API key for protected endpoints (disabled if empty)                    |

## Database schema

```sql
CREATE TABLE repositories
(
    id            UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL UNIQUE, -- "owner/repo"
    last_seen_tag TEXT,                        -- NULL until first release detected
    checked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE subscriptions
(
    id                UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    repository_id     UUID        NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    email             TEXT        NOT NULL,
    confirm_token     TEXT        NOT NULL UNIQUE,
    unsubscribe_token TEXT        NOT NULL UNIQUE,
    confirmed         BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (email, repository_id)
);

CREATE INDEX idx_subscriptions_email ON subscriptions (email);
```

The `last_seen_tag` on `repositories` is shared across all subscribers. This means a single GitHub API call per
repository per scan cycle, regardless of how many subscribers it has.

## Tests

```bash
go test ./...
```

Unit tests cover all packages with business logic: service, scanner, GitHub client (including cache behavior), mailer, and API middleware. All external dependencies (database, Redis, GitHub API, SMTP) are replaced with focused interface mocks. The test suite runs with `-race` in CI.