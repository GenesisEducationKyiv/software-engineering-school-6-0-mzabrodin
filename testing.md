# Testing

## Requirements

| Test type   | Requirements    |
|-------------|-----------------|
| Unit        | Go              |
| Integration | Go + Docker     |

---

## Unit tests

Unit tests live alongside the code in `_test.go` files and use no external infrastructure. Run them with:

```bash
make test
# equivalent: go test -v -count=1 ./...
```

---

## Integration tests

Integration tests live in `test/integration/` and carry the `//go:build integration` tag, so they are invisible to `go test ./...`. PostgreSQL starts automatically in a Docker container via testcontainers, no manual setup is needed.

Each test truncates all tables before running to guarantee isolation. The shared DB pool is created once in `TestMain` and reused across all tests in the suite.

```bash
make test-integration
# equivalent: go test -v -count=1 -tags integration ./test/integration/...
```

---

## Linting

```bash
make lint                # lint all packages
make lint-integration    # lint integration tests (applies the integration build tag)
```

Format:

```bash
make fmt    # runs gofmt + goimports + golines across all files
```