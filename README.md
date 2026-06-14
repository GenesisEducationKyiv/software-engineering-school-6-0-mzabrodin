# GitHub Release Notifier

A service that monitors GitHub repositories for new releases and delivers email notifications to subscribers.

## Features

- Subscribe an email to any public GitHub repository
- Email confirmation flows before notifications begin
- Background scanner detects new releases and dispatches batch emails
- One port (`:8080`) serving REST, Connect, gRPC, and gRPC-Web from a single connect-go handler (REST transcoded by
  Vanguard)
- Redis caching of GitHub API responses (10-minute TTL)
- API key authentication for write/read endpoints
- Structured JSON logging (`slog`)
- Observability stack: Prometheus + Grafana metrics, Filebeat → Elasticsearch → Kibana logs
- GitHub Actions CI — lint, race-detected unit tests, and integration tests on every push

## Architecture

Two Go binaries organized in hexagonal layers. Dependencies point inward:
inbound adapter → use case → outbound adapter → domain. A single connect-go
handler serves the public API. [Vanguard](https://github.com/connectrpc/vanguard-go)
transcode the REST URLs onto it, so REST, Connect, gRPC, and gRPC-Web share one port and one set of use
cases ([ADR-011](docs/adr/011-connect-transcoding.md)).
SMTP delivery lives in a separate emailer microservice
([ADR-009](docs/adr/009-microservices-split.md)) that the app reaches over gRPC
secured with mTLS.

A modulith ([ADR-010](docs/adr/010-modulith-bounded-contexts.md)): `internal/` is sliced vertically
by bounded context, with clean-arch layers *inside* each module. Modules stay independent (no module
imports another — they talk through consumer-defined ports wired in `cmd/` and `internal/bootstrap`),
enforced in CI by golangci-lint `depguard`.

| Module         | Package                                                                                           | Responsibility                                                                                                                                                                                           |
|----------------|---------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `subscription` | `internal/subscription/{usecase/*,adapter/{connectrpc,repository}}`                               | Public API (`subscription.v1`) via one connect-go handler & Vanguard; owns the repos & subscriptions tables; drives the scan (`scan` use case) and owns the notify decision; dials the scanner + emailer |
| `scanner`      | `internal/scanner/{usecase/scanner,adapter/{scannerserver,scannerclient}}`                        | A reactive GitHub-fetch service (the `cmd/scanner` service lives behind this); `Scan(repos)` over mTLS; owns its proto + server + client, like the notifier                               |
| `notifier`     | `internal/notifier/adapter/{notifierclient,notifierserver,mailer}`                                | Email delivery (the `cmd/emailer` service lives behind this); the app dials it through `notifierclient`                                                                                                  |
| Shared kernel  | `internal/shared/{entity,github}`                                                                 | Cross-context domain types + constructors + sentinel errors; GitHub REST client                                                                                                                          |
| Infrastructure | `internal/infrastructure/{config,db,cache,urlbuilder,logging,metrics,tlsconfig,certgen}`          | Env config, pgx pool + migrations, Redis, URL building, slog, Prometheus, mTLS config + cert gen                                                                                                         |
| Composition    | `internal/bootstrap/{subscription,scanner,emailer}`, `cmd/{subscription,scanner,emailer}/main.go` | Cross-module wiring lives in `internal/bootstrap`; the `cmd/` mains just load config and call `Run`                                                                                                      |

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

All four operations are defined once in `proto/subscription/v1/subscription.proto` and served by a single connect-go
handler on `PORT` (default `8080`).

### REST (transcode by Vanguard)

All endpoints return `application/json`. Protected endpoints require an `Authorization: Bearer <api-key>` header.

| Method | Path                        | Auth    | Description                     |
|--------|-----------------------------|---------|---------------------------------|
| `POST` | `/api/subscribe`            | API key | Subscribe email to a repository |
| `GET`  | `/api/confirm/{token}`      | —       | Confirm subscription            |
| `GET`  | `/api/unsubscribe/{token}`  | —       | Unsubscribe                     |
| `GET`  | `/api/subscriptions?email=` | API key | List subscriptions for an email |
| `GET`  | `/health`                   | —       | Health check                    |
| `GET`  | `/metrics`                  | —       | Prometheus metrics              |

### gRPC / Connect / gRPC-Web (same `PORT`)

Service `subscription.v1.SubscriptionService` — contract in `proto/subscription/v1/subscription.proto`. The same handler answers
the Connect, gRPC, and gRPC-Web protocols over h2c on the same port.

| RPC                 | Auth                    | Description                     |
|---------------------|-------------------------|---------------------------------|
| `Subscribe`         | `Authorization: Bearer` | Subscribe email to a repository |
| `Confirm`           | —                       | Confirm subscription by token   |
| `Unsubscribe`       | —                       | Unsubscribe by token            |
| `ListSubscriptions` | `Authorization: Bearer` | List subscriptions for an email |

- **Server reflection** and the **gRPC health service** are registered, so clients can introspect and health check
  without local protos.
- Request validation is declarative via [protovalidate](https://github.com/bufbuild/protovalidate) rules embedded in the
  proto.
- Generated stubs live inside each module under `grpc/gen/` (gitignored) — run `task proto` to regenerate.

Domain errors map to connect codes (`internal/subscription/adapter/connectrpc/errors.go`); Vanguard turns them into the
matching HTTP status for REST callers:

| Condition                  | HTTP | gRPC code         |
|----------------------------|------|-------------------|
| Invalid input / validation | 400  | `InvalidArgument` |
| Missing / wrong API key    | 401  | `Unauthenticated` |
| Not found                  | 404  | `NotFound`        |
| Already subscribed         | 409  | `AlreadyExists`   |
| Unexpected error           | 500  | `Internal`        |

## Scanner

The scanner (`cmd/scanner`) is a separate binary — a reactive GitHub-fetch service the app calls over
mTLS (`scanner.v1.ScannerService`). It makes no decision and never touches the database. The app drives
the scan on a configurable interval (`SCAN_INTERVAL`, default `10m`):

1. The app lists its repositories with ≥1 confirmed subscriber (Postgres)
2. The app calls `Scan(names)`; the scanner, with a pool of `SCAN_WORKERS` workers, calls
   `GetLatestRelease` per repo (Redis-cached) and returns the observed `{repo, tag, release_url}` batch
   (repos with no release are omitted; per-repo errors are isolated; a rate-limit/auth error aborts the pass)
3. The app decides per repo: if `last_seen_tag` is NULL it seeds silently (the confirmation flow owns the current
   release); if the tag changed it emails the confirmed subscribers via the emailer and advances
   `last_seen_tag` only on a successful sending

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

The Grafana and Kibana dashboards cover the public API traffic by protocol (rate, errors, latency, status codes) plus
the
scanner, database, cache, email, and GitHub API layers.

## Metrics

Exposed at `/metrics`.

| Metric                                | Type      | Labels                    | Description                                                                                                      |
|---------------------------------------|-----------|---------------------------|------------------------------------------------------------------------------------------------------------------|
| `requests_total`                      | Counter   | protocol, procedure, code | Every RPC across both services (protocol = rest/connect/grpc/grpc-web); app vs emailer split by Prometheus `job` |
| `request_duration_seconds`            | Histogram | protocol, procedure       | Request latency for both services                                                                                |
| `subscription_operations_total`       | Counter   | operation, result         | Subscribe / confirm / unsubscribe / list outcomes                                                                |
| `scanner_runs_total`                  | Counter   | —                         | Completed scan cycles                                                                                            |
| `scanner_duration_seconds`            | Histogram | —                         | Time per scan cycle                                                                                              |
| `scanner_errors_total`                | Counter   | reason                    | Scan errors (list_repos / parse_repo / fetch_release / report_releases / process_release)                        |
| `notifications_sent_total`            | Counter   | —                         | Release emails dispatched                                                                                        |
| `db_queries_total`                    | Counter   | operation, table          | Database queries                                                                                                 |
| `db_query_errors_total`               | Counter   | operation, table          | Database infrastructure errors                                                                                   |
| `db_query_duration_seconds`           | Histogram | operation, table          | Database query latency                                                                                           |
| `cache_operations_total`              | Counter   | operation, result         | Redis operations (hit / miss / error / success)                                                                  |
| `cache_operation_duration_seconds`    | Histogram | operation                 | Redis operation latency                                                                                          |
| `email_sends_total`                   | Counter   | type, status              | SMTP sends by email type                                                                                         |
| `email_send_duration_seconds`         | Histogram | type                      | SMTP send latency                                                                                                |
| `github_api_requests_total`           | Counter   | operation, result         | GitHub API requests                                                                                              |
| `github_api_request_duration_seconds` | Histogram | operation                 | GitHub API latency                                                                                               |
| `github_api_errors_total`             | Counter   | type                      | GitHub API errors (rate_limited / unauthorized / other)                                                          |

## Running locally

### With Docker Compose

```bash
cp .env.example .env
# Fill in GITHUB_TOKEN, SMTP_*, DB_*, REDIS_PASSWORD, GRAFANA_PASSWORD, API_KEY

task certs                     # generate the mTLS certs (subscription/scanner/emailer) into certs/ (prerequisite)
task up-build                  # build the images, then start the whole stack
```

The app, the **emailer** service, PostgreSQL, Redis, and the observability stack start
in dependency order. In Docker, the app's `DATABASE_URL` and `REDIS_URL` are constructed
automatically from the `DB_*` / `REDIS_*` values. Database migrations run on startup.
The emailer mounts the **server** cert, and the app mounts the **client** cert from `certs/`;
both verify the peer against the shared CA, so `task certs` must run first.

The public API (REST, Connect, gRPC, gRPC-Web) is served on `:8080`; the emailer listens on `:50052` (gRPC, internal
only).

### Regenerating stubs and API docs

```bash
task proto      # requires the buf CLI; regenerates each module's grpc/gen/ stubs
```

## Environment variables

### Subscription app (`cmd/subscription`)

| Variable        | Default                 | Required | Description                                                      |
|-----------------|-------------------------|----------|------------------------------------------------------------------|
| `PORT`          | `8080`                  | —        | Single public port (REST + Connect + gRPC + gRPC-Web)            |
| `BASE_URL`      | `http://localhost:8080` | —        | Used in confirmation/unsubscribe links                           |
| `GITHUB_TOKEN`  | —                       | —        | GitHub PAT (optional; raises rate limit to 5 000/hr)             |
| `SCANNER_ADDR`  | `localhost:50051`       | —        | Address of the scanner gRPC service (host must match a cert SAN) |
| `EMAILER_ADDR`  | `localhost:50052`       | —        | Address of the emailer gRPC service (host must match a cert SAN) |
| `SCAN_INTERVAL` | `10m`                   | —        | How often the app runs a scan pass                               |
| `DATABASE_URL`  | —                       | yes      | PostgreSQL connection URL                                        |
| `REDIS_URL`     | —                       | yes      | Redis connection URL                                             |
| `TLS_CERT_FILE` | —                       | yes      | Path to the app's **client** certificate (mTLS; dials out only)  |
| `TLS_KEY_FILE`  | —                       | yes      | Path to the app's client key                                     |
| `TLS_CA_FILE`   | —                       | yes      | Path to the shared CA certificate                                |
| `API_KEY`       | —                       | —        | Key for protected endpoints (auth disabled if empty)             |
| `LOG_LEVEL`     | `info`                  | —        | `debug` / `info` / `warn` / `error`                              |

### Scanner (`cmd/scanner`)

| Variable            | Default | Required | Description                                                 |
|---------------------|---------|----------|-------------------------------------------------------------|
| `SCANNER_GRPC_PORT` | `50051` | —        | mTLS gRPC port serving `ScannerService` (dialed by the app) |
| `SCANNER_HTTP_PORT` | `8082`  | —        | Scanner metrics + health HTTP port                          |
| `SCAN_WORKERS`      | `5`     | —        | Concurrent GitHub fetches per pass                          |
| `GITHUB_TOKEN`      | —       | —        | GitHub PAT (optional; raises rate limit to 5 000/hr)        |
| `REDIS_URL`         | —       | yes      | Redis connection URL (GitHub response cache)                |
| `TLS_CERT_FILE`     | —       | yes      | Path to the scanner's **server** certificate (mTLS)         |
| `TLS_KEY_FILE`      | —       | yes      | Path to the scanner's server key                            |
| `TLS_CA_FILE`       | —       | yes      | Path to the shared CA certificate                           |
| `LOG_LEVEL`         | `info`  | —        | `debug` / `info` / `warn` / `error`                         |

### Emailer (`cmd/emailer`)

| Variable            | Default | Required | Description                                         |
|---------------------|---------|----------|-----------------------------------------------------|
| `EMAILER_GRPC_PORT` | `50052` | —        | Emailer gRPC listen port (mTLS)                     |
| `EMAILER_HTTP_PORT` | `8081`  | —        | Emailer metrics + health HTTP port                  |
| `SMTP_HOST`         | —       | yes      | SMTP server host                                    |
| `SMTP_PORT`         | `587`   | —        | SMTP server port                                    |
| `SMTP_USER`         | —       | yes      | SMTP username                                       |
| `SMTP_PASSWORD`     | —       | yes      | SMTP password                                       |
| `SMTP_FROM`         | —       | yes      | From address for outgoing emails                    |
| `TLS_CERT_FILE`     | —       | yes      | Path to the emailer's **server** certificate (mTLS) |
| `TLS_KEY_FILE`      | —       | yes      | Path to the emailer's server key                    |
| `TLS_CA_FILE`       | —       | yes      | Path to the shared CA certificate                   |
| `LOG_LEVEL`         | `info`  | —        | `debug` / `info` / `warn` / `error`                 |

The internal links (app ↔ scanner, app ↔ emailer) have no `API_KEY` — mutual TLS is the authentication.

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
with a proper secret manager. The `.env` file is listed in `.gitignore`; the committed `.env.example` (full stack) and
the per-service `.env.app.example` / `.env.emailer.example` contain only placeholder values and are safe to keep in the
repository.

## Tests

```bash
task test-unit          # unit tests (also: go test ./...)
task test-integration   # integration tests (requires Docker)
                        # or run `task test-all`
```

Unit tests cover all packages with business logic — use cases, scanner, GitHub client (including cache behavior),
mailer, and the connection adapter — with all external dependencies replaced by focused interface mocks. The
`internal/bootstrap/subscription` tests drive the full Vanguard handler over both REST and gRPC. The suite runs with `-race`
in CI.

Integration tests (under `test/integration`, behind the `integration` build tag) spin up real PostgreSQL and Redis via
[testcontainers](https://golang.testcontainers.org/) and exercise both the REST and gRPC transports end-to-end against
the unified handler and the real database.
