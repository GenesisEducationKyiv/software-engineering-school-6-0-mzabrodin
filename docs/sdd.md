# System Design

## System Requirements

### Functional Requirements

1. Subscription is created by providing an email and a repository in `owner/repo` format. The repository is verified
   against GitHub API before saving
2. New subscriptions are inactive until the user confirms ownership by clicking a one-time link sent to their email
3. Every release notification email contains a personal unsubscribe link
4. All subscriptions (active and pending) for an email can be listed via API
5. A background scanner checks all repositories with confirmed subscribers on a fixed interval and sends email
   notifications when a new release tag is detected
6. Each repository stores a single `last_seen_tag`. The scanner notifies only when a newer tag appears, not on every run
7. All four operations (subscribe, confirm, unsubscribe, list) are available over both an HTTP/JSON REST API and a gRPC
   API

### Non-Functional Requirements

1. The system is three Go binaries: the subscription app (`cmd/subscription`) serves the HTTP REST + gRPC API,
   owns Postgres, the scan schedule, and the notification decision (the `scan` use case); the scanner
   (`cmd/scanner`) is a reactive GitHub-fetch service; the emailer (`cmd/emailer`) owns SMTP and the mailer
   dispatch queue. The app dials both — both internal links (subscription → scanner, subscription → emailer) are gRPC
   secured with mTLS (see [ADR-009](adr/009-microservices-split.md))
2. Code is organized as a hexagonal architecture (`entity` / `usecase` / `adapter` / `infrastructure`) so business logic
   is independent of transport and providers; both transports reuse the same use cases (see [ADR-008](adr/008-hexagonal-architecture.md), [ADR-005](adr/005-grpc-api-alongside-rest.md))
3. GitHub API responses are cached in Redis with a 10-minute TTL to stay within rate limits (60 req/hour without a
   token, 5 000 with one)
4. PostgreSQL is used for persistence. Schema migrations run automatically on startup via `golang-migrate`
5. Write/read endpoints are optionally protected with a static API key sent as an `Authorization: Bearer <api-key>`
   header (the same scheme across REST, Connect, and gRPC)
6. gRPC request validation is declarative via protovalidate rules in the proto; server reflection and the gRPC health
   service are enabled
7. Observability: Prometheus metrics at `/metrics` (visualized in Grafana); structured JSON logs shipped by Filebeat to
   Elasticsearch and viewed in Kibana
8. The entire system — app, emailer, PostgreSQL, Redis, and the observability stack — starts with `docker compose up`
   (run `make certs` first to generate the app↔emailer mTLS material)
9. A GitHub Actions pipeline runs the linter, race-detected unit tests, and integration tests on every push

### Limitations

- Confirmation and release emails pass through an in-memory dispatch queue (see [ADR-006](adr/006-mailer-dispatch-queue.md)). Delivery is best-effort with no retries; if the process crashes,
  queued mail is lost
- A repo that just published its first release may not be detected for up to one full scanner interval (default 10
  minutes) plus the cache TTL
- GitHub `ETag` conditional requests are not implemented. Every cache refresh after TTL expiry counts against the
  rate-limit quota even if nothing changed
- Notifications are **at-most-once**: `last_seen_tag` is advanced after a batch in which at least one sending succeeded,
  so a subscriber whose individual sending failed is not retried for that release (see [ADR-007](adr/007-background-scanner.md)). If *every* send for a release fails, the tag is not advanced and the
  release is retried next cycle

---

## Architecture

```mermaid
flowchart LR
    User([Client])

    subgraph app[cmd/subscription]
        direction TB
        HTTP[HTTP API]
        GRPC[gRPC API]
        UC[Use Cases]
        Scheduler[Scheduler]
        ScanUC[Scan Use Case]
        ScannerClient[Scanner Client]
        Repo[Repository]
        GHClient[GitHub Client]
        NotifierClient[Notifier Client]
    end

    subgraph scanner[cmd/scanner]
        direction TB
        ScannerServer[ScannerService server]
        Scanner[Scanner]
        ScanGHClient[GitHub Client]
    end

    subgraph emailer[cmd/emailer]
        direction TB
        EmailerServer[gRPC Server]
        Mailer[Mailer queue]
    end

    subgraph storage[Storage]
        direction TB
        DB[(PostgreSQL)]
        Redis[(Redis)]
    end

    subgraph external[External]
        direction TB
        GHAPI([GitHub API])
        SMTP([SMTP Server])
    end

    subgraph obs[Observability]
        direction TB
        Prom[(Prometheus)] --> Graf[Grafana]
        ES[(Elasticsearch)] --> Kib[Kibana]
    end

    User --> HTTP & GRPC
    HTTP --> UC
    GRPC --> UC
    UC --> Repo & GHClient & NotifierClient
    Scheduler --> ScanUC
    ScanUC --> Repo & ScannerClient & NotifierClient
    ScannerClient -->|mTLS gRPC| ScannerServer --> Scanner --> ScanGHClient
    NotifierClient -->|mTLS gRPC| EmailerServer --> Mailer
    Repo --> DB
    GHClient --> Redis
    ScanGHClient --> Redis
    GHClient --> GHAPI
    ScanGHClient --> GHAPI
    Mailer --> SMTP
    app -.-> obs
    scanner -.-> obs
    emailer -.-> obs
```

---

## Key Flows

### Subscribe

Shown over HTTP; the gRPC `Subscribe` RPC follows the same path, mapping the
domain errors to `InvalidArgument` / `NotFound` / `AlreadyExists` instead of HTTP codes.

```mermaid
sequenceDiagram
    actor User
    participant API
    participant UC as Subscribe use case
    participant GH as GitHub Client
    participant DB as PostgreSQL
    participant EC as Notifier Client
    participant ES as Emailer Service
    User ->> API: POST /api/subscribe
    API ->> UC: Execute(email, repo)
    alt invalid format
        UC -->> API: ErrInvalidRepo
        API -->> User: 400 Bad Request
    else valid format
        UC ->> GH: RepoExists(owner, repo)
        GH -->> UC: true / false
        alt repo not found
            UC -->> API: ErrRepoNotFound
            API -->> User: 404 Not Found
        else repo exists
            UC ->> DB: GetByName(repo)
            alt not in DB
                UC ->> DB: CreateRepository(repo)
            end
            UC ->> DB: CreateSubscription(email, repo, tokens)
            alt already subscribed
                DB -->> UC: ErrAlreadyExists
                UC -->> API: ErrAlreadyExists
                API -->> User: 409 Conflict
            else
                DB -->> UC: ok
                UC -->> API: ok
                API -->> User: 200 OK
                UC -) EC: SendConfirmation(email, repo, confirmURL)
                EC -) ES: SendConfirmation(email, repo, confirmURL)
            end
        end
    end
```

### Scanner Execution

The app drives the scan and owns the decision (the `scan` use case); the scanner is a reactive service
that only reads GitHub and returns what it saw, fanning out over a bounded worker pool (`SCAN_WORKERS`). Per-repo
errors are isolated and omitted from the response; `ErrRateLimited` / `ErrUnauthorized` abort the pass.

```mermaid
sequenceDiagram
    participant Scheduler
    participant App as App
    participant DB as PostgreSQL
    participant Scanner as Scanner
    participant GH as GitHub Client
    participant EC as Notifier Client
    Scheduler ->> App: Run
    App ->> DB: repos with confirmed subs
    DB -->> App: repos[] (with last_seen_tag)
    App ->> Scanner: Scan(names)
    loop for each repo (worker pool)
        Scanner ->> GH: GetLatestRelease(owner, repo)
        GH -->> Scanner: Release / ErrNoRelease
    end
    Scanner -->> App: observed[]
    loop scan use case, for each observed repo
        alt last_seen_tag NULL
            App ->> DB: seed last_seen_tag (no email)
        else tag changed
            App ->> DB: GetConfirmedByRepoID(repoID)
            DB -->> App: subscribers[]
            App ->> EC: SendReleaseNotifications(batch)
            EC -->> App: BatchResult
            App ->> DB: UpdateLastSeenTag(repo, tag) on success
        end
    end
    App -->> Scanner: ok
```

### Email Dispatch

The app never speaks SMTP. Both email paths cross the **gRPC + mTLS** link into the
emailer service ([ADR-009](adr/009-microservices-split.md)), where they land on
the in-process dispatch queue ([ADR-006](adr/006-mailer-dispatch-queue.md)). Confirmation
sends are **fire-and-forget** (the app does not wait); release sends are synchronous and
return a `BatchResult`. A transport failure on the release path is reported as every
recipient failing, so the app does not advance `last_seen_tag` on a lost batch.

```mermaid
flowchart TD
    subgraph appbin[cmd/subscription]
        Sub[Subscribe]
        ScanUC[Scan Use Case]
        EC[Notifier Client]
    end

    subgraph emailerbin[cmd/emailer]
        ES[Emailer Server]
        Queue[[Dispatch queue]]
        Disp[Dispatcher goroutine]
    end

    SMTP([SMTP Server])
    Sub -- " SendConfirmation " --> EC
    ScanUC -- " SendReleaseNotifications " --> EC
    EC -- " gRPC " --> ES
    ES --> Queue
    Queue --> Disp
    Disp -- " SMTP " --> SMTP
```

The emailer server's `SendConfirmation` enqueues and returns immediately (an `OK` status
means *accepted*, not *delivered*). `SendReleaseNotifications` blocks until the batch is
sent and maps the `BatchResult` (sent count and failed recipients) back over the wire. On
shutdown the dispatcher drains the queue within a bounded context before the process exits.

---

## Component Design

### HTTP API (`internal/subscription/adapter/http`)

Chi router; maps domain errors to HTTP status codes; API-key middleware on protected routes.

| Method | Path                      | Protected | Description                     |
|--------|---------------------------|-----------|---------------------------------|
| POST   | /api/subscribe            | Yes       | Subscribe email to a repository |
| GET    | /api/confirm/{token}      | No        | Confirm subscription            |
| GET    | /api/unsubscribe/{token}  | No        | Unsubscribe                     |
| GET    | /api/subscriptions?email= | Yes       | List subscriptions for an email |
| GET    | /metrics                  | No        | Prometheus metrics              |
| GET    | /health                   | No        | Health check                    |

### gRPC API (`internal/subscription/adapter/grpc`)

Service `subscription.v1.SubscriptionService` — contract in `proto/subscription/v1/subscription.proto`, generated with
buf (`task proto`). The connection interceptor chain is observability → `Authorization: Bearer` auth → protovalidate
validation; server reflection and the gRPC health service are registered.

| RPC                 | Auth                    | Description                     |
|---------------------|-------------------------|---------------------------------|
| `Subscribe`         | `Authorization: Bearer` | Subscribe email to a repository |
| `Confirm`           | –                       | Confirm subscription by token   |
| `Unsubscribe`       | –                       | Unsubscribe by token            |
| `ListSubscriptions` | `Authorization: Bearer` | List subscriptions for an email |

Domain errors map to gRPC status codes, mirroring the HTTP table (see [ADR-005](adr/005-grpc-api-alongside-rest.md)):

| Domain error / condition                | gRPC code         |
|-----------------------------------------|-------------------|
| `ErrInvalidRepo`, protovalidate failure | `InvalidArgument` |
| `ErrRepoNotFound`, `ErrNotFound`        | `NotFound`        |
| `ErrAlreadyExists`                      | `AlreadyExists`   |
| missing / invalid API key               | `Unauthenticated` |
| any other error                         | `Internal`        |

### Use cases (`internal/subscription/usecase/*`)

Transport-agnostic business logic. Each exposes `Execute(ctx, In) (Out, error)`, declares its own narrow port
interfaces (implemented by adapters), and is optionally wrapped by `metrics.NewMetered`. Both transports share the same
instances.

| Use case      | Input       | Responsibility                                                                                                                         |
|---------------|-------------|----------------------------------------------------------------------------------------------------------------------------------------|
| `subscribe`   | email, repo | Validate `owner/repo`, check existence, persist with two random tokens, queue confirmation                                             |
| `confirm`     | token       | Mark the subscription confirmed; on a fresh confirm, deliver the repo's current release                                                |
| `unsubscribe` | token       | Delete the subscription                                                                                                                |
| `list`        | email       | Return all subscriptions for the email                                                                                                 |
| `scan`        | –           | List watched repos → fetch latest releases via the scanner → per repo seed NULL silently / skip unchanged / notify + advance on change |

### Repository Layer (`internal/subscription/adapter/repository`)

Two structs backed by `pgxpool.Pool`:

| Struct                   | Table           | Operations                                                           | Error mapping                                                                                       |
|--------------------------|-----------------|----------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| `GitHubRepoRepository`   | `repositories`  | get by name, create, list-with-subscriptions, update `last_seen_tag` | `pgx.ErrNoRows` → `entity.ErrNotFound`                                                              |
| `SubscriptionRepository` | `subscriptions` | create, confirm, list by email, get confirmed by repo, delete        | `pgx.ErrNoRows` → `entity.ErrNotFound`; unique `(email, repository_id)` → `entity.ErrAlreadyExists` |

### Scanner (`internal/scanner`) + Scheduler (`internal/infrastructure/scheduler`)

The scanner is a separate binary (`cmd/scanner`) — a reactive GitHub-fetch service. The app's `Scheduler` runs the
`scan` use case once immediately, then every `SCAN_INTERVAL`; the scanner only reads GitHub and returns what it saw,
while the app makes every decision (see [ADR-007](adr/007-background-scanner.md), [ADR-009](adr/009-microservices-split.md)):

| Concern          | Owner   | Behavior                                                                              |
|------------------|---------|---------------------------------------------------------------------------------------|
| Scheduling       | app     | Run immediately on startup, then every `SCAN_INTERVAL`                                |
| Work set         | app     | List repositories with ≥1 confirmed subscriber (Postgres), call the scanner           |
| Concurrency      | scanner | `errgroup` bounded to `SCAN_WORKERS`; return `{repo, tag, release_url}` batch         |
| Per-repo errors  | scanner | Logged + `scanner_errors_total`; omitted from the response, the pass continues        |
| Abort conditions | scanner | `ErrRateLimited` / `ErrUnauthorized` cancel the whole pass                            |
| Detection        | app     | Compare observed tag with `last_seen_tag`; NULL → seed silently; skip if unchanged    |
| Tag update       | app     | After a batch with ≥1 successful send (or immediately when a repo has no subscribers) |

### GitHub Client (`internal/shared/github`)

HTTP client wrapping the GitHub REST API; plugs into the Redis cache via `NewClient(token, log).WithCache(cache, ttl)`.
Returns typed sentinel errors so callers handle each case explicitly:

| Sentinel error    | Meaning                                          |
|-------------------|--------------------------------------------------|
| `ErrRateLimited`  | HTTP 429, or 403 with `X-RateLimit-Remaining: 0` |
| `ErrUnauthorized` | Invalid or missing token                         |
| `ErrNoRelease`    | Repository has no releases yet                   |

### Emailer service (`cmd/emailer`, `internal/notifier/adapter/{emailerserver,mailer}`)

SMTP delivery runs as a separate gRPC service ([ADR-009](adr/009-microservices-split.md)). `emailerserver`
exposes `notifier.v1.NotifierService` over mTLS and forwards to the `mailer`: a go-mail SMTP client behind an in-process
dispatch queue (see [ADR-006](adr/006-mailer-dispatch-queue.md)) — a single dispatcher goroutine consumes a buffered job
channel and dials one SMTP connection per batch. The app calls it through the `emailerclient` outbound adapter, which
satisfies the same `subscribe` and release-notifier ports the in-process mailer used to. `SendConfirmation` stays
fire-and-forget (the RPC runs on a background goroutine); `SendReleaseNotifications` is synchronous and reports a
transport failure as every recipient failing so the app (in the `scan` use case) does not advance `last_seen_tag`.
The table below is the `mailer` port the emailer server wraps:

| Method                     | Trigger     | Mode                                             | Returns              |
|----------------------------|-------------|--------------------------------------------------|----------------------|
| `SendConfirmation`         | Subscribe   | Async, fire-and-forget (`context.WithoutCancel`) | –                    |
| `SendReleaseNotifications` | scan        | Enqueue batch, block for the result              | `entity.BatchResult` |
| `Shutdown`                 | Server stop | Drain the queue within a bounded context         | –                    |

### Cache (`internal/infrastructure/cache`)

Redis-backed implementation of the `Cache` interface. On any Redis error the GitHub client logs it and falls through to
the real source — a Redis outage degrades performance, not availability.

| Method  | Behavior                                                                |
|---------|-------------------------------------------------------------------------|
| `Get`   | Returns `(value, found, err)`; a miss is `found == false` (`redis.Nil`) |
| `Set`   | Store a value with a TTL                                                |
| `Close` | Close the Redis client                                                  |

### Infrastructure (`internal/infrastructure/*`)

| Package     | Responsibility                                                            |
|-------------|---------------------------------------------------------------------------|
| `config`    | All settings from environment variables (below)                           |
| `db`        | pgx pool + `golang-migrate` migrations on startup                         |
| `logging`   | `slog` JSON handler, request-ID / scan-ID context, HTTP + gRPC middleware |
| `metrics`   | Prometheus registration + HTTP / gRPC / use-case / DB instrumentation     |
| `scheduler` | Periodic scan ticker                                                      |

### Metrics (`internal/infrastructure/metrics`)

Prometheus counters and histograms registered at package init:

| Metric                                                                                          | Type                          | Description                                                 |
|-------------------------------------------------------------------------------------------------|-------------------------------|-------------------------------------------------------------|
| `requests_total`                                                                                | Counter                       | Every RPC across both services by protocol, procedure, code |
| `request_duration_seconds`                                                                      | Histogram                     | Request latency by protocol and procedure                   |
| `subscription_operations_total`                                                                 | Counter                       | Use-case outcomes by operation and result                   |
| `scanner_runs_total`                                                                            | Counter                       | Completed scan cycles                                       |
| `scanner_duration_seconds`                                                                      | Histogram                     | Time per scan cycle                                         |
| `scanner_errors_total`                                                                          | Counter                       | Scanner errors by reason                                    |
| `notifications_sent_total`                                                                      | Counter                       | Release emails dispatched                                   |
| `db_queries_total` / `db_query_errors_total` / `db_query_duration_seconds`                      | Counter / Counter / Histogram | DB queries by operation and table                           |
| `cache_operations_total` / `cache_operation_duration_seconds`                                   | Counter / Histogram           | Redis operations and latency                                |
| `email_sends_total` / `email_send_duration_seconds`                                             | Counter / Histogram           | SMTP sends by type and status                               |
| `github_api_requests_total` / `github_api_request_duration_seconds` / `github_api_errors_total` | Counter / Histogram / Counter | GitHub API requests, latency, errors                        |

### Config (`internal/infrastructure/config`)

Configuration is read from environment variables via `envconfig`, split into `config.Load()` for the app,
`config.LoadScanner()` for the scanner, and `config.LoadEmailer()` for the emailer. The app consumes single
connection URLs (`DATABASE_URL`, `REDIS_URL`); under Docker Compose those are assembled from `DB_*` / `REDIS_*`
component variables. The `SMTP_*` variables live on the emailer; all three binaries read the `TLS_*` mTLS paths
(mounting the client / server / server cert respectively for subscription / scanner / emailer).

Subscription app (`cmd/subscription`):

| Variable                                         | Default                      | Required | Description                                                 |
|--------------------------------------------------|------------------------------|----------|-------------------------------------------------------------|
| `PORT`                                           | `8080`                       | –        | Single public port                                          |
| `BASE_URL`                                       | `http://localhost:8080`      | –        | Base for confirm/unsubscribe links                          |
| `GITHUB_TOKEN`                                   | –                            | –        | Raises GitHub rate limit to 5 000/hr                        |
| `SCANNER_ADDR` / `EMAILER_ADDR`                  | `localhost:50051` / `:50052` | –        | Scanner + emailer addresses (host must match each cert SAN) |
| `SCAN_INTERVAL`                                  | `10m`                        | –        | How often the app runs a scan pass                          |
| `DATABASE_URL` / `REDIS_URL`                     | –                            | yes      | PostgreSQL and Redis connection URLs                        |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` / `TLS_CA_FILE` | –                            | yes      | App **client** cert/key + shared CA (mTLS)                  |
| `API_KEY`                                        | –                            | –        | Protects write/read endpoints (off if empty)                |
| `LOG_LEVEL`                                      | `info`                       | –        | `debug` / `info` / `warn` / `error`                         |

Scanner (`cmd/scanner`):

| Variable                                         | Default | Required | Description                                    |
|--------------------------------------------------|---------|----------|------------------------------------------------|
| `SCANNER_GRPC_PORT`                              | `50051` | –        | mTLS `ScannerService` port (dialed by the app) |
| `SCANNER_HTTP_PORT`                              | `8082`  | –        | Metrics + health port                          |
| `SCAN_WORKERS`                                   | `5`     | –        | GitHub fetch concurrency per pass              |
| `GITHUB_TOKEN`                                   | –       | –        | Raises GitHub rate limit to 5 000/hr           |
| `REDIS_URL`                                      | –       | yes      | Redis connection URL (GitHub cache)            |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` / `TLS_CA_FILE` | –       | yes      | Scanner **server** cert/key + shared CA (mTLS) |
| `LOG_LEVEL`                                      | `info`  | –        | `debug` / `info` / `warn` / `error`            |

Emailer (`cmd/emailer`):

| Variable                                                  | Default          | Required | Description                               |
|-----------------------------------------------------------|------------------|----------|-------------------------------------------|
| `EMAILER_GRPC_PORT` / `EMAILER_HTTP_PORT`                 | `50052` / `8081` | –        | gRPC (mTLS) and metrics/health HTTP ports |
| `SMTP_HOST` / `SMTP_USER` / `SMTP_PASSWORD` / `SMTP_FROM` | –                | yes      | SMTP credentials and from address         |
| `SMTP_PORT`                                               | `587`            | –        | SMTP port                                 |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` / `TLS_CA_FILE`          | –                | yes      | Emailer **server** cert/key + shared CA   |
| `LOG_LEVEL`                                               | `info`           | –        | `debug` / `info` / `warn` / `error`       |

The Docker Compose–only variables (`DB_*`, `REDIS_*`, `ES_*`, `KIBANA_PORT`, `PROMETHEUS_PORT`, `GRAFANA_*`) are listed
in the README.
