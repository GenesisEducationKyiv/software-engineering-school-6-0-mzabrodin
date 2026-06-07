# ADR-007: Background Scanner with a Concurrent Worker Pool

**Status:** Accepted

**Author:** Zabrodin Maksym

## Context

The service must periodically check every subscribed repository for a new release and notify confirmed subscribers.
Three constraints shape the design:

- One slow or broken repository must not block the rest of the pass.
- GitHub rate limits are shared across the whole process, so hitting a limit should stop the current pass rather than
  hammer the API.
- The work is I/O-bound (GitHub + SMTP + DB), so some concurrency helps, but unbounded concurrency would risk rate
  limits and resource spikes.

## Candidates

1. **Sequential scan** — one repo at a time
    - Pro: trivial, no rate-limit bursts
    - Con: a slow repo delays everything; total time scales with repo count

2. **Goroutine per repo** — unbounded fan-out
    - Pro: fast
    - Con: uncontrolled concurrency, easy-to-trip rate limits

3. **Bounded worker pool** — fixed concurrency across repos
    - Pro: predictable parallelism, rate-limit friendly
    - Con: slightly more coordination

## Decision

Schedule with a ticker and scan with a bounded pool.

- **Scheduling:** a `Scheduler` (in `internal/infrastructure/scheduler`) runs the scan once immediately, then every
  `SCAN_INTERVAL`. It is kept separate from the `Scanner` use case (`internal/usecase/scanner`) so timing is
  infrastructure and detection is business logic.
- **Concurrency:** an `errgroup` with `SetLimit(SCAN_WORKERS)` processes repositories with bounded parallelism (candidate 3).
- **Per-repo error isolation:** an error on one repository is logged and counted (`scanner_errors_total{reason}`) but
  does not fail the pass — the worker returns `nil` so the group keeps going.
- **Rate-limit / auth abort:** `ErrRateLimited` and `ErrUnauthorized` are the exception — they are returned from the
  worker, which cancels the group context and stops the pass early, since they affect every repository, not just one.
- **Shared `last_seen_tag`:** the latest seen tag is stored once per repository (not per subscriber), so each cycle
  makes a single GitHub call per repo regardless of subscriber count. The tag is advanced after a successful notification (or
  immediately when a repo has no confirmed subscribers).
- **Log correlation:** each pass is tagged with a scan ID injected into the context (`logging.WithScanID`), so all log
  lines for one cycle share an identifier.

## Consequences

**Pros:**

- Resilient — a broken repository never aborts the whole pass
- Bounded, rate-limit-aware concurrency
- One GitHub call per repo per cycle regardless of subscriber count
- Observable via metrics and per-pass scan IDs

**Cons:**

- **At-most-once** notification semantics: because `last_seen_tag` advances after a partially successful batch, a
  subscriber whose individual sending failed is not retried for that release (failures are logged only)
- Detection latency is up to `SCAN_INTERVAL` plus the GitHub cache TTL (see [ADR-003](003-redis-caching-strategy.md))
