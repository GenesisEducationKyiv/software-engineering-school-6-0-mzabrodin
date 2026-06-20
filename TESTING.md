# Testing

## Requirements

| Test type   | Requirements    |
|-------------|-----------------|
| Unit        | Go              |
| Integration | Go + Docker     |

The project uses [Task](https://taskfile.dev) (`Taskfile.yml`).

---

## Unit tests

Unit tests live alongside the code in `_test.go` files and use no external infrastructure. They use
`testify` suites with inline mocks (mocks are hand-written in the `_test.go` files, not generated).

```bash
task test-unit
# equivalent: go test -count=1 ./...
```

CI additionally runs the suite with the race detector to catch concurrency issues:

```bash
go test -count=1 -race ./...
```

---

## Integration tests

Integration tests live in `test/integration/` and carry the `//go:build integration` tag, so they are
invisible to a plain `go test ./...`. Their backing infrastructure starts automatically in Docker
containers via [testcontainers](https://golang.testcontainers.org/) — no manual setup is needed:

- **PostgreSQL** — the subscription service schema, plus the outbox table (applied under a separate
  `outbox_schema_migrations` table). Migrations are embedded in the binary and applied on startup.
- **Redis** — the GitHub response cache.
- **NATS (JetStream)** — the event bus the services publish to and consume from.

The containers and a shared DB pool are created once in `TestMain` and reused across the whole suite.
Each test truncates the relevant tables before running to guarantee isolation. The tests exercise the
public handler (`subscription.NewHandler`) over an in-process h2c server — REST through a plain HTTP
client and gRPC through a grpc-go client against the same port.

```bash
task test-integration
# equivalent: go test -count=1 -tags integration ./test/integration/...
```

Run both unit and integration tests together:

```bash
task test-all
```

---

## Linting

```bash
task lint-core           # lint cmd & internal
task lint-integration    # lint integration tests (applies the integration build tag)
task lint-all            # both of the above
```

Pass `--fix` through any lint task, e.g. `task lint-core -- --fix`.

Format:

```bash
task fmt    # golangci-lint fmt
```