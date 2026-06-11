# GitHub Release Notifier

A service that monitors GitHub repositories for new releases and delivers email notifications to subscribers.

## Features

- Subscribe an email to any public GitHub repository
- Email confirmation flows before notifications begin
- Background scanner detects new releases and dispatches batch emails
- Two transports from one set of use cases: HTTP REST (`:8080`) and gRPC (`:50051`)
- Redis caching of GitHub API responses (10-minute TTL)
- API key authentication for write/read endpoints
- Structured JSON logging (`slog`)
- Observability stack: Prometheus + Grafana metrics, Filebeat → Elasticsearch → Kibana logs
- GitHub Actions CI — lint, race-detected unit tests, and integration tests on every push

## Architecture

Two Go binaries organized in hexagonal layers. Dependencies point inward:
inbound adapter → use case → outbound adapter → domain. The HTTP and gRPC adapters
call the **same** use cases, so business logic is transport-agnostic. SMTP delivery
lives in a separate **emailer** microservice ([ADR-009](docs/adr/009-future-microservices-split.md))
that the app reaches over gRPC secured with **mTLS**.

A **modulith**: `internal/` is sliced vertically by bounded context, with clean-arch layers *inside*
each module. Modules stay independent (no module imports another — they talk through consumer-defined
ports wired in `cmd/`), enforced in CI by golangci-lint `depguard`.

| Module         | Package                                                                              | Responsibility                                                                                               |
|----------------|--------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------|
| `subscription` | `internal/subscription/{usecase/*,adapter/{http,grpc,repository}}`                   | Public REST (chi) + gRPC (`app.v1`); subscribe/confirm/unsubscribe/list; owns the repos+subscriptions tables |
| `scanner`      | `internal/scanner/{usecase/scanner,scheduler}`                                       | Detect new releases on a ticker; notify confirmed subscribers                                                |
| `notifier`     | `internal/notifier/{adapter/{emailerclient,emailerserver,mailer},tlsconfig,certgen}` | Email delivery (the `cmd/emailer` service lives behind this); mTLS for the app↔emailer link                  |
| Shared kernel  | `internal/shared/{entity,github}`                                                    | Cross-context domain types + constructors + sentinel errors; GitHub REST client                              |
| Infrastructure | `internal/infrastructure/{config,db,cache,urlbuilder,logging,metrics}`               | Env config, pgx pool + migrations, Redis, URL building, slog, Prometheus                                     |
| Entrypoints    | `cmd/server/main.go`, `cmd/emailer/main.go`                                          | Composition roots: app (HTTP + gRPC + scanner) and the emailer service (SMTP + `emailer.v1` over mTLS)       |

## Subscription flow

The same four operations are exposed over both REST and gRPC.

**Subscribe**:

- Validate the repository name format — rejected if not `owner/repo`
- Verify the repository exists via the GitHub API — not-found if missing
- Create or reuse the repository record in the database
- Create the subscription with cryptographically random confirm and unsubscribe tokens — conflict if already subscribed
- Send a confirmation email in a background goroutine (non-blocking)

**Confirm**: mark the subscription as confirmed — the scanner will start notifying this email.

**Unsubscribe**: delete the subscription record — the unsubscribe link is included in every release email.

**List subscriptions**: return all subscriptions for an email — confirmed and pending, including confirmation status and
last seen release tag.

Confirm and unsubscribe are intentionally **public** (no API key) because they are opened directly from email links in a
browser, which cannot set custom headers.

## API

### HTTP REST (`HTTP_PORT`, default `8080`)

All endpoints return `application/json`. Protected endpoints require the `X-API-Key` header.

| Method | Path                        | Auth    | Description                     |
|--------|-----------------------------|---------|---------------------------------|
| `POST` | `/api/subscribe`            | API key | Subscribe email to a repository |
| `GET`  | `/api/confirm/{token}`      | —       | Confirm subscription            |
| `GET`  | `/api/unsubscribe/{token}`  | —       | Unsubscribe                     |
| `GET`  | `/api/subscriptions?email=` | API key | List subscriptions for an email |
| `GET`  | `/health`                   | —       | Health check                    |
| `GET`  | `/metrics`                  | —       | Prometheus metrics              |

### gRPC (`GRPC_PORT`, default `50051`)

Service `app.v1.SubscriptionService` — contract in `proto/app/v1/app.proto`.

| RPC                 | Auth                 | Description                     |
|---------------------|----------------------|---------------------------------|
| `Subscribe`         | `x-api-key` metadata | Subscribe email to a repository |
| `Confirm`           | —                    | Confirm subscription by token   |
| `Unsubscribe`       | —                    | Unsubscribe by token            |
| `ListSubscriptions` | `x-api-key` metadata | List subscriptions for an email |

- **Server reflection** and the **gRPC health service** are registered, so clients can introspect and health check
  without local protos.
- Request validation is declarative via [protovalidate](https://github.com/bufbuild/protovalidate) rules embedded in the
  proto.
- Generated stubs live inside each module under `grpc/gen/` (gitignored) — run `make proto` to regenerate.

Domain errors map to gRPC status codes (`internal/subscription/adapter/grpc/errors.go`), mirroring the HTTP status codes:

| Condition                  | HTTP | gRPC code         |
|----------------------------|------|-------------------|
| Invalid input / validation | 400  | `InvalidArgument` |
| Missing / wrong API key    | 401  | `Unauthenticated` |
| Not found                  | 404  | `NotFound`        |
| Already subscribed         | 409  | `AlreadyExists`   |
| Unexpected error           | 500  | `Internal`        |

## Scanner

A goroutine starts alongside the HTTP and gRPC servers and runs on a configurable interval (`SCAN_INTERVAL`, default
`10m`):

1. Fetch all repositories that have at least one confirmed subscription
2. For each repository (processed by a pool of `SCAN_WORKERS` workers):
    - Call `GetLatestRelease` (served from Redis cache when warm)
    - Skip if `tag == last_seen_tag` — no new release
    - Fetch all confirmed subscribers for this repository
    - Build notification objects (release URL + per-subscriber unsubscribe URL)
    - Send batch emails over a single reused SMTP connection
    - Update `last_seen_tag` in the database
3. Record metrics: scan duration, notifications sent, error reasons

Errors on individual repositories are logged and skipped — a broken repository never stops the scan of others.

## GitHub API caching

The `Client` carries an optional `cache.Cache`. Attach it with:

```go
github.NewClient(token, log).WithCache(redisCache, 10*time.Minute)
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
within one scan cycle — the same delay as any other release.

On any Redis error, the client falls through to the GitHub API and logs a warning. The service continues operating
correctly without the cache.

## Rate limiting

GitHub allows 60 unauthenticated requests/hour and 5 000 with a token. The client handles both rate-limit signals:

- HTTP `429 Too Many Requests` — always rate limited
- HTTP `403 Forbidden` + `X-RateLimit-Remaining: 0` — secondary rate limit

The `Retry-After` response header is parsed (seconds) and included in the returned error. The scanner logs the error and
skips the current cycle; it does not crash or block.

## Observability

Brought up by `docker compose` alongside the app:

| Component     | Service         | Default port | Purpose                                                 |
|---------------|-----------------|--------------|---------------------------------------------------------|
| Prometheus    | `prometheus`    | `9090`       | Scrapes `/metrics`                                      |
| Grafana       | `grafana`       | `3000`       | Dashboard auto-provisioned from `grafana/dashboards/`   |
| Elasticsearch | `elasticsearch` | —            | Log storage                                             |
| Kibana        | `kibana`        | `5601`       | Log dashboards imported from `kibana/dashboards.ndjson` |
| Filebeat      | `filebeat`      | —            | Ships app container logs to Elasticsearch               |

The Grafana and Kibana dashboards cover both HTTP and gRPC traffic (rate, errors, latency, status codes) plus the
scanner, database, cache, email, and GitHub API layers.

## Metrics

Exposed at `/metrics`.

| Metric                                | Type      | Labels                    | Description                                             |
|---------------------------------------|-----------|---------------------------|---------------------------------------------------------|
| `http_requests_total`                 | Counter   | method, path, status_code | HTTP requests                                           |
| `http_request_duration_seconds`       | Histogram | method, path              | HTTP latency                                            |
| `grpc_requests_total`                 | Counter   | method, code              | gRPC requests by full method and status code            |
| `grpc_request_duration_seconds`       | Histogram | method                    | gRPC latency                                            |
| `subscription_operations_total`       | Counter   | operation, result         | Subscribe / confirm / unsubscribe / list outcomes       |
| `scanner_runs_total`                  | Counter   | —                         | Completed scan cycles                                   |
| `scanner_duration_seconds`            | Histogram | —                         | Time per scan cycle                                     |
| `scanner_errors_total`                | Counter   | reason                    | Scanner errors (fetch_repos / check_repo)               |
| `notifications_sent_total`            | Counter   | —                         | Release emails dispatched                               |
| `db_queries_total`                    | Counter   | operation, table          | Database queries                                        |
| `db_query_errors_total`               | Counter   | operation, table          | Database infrastructure errors                          |
| `db_query_duration_seconds`           | Histogram | operation, table          | Database query latency                                  |
| `cache_operations_total`              | Counter   | operation, result         | Redis operations (hit / miss / error / success)         |
| `cache_operation_duration_seconds`    | Histogram | operation                 | Redis operation latency                                 |
| `email_sends_total`                   | Counter   | type, status              | SMTP sends by email type                                |
| `email_send_duration_seconds`         | Histogram | type                      | SMTP send latency                                       |
| `github_api_requests_total`           | Counter   | operation, result         | GitHub API requests                                     |
| `github_api_request_duration_seconds` | Histogram | operation                 | GitHub API latency                                      |
| `github_api_errors_total`             | Counter   | type                      | GitHub API errors (rate_limited / unauthorized / other) |

## Running locally

### With Docker Compose

```bash
cp .env.example .env
# Fill in GITHUB_TOKEN, SMTP_*, DB_*, REDIS_PASSWORD, GRAFANA_PASSWORD, API_KEY

make certs                     # generate the app↔emailer mTLS certs into certs/ (prerequisite)
docker compose up --build      # or: make up-build
```

The app, the **emailer** service, PostgreSQL, Redis, and the observability stack start
in dependency order. In Docker, the app's `DATABASE_URL` and `REDIS_URL` are constructed
automatically from the `DB_*` / `REDIS_*` values. Database migrations run on startup.
The emailer mounts the **server** cert and the app mounts the **client** cert from `certs/`;
both verify the peer against the shared CA, so `make certs` must run first.

HTTP is served on `:8080`, public gRPC on `:50051`; the emailer listens on `:50052` (gRPC, internal only).

### Regenerating gRPC stubs

```bash
make proto      # requires the buf CLI; regenerates each module's grpc/gen/
```

## Environment variables

### App (`cmd/server`)

| Variable              | Default                 | Required | Description                                                |
|-----------------------|-------------------------|----------|------------------------------------------------------------|
| `HTTP_PORT`           | `8080`                  | —        | HTTP listen port                                           |
| `GRPC_PORT`           | `50051`                 | —        | Public gRPC listen port                                    |
| `BASE_URL`            | `http://localhost:8080` | —        | Used in confirmation/unsubscribe links                     |
| `GITHUB_TOKEN`        | —                       | —        | GitHub PAT (optional; raises rate limit to 5 000/hr)       |
| `SCAN_INTERVAL`       | `10m`                   | —        | How often the scanner checks for new releases              |
| `SCAN_WORKERS`        | `5`                     | —        | Concurrent workers per scan cycle                          |
| `DATABASE_URL`        | —                       | **yes**  | PostgreSQL connection URL                                  |
| `REDIS_URL`           | —                       | **yes**  | Redis connection URL                                       |
| `EMAILER_ADDR`        | `localhost:50052`       | —        | Address of the emailer gRPC service                        |
| `EMAILER_SERVER_NAME` | `emailer`               | —        | Server name verified in the emailer's cert (a server SAN)  |
| `TLS_CERT_FILE`       | —                       | **yes**  | Path to the app's **client** certificate (mTLS)            |
| `TLS_KEY_FILE`        | —                       | **yes**  | Path to the app's client key                               |
| `TLS_CA_FILE`         | —                       | **yes**  | Path to the shared CA certificate                          |
| `API_KEY`             | —                       | —        | Key for protected endpoints (auth disabled if empty)       |
| `LOG_LEVEL`           | `info`                  | —        | `debug` / `info` / `warn` / `error`                        |

### Emailer (`cmd/emailer`)

| Variable            | Default | Required | Description                                          |
|---------------------|---------|----------|------------------------------------------------------|
| `EMAILER_GRPC_PORT` | `50052` | —        | Emailer gRPC listen port (mTLS)                      |
| `EMAILER_HTTP_PORT` | `8081`  | —        | Emailer metrics + health HTTP port                   |
| `SMTP_HOST`         | —       | **yes**  | SMTP server host                                     |
| `SMTP_PORT`         | `587`   | —        | SMTP server port                                     |
| `SMTP_USER`         | —       | **yes**  | SMTP username                                        |
| `SMTP_PASSWORD`     | —       | **yes**  | SMTP password                                        |
| `SMTP_FROM`         | —       | **yes**  | From address for outgoing emails                     |
| `TLS_CERT_FILE`     | —       | **yes**  | Path to the emailer's **server** certificate (mTLS)  |
| `TLS_KEY_FILE`      | —       | **yes**  | Path to the emailer's server key                     |
| `TLS_CA_FILE`       | —       | **yes**  | Path to the shared CA certificate                    |
| `LOG_LEVEL`         | `info`  | —        | `debug` / `info` / `warn` / `error`                  |

The internal app↔emailer link has **no `API_KEY`** — mutual TLS is the authentication.

### Docker Compose only (build URLs and configure the stack)

| Variable                              | Default                      | Description                                         |
|---------------------------------------|------------------------------|-----------------------------------------------------|
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | —                            | Postgres credentials; compose builds `DATABASE_URL` |
| `DB_PORT` / `DB_SSL_MODE`             | `5432` / `disable`           | Postgres port and SSL mode                          |
| `REDIS_PASSWORD` / `REDIS_PORT`       | — / `6379`                   | Redis credentials; compose builds `REDIS_URL`       |
| `ES_JAVA_OPTS` / `ES_PORT`            | `-Xms512m -Xmx512m` / `9200` | Elasticsearch heap and port                         |
| `KIBANA_PORT`                         | `5601`                       | Kibana UI port                                      |
| `PROMETHEUS_PORT`                     | `9090`                       | Prometheus UI port                                  |
| `GRAFANA_PORT` / `GRAFANA_PASSWORD`   | `3000` / `admin`             | Grafana UI port and admin password                  |

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

## Security notes

The `.env` file approach is convenient for development and evaluation but is not suitable for production — replace it
with a proper secret manager. The `.env` file is listed in `.gitignore`; the committed `.env.example` contains only
placeholder values and is safe to keep in the repository.

## Tests

```bash
make test               # unit tests (also: go test ./...)
make test-integration   # integration tests (requires Docker)
```

Unit tests cover all packages with business logic — use cases, scanner, GitHub client (including cache behavior),
mailer, and the HTTP/gRPC adapters — with all external dependencies replaced by focused interface mocks. The suite runs
with `-race` in CI.

Integration tests (under `test/integration`, behind the `integration` build tag) spin up real PostgreSQL and Redis via
[testcontainers](https://golang.testcontainers.org/) and exercise both the HTTP and gRPC transports end-to-end against
the real database.
