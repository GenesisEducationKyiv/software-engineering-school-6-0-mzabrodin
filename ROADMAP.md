# Refactoring & Feature Roadmap

> 2-week sprint starting 2026-05-30
> Owner: Maksym Zabrodin
> Scope: 10 working days

---

## Your Ideas

- [ ] Apply a clean-layered architecture across all packages
- [ ] Review and improve all existing tests
- [ ] Build a small frontend with E2E tests
- [ ] Refactor toward idiomatic Go + SOLID & GRASP (use-case handlers, etc.)
- [ ] Refactor the scanner to use channels
- [ ] Refactor the mailer for more consistent work with the scanner

---

## Honest State of the Code (read first)

The codebase is already in good shape. Before listing problems, here's what is **already correct** so we don't "refactor" working code for its own sake:

- **Layering is already clean.** `API → Service → Repository → Domain` is respected. `domain` has no external deps beyond `uuid`.
- **Interfaces are already defined consumer-side** (`scanner` declares the narrow repo/GitHub interfaces it needs; `service` declares its own). This *is* idiomatic Go and satisfies Interface Segregation. We will **not** centralize them into one `interfaces.go` — that would be less idiomatic, not more.
- **DTOs already exist and are used** (`api/dto.go`, `toSubscriptionResponses`). The domain type never leaks to the JSON wire format.
- **The scanner is already factored** into `checkRepo` / `getRelease` / `handleReleaseError` / `notify`. It is unit-tested across 8+ cases.
- **The concrete `Mailer` is already async and context-aware** (`SendConfirmation(ctx,...) error`, `SendReleaseNotifications(ctx,...) error`). `ConfirmationNotifier` is a deliberate fire-and-forget *adapter*, not an accidental mismatch.
- **`testcontainers-go` and `golang.org/x/sync` are already in `go.mod`** — integration-test and `errgroup` infrastructure is ready to use.

This reframes the work: it's mostly **filling test gaps, adding a frontend, and a few targeted refactors**, not a teardown.

---

## Real Issues Found (grounded, with file:line)

| # | File:line                             | Problem                                                                                                                                                                                 | Severity | Real or stylistic?     |
|---|---------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|------------------------|
| 1 | `config/config.go:39-46`              | `Load` logs and swallows `envconfig.Process` errors, then returns partial config. App boots with empty `DATABASE_URL`/SMTP and fails later in a confusing way. `run()` never checks it. | **High** | Real bug               |
| 2 | `service/subscription.go:100`         | `SendConfirmation` is fire-and-forget; if the email never sends, the user still gets "subscription successful, confirmation email sent".                                                | Medium   | Real (design tradeoff) |
| 3 | `scanner/scanner.go:140-148`          | `handleReleaseError` returns `nil` on `ErrUnauthorized`. A bad/missing token means *every* scan silently no-ops with only a `Warn`.                                                     | Medium   | Real (needs alerting)  |
| 4 | `service/confirmation_notifier.go:27` | Uses `context.Background()`, dropping request-scoped values (trace/request IDs) from logs in the send goroutine.                                                                        | Low      | Real, minor            |
| 5 | `mailer/mailer.go:125`                | `errors.Join(errs...)` collapses partial batch success into one opaque error; caller can't tell 1 vs 1000 failures (each is logged, though).                                            | Low      | Real, minor            |
| 6 | `api/handler.go:16,74,95`             | `tokenHexLen = 64` magic number; the per-handler `switch errors.Is(...)` is duplicated 4x.                                                                                              | Low      | Stylistic              |
| 7 | `api/routes.go`                       | No rate limiting on `POST /api/subscribe`; unauthenticated when `API_KEY` unset → unbounded unconfirmed rows.                                                                           | Medium   | Real (hardening)       |

### Test gaps (the biggest real opportunity)

| Package                                              | Current coverage             | Gap                                                                                                          |
|------------------------------------------------------|------------------------------|--------------------------------------------------------------------------------------------------------------|
| `repository/`                                        | **none**                     | No tests at all. `testcontainers/postgres` already a dep → integration tests are the highest-value addition. |
| `config/`                                            | none                         | `Load` (and the swallow bug) untested.                                                                       |
| `mailer/`                                            | templates + empty-slice only | The actual SMTP send/batch/partial-failure path is untested.                                                 |
| `cache/`                                             | none                         | Redis wrapper untested (`testcontainers/redis` already a dep).                                               |
| `urlbuilder/`, `repository/metrics.go`               | none                         | Trivial but uncovered.                                                                                       |
| `api/`, `service/`, `scanner/`, `github/`, `domain/` | good                         | Solid suites already exist.                                                                                  |

---

## My Proposed Additions

- [x] **Fix the config swallow bug first** (#1) — make `Load` return `(*Config, error)` and fail fast in `run()`.
- [ ] **Metrics as a decorator**, not inline `defer` in every method — demonstrates Decorator and Single Responsibility cleanly, removes ~10 repeated blocks.
- [ ] **Error→HTTP mapping table** in the API instead of 4 duplicated `switch` blocks — demonstrates Open/Closed.
- [ ] **Unify the two email paths through one channel-fed dispatcher** in the mailer — this is what makes "mailer consistent with scanner" concrete (see Day 4).
- [ ] **Rate-limit middleware** on `/api/subscribe` (`golang.org/x/time/rate`).
- [ ] **Surface unauthorized GitHub token** as a dedicated metric/health signal instead of a silent per-repo `Warn` (#3).

---

## Week 1 — Targeted Refactoring

### Day 1 · Config fix and metrics decorator
**Goal:** fail fast on bad config; stop scattering metrics across business logic.

- `config.Load` → `func Load() (*Config, error)`; `run()` returns the error so the process exits with a clear message instead of booting half-configured. Covers issue #1.
- Extract the repeated `defer func() { metrics.X.WithLabelValues(...).Inc() }()` blocks (in every `service` and `repository` method) into a thin decorator:
  - `service`: a `metricsService` wrapper implementing the `subscriptionService` interface, delegating to the real one, and recording the metric. The API already depends on the interface, so wiring is a one-line change in `main.go`.
  - This is the concrete "use-case handler / decorator" idea — SRP: business logic stops knowing about Prometheus.
- **Tests:** `config_test.go` (required-var missing → error; defaults applied; `SlogLevel` parsing). Decorator test asserting the wrapped call still runs and the metric label is chosen correctly.

### Day 2 · API cleanup (Open/Closed)
**Goal:** remove duplication in handlers.

- Replace the 4 duplicated `switch errors.Is(...)` blocks with a single mapping:
  ```go
  var statusByErr = []struct{ err error; status int; msg string }{
      {domain.ErrInvalidEmail,  400, "invalid email format"},
      {domain.ErrInvalidRepo,   400, "invalid repo format, expected owner/repo"},
      {domain.ErrRepoNotFound,  404, "repository not found on GitHub"},
      {domain.ErrAlreadyExists, 409, "email already subscribed to this repository"},
      {domain.ErrNotFound,      404, "token not found"},
  }
  func writeDomainError(w, log, ctx, err) { /* iterate, fall back to 500 */ }
  ```
  (slice, not map — `errors.Is` needs iteration for wrapped sentinels.)
- Name the magic `tokenHexLen = 64` with a comment tying it to `tokenBytes * 2`.
- **Tests:** existing `handler_test.go` should pass unchanged (behavior-preserving) — that's the safety net. Add a case for an unmapped error → 500.

### Day 3 · Scanner → channel pipeline
**Goal:** bounded-concurrency pipeline; cleaner cancellation. Honest note: the current sequential loop is correct; the real win here is **parallelizing the slow GitHub I/O with a worker pool while bounding it** (don't exceed rate limits).

Target shape (using the already-present `golang.org/x/sync/errgroup`):
```
GetAllWithSubscriptions
   → jobs channel (chan *domain.Repository)
   → N worker goroutines  → each calls checkRepo (the slow GitHub I/O)
   → ctx-aware via errgroup.WithContext
```
- Add `WorkerCount int` to config (`SCAN_WORKERS`, default 5).
- Keep `checkRepo` exactly as-is (it's already pure per-repo logic) — only `scan()` changes to fan out work.
- Per-repo errors stay isolated (already the contract) — log plus metric, never abort the batch.
- **Tests:** new `scan()` test asserting (a) all repos processed, (b) one repo erroring doesn't stop others, (c) `ctx` cancel stops the pool promptly. Existing `checkRepo` suite stays untouched.

### Day 4 · Mailer ↔ scanner consistency
**Goal:** one email path, fed by a channel — this is the "consistent work with scanner" centerpiece.

Current asymmetry: release emails return errors synchronously; confirmation emails are fire-and-forget via `ConfirmationNotifier`. Proposal — a single dispatcher both sides feed:
```
type emailJob struct { kind; to; repo; ... }   // confirmation OR release
Mailer runs one dispatcher goroutine reading a buffered chan emailJob,
reusing one SMTP connection, with retry + per-job result.
```
- Both `service` (confirmation) and `scanner`'s `ReleaseNotifier` (release) enqueue jobs; neither blocks on SMTP.
- `Shutdown(ctx)` drains the channel and closes the connection (replaces both `ConfirmationNotifier.Shutdown` and the per-batch dial/close at `mailer.go:102-109`).
- Add bounded retry (three attempts, 1 s/2s/4s) for transient SMTP errors; return a `BatchResult{Sent, Failed []string}` for the release path so partial success is observable (fixes #5).
- Switch the confirmation goroutine to `context.WithoutCancel(ctx)` to keep trace IDs in logs (fixes #4).
- **Tests:** `mailer` dispatcher test (enqueue → drained on shutdown), retry test (fail twice then succeed), `BatchResult` partial-failure test. This finally covers the SMTP path that today has zero tests.

### Day 5 · Hardening and buffer
**Goal:** security hardening; absorb slippage from Days 1–4.

- Rate-limit middleware on `/api/subscribe` via `golang.org/x/time/rate` (token bucket keyed by `middleware.RealIP`, e.g., 10/min). Issue #7.
- Promote unauthorized-token detection (#3) to a gauge/health signal so a misconfigured `GITHUB_TOKEN` is visible, not silent.
- Catch-up day for anything that overran. **Do not** start the frontend until the backend refactor is green (`go test -race ./...` + `golangci-lint run`).

---

## Week 2 — Tests, Frontend & E2E

### Day 6–7 · Repository & infrastructure tests
**Goal:** close the single biggest coverage gap. `testcontainers` is already a dependency.

- `repository/integration_test.go` (build-tagged `//go:build integration`, real Postgres container, real migrations):
  - `Create` + `ON CONFLICT` duplicate → `ErrAlreadyExists`
  - `GetByName` hit / miss (`ErrNotFound`)
  - `GetAllWithSubscriptions` returns only repos with **confirmed** subs
  - `GetConfirmedByRepoID`, `Confirm`, `Delete` (incl. `RowsAffected()==0 → ErrNotFound`)
  - `UpdateLastSeenTag` sets tag + `checked_at`
- `cache/cache_test.go` against a Redis container: set/get/miss, graceful behavior on a closed client.
- `repository/metrics_test.go`: `trackDBQuery` records the right labels on success vs. error.
- Wire an `integration` tag target into CI (keep unit tests fast/default).

### Day 8 · Scanner & service test top-ups
**Goal:** cover the Day-3/Day-4 changes and the remaining unit gaps.

- Pipeline tests from Day 3 (if not already landed there).
- `urlbuilder` table test (trivial, completes coverage).
- `service` test: confirmation now goes through the dispatcher — assert enqueue happens and `Shutdown` drains.
- Run `go test -race -count=1 ./...` and treat any flake as a real finding.

### Day 9 · Frontend
**Goal:** minimal, dependency-free UI for the real user journey. Served by Go via `embed.FS` — no Node toolchain.

```
web/static/
├── index.html        # subscribe form (email + owner/repo) → POST /api/subscribe
├── subscriptions.html# look up subscriptions by email → GET /api/subscriptions
└── style.css
internal/web/embed.go # go:embed + http.FileServer, mounted at "/" in routes.go
```
- Confirm/unsubscribe already work via `GET /api/confirm/{token}` and `/unsubscribe/{token}` (links in emails) — the frontend just needs subscribe and lookup pages. No new backend endpoints.
- Plain HTML/CSS/vanilla JS; accessible, no framework. Keeps it auditable and matches the "small frontend" ask.
- Mount static routes **after** `/api` in `NewRouter`.

### Day 10 · E2E tests
**Goal:** full happy path and key errors against a real server, Go-only (no Playwright/Cypress).

- `e2e/e2e_test.go` (`//go:build e2e`): spin up Postgres + Redis containers, a stub SMTP sink, and the real router via `httptest.Server`.
  - `Subscribe → read confirm token from DB → GET /confirm/{token} → 200`
  - `Subscribe` twice → second `409`
  - `Subscribe → confirm → GET /unsubscribe/{token}` → row deleted
  - Trigger one `scan()` manually → assert the SMTP sink received the release email
  - `GET /` → 200 and body contains the subscribe `<form>`
- This exercises the scanner→mailer→repository chain end-to-end, which nothing covers today.

---

## Dependency Additions

| Package                              | Reason                          | Status                                             |
|--------------------------------------|---------------------------------|----------------------------------------------------|
| `golang.org/x/time/rate`             | Subscribe rate limiting (Day 5) | New (small, std-adjacent)                          |
| `golang.org/x/sync/errgroup`         | Scanner worker pool (Day 3)     | **Already present** (indirect) — promote to direct |
| `testcontainers-go` (postgres/redis) | Integration + E2E tests         | **Already present**                                |

No web framework, ORM, message queue, or mock generator is added — see below.

---

## Scope Decisions (what we are NOT doing, and why)

| Idea                                                      | Decision                                                                                                 |
|-----------------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| Central `repository/interfaces.go`                        | **Rejected** — consumer-side interfaces are already idiomatic; centralizing reduces ISP quality.         |
| React/Vue frontend                                        | **Rejected** — adds a build toolchain; plain `embed.FS` HTML meets "small frontend" and stays auditable. |
| Playwright/Cypress E2E                                    | **Rejected** — keep the stack Go-only; `httptest` + testcontainers covers the journey.                   |
| ORM                                                       | **Rejected** — raw `pgx` gives explicit, reviewable SQL.                                                 |
| Generated mocks (mockery)                                 | **Rejected** — inline testify mocks are consistent and need no regen step.                               |
| Message queue (Kafka/NATS)                                | **Rejected** — the in-process channel dispatcher (Day 4) gives the decoupling this scale needs.          |
| Splitting `SubscriptionService` into per-use-case structs | **Optional / stretch** — current single service is cohesive; only split if a method count grows.         |

---

## Timeline Summary

| Day | Focus                                                   | Net new tests                      |
|-----|---------------------------------------------------------|------------------------------------|
| 1   | Config fail-fast + metrics decorator                    | config, decorator                  |
| 2   | API error-mapping table (Open/Closed)                   | unmapped-error case                |
| 3   | Scanner channel/worker-pool pipeline                    | scan() concurrency                 |
| 4   | Mailer channel dispatcher + retry (scanner consistency) | dispatcher, retry, partial-failure |
| 5   | Rate limiting + token-error visibility + buffer         | middleware                         |
| 6-7 | Repository + cache integration tests (testcontainers)   | repository, cache, db-metrics      |
| 8   | Scanner/service/urlbuilder top-ups, `-race` pass        | misc                               |
| 9   | Frontend (`embed.FS`, 2 pages)                          | —                                  |
| 10  | E2E suite                                               | full-chain E2E                     |

**Definition of done:** `go test -race -count=1 ./...` green, integration + e2e tags green, `golangci-lint run` clean, frontend reachable at `/`.