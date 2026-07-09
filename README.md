# GitHub Release Notifier

A service that monitors GitHub repositories for new releases and delivers email notifications to subscribers.

> This README states the facts about the project. For design rationale and detail, see the
> [system design document](docs/SDD.md) and the [architecture decision records](docs/adr/).
>
> [NATS vs. gRPC comparison](docs/hw9-service-communication-grpc-and-buf-proto.md)

## What it does

- Subscribe an email to any public GitHub repository.
- Confirm the subscription via a link emailed to the subscriber (stateless JWT — no token stored).
- A scanner poll watched repositories and detects new releases.
- New releases are emailed to confirmed subscribers; every release email carries an unsubscribe link.

## Architecture

Four autonomous Go services, each owning its own PostgreSQL database, communicating over a NATS JetStream
event bus. The only synchronous service-to-service call is the saga's optional gRPC compensation transport (
`SAGA_COMPENSATE_TRANSPORT=grpc`); by default, that step is a NATS command too, so the event bus is the primary link.

| Service             | Binary                  | Responsibility                                                                                    |
|---------------------|-------------------------|---------------------------------------------------------------------------------------------------|
| `subscription-svc`  | `cmd/subscription`      | Public API; owns the repositories & subscriptions tables; publishes subscription lifecycle events |
| `scanner-svc`       | `cmd/scanner`           | Polls watched repositories on a schedule; publishes detected releases                             |
| `notifier-svc`      | `cmd/notifier`          | Consumes events; sends confirmation and release emails over SMTP                                  |
| `saga-orchestrator` | `cmd/saga-orchestrator` | Coordinates the Subscribe transaction; compensates (rolls back) a permanently failed confirmation |

- **Event bus:** services publish and consume JSON domain events over NATS JetStream. Inter-service
  events are JSON; the public API contract is protobuf.
- **Code layout:** a single Go module, sliced by bounded context under `internal/`, with hexagonal
  layers inside each module ([ADR-008](docs/adr/008-hexagonal-architecture.md),
  [ADR-010](docs/adr/010-modulith-bounded-contexts.md)). Module boundaries are enforced by
  `golangci-lint` `depguard` (lint) and by `arch-go` (`task arch-test`).
- **Shared infrastructure:** one Redis cache for GitHub responses, plus the observability stack.

For per-service component and sequence diagrams, the event catalog, and the layer-distribution diagram, see
the [system design document](docs/SDD.md).

## Public API

`subscription-svc` serves its entire public API from one connect-go handler on `SUBSCRIPTION_PORT` (default `8080`),
fronted by [Vanguard](https://github.com/connectrpc/vanguard-go) so REST, Connect, gRPC, and gRPC-Web
all share that port ([ADR-011](docs/adr/011-connect-transcoding.md)). The contract is
`proto/subscription/v1/subscription.proto`.

| Method | Path                        | Auth    | Description                     |
|--------|-----------------------------|---------|---------------------------------|
| `POST` | `/api/subscribe`            | API key | Subscribe email to a repository |
| `GET`  | `/api/confirm/{token}`      | —       | Confirm subscription            |
| `GET`  | `/api/unsubscribe/{token}`  | —       | Unsubscribe                     |
| `GET`  | `/api/subscriptions?email=` | API key | List subscriptions for an email |
| `GET`  | `/health`                   | —       | Health check                    |
| `GET`  | `/metrics`                  | —       | Prometheus metrics              |

Confirm and unsubscribe are public (no API key) because they are opened from email links in a browser.

## Observability

Brought up by `docker compose` alongside the services: Prometheus + Grafana for metrics, and
Filebeat → Elasticsearch → Kibana for logs. Each service exposes `/metrics` and `/health`. Logging is
structured JSON (`slog`).

## Running locally

```bash
cp .env.example .env
# fill in the secrets (GITHUB_TOKEN, SMTP_*, DB_*, REDIS_PASSWORD, JWT_SECRET, GRAFANA_PASSWORD, API_KEY)

task up-build    # build the images and start the whole stack
```

Each service runs its database migrations on startup. The public API is served on `:8080`.

See [`.env.example`](.env.example) for the full set of configuration variables.

## Development

The project uses [Task](https://taskfile.dev) (`Taskfile.yml`):

```bash
task build              # build all packages
task test-unit          # unit tests
task test-integration   # integration tests (requires Docker)
task lint-all           # lint everything
task arch-test          # verify architectural dependency rules (arch-go)
task fmt                # format
task proto              # regenerate protobuf stubs (requires the buf CLI)
```

See [TESTING.md](TESTING.md) for details on the test suites.

## Security notes

The `.env` file is for development and evaluation only — use a real secret manager in production. It is
gitignored; the committed [`.env.example`](.env.example) holds only placeholder values.