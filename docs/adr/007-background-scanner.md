# ADR-007: Background Scanner with a Concurrent Worker Pool

**Status:** Accepted.

**Author:** Zabrodin Maksym

## Context

The system must periodically check every watched repository for a new release. Three constraints shape
the design:

- One slow or broken repository must not block the rest of the pass.
- GitHub rate limits are shared across the process, so hitting a limit should stop the current pass
  rather than hammer the API.
- The work is I/O-bound, so bounded concurrency helps; unbounded concurrency risks rate limits and
  resource spikes.

## Decision

The scanner service owns the schedule and the watch set; it scans with a bounded worker pool.

- **Scheduling:** the scanner's `Scheduler` (`internal/infrastructure/scheduler`) runs a pass once on
  startup, then every `SCAN_INTERVAL`, minting a `scan_id` per pass for log correlation. Timing is
  infrastructure; detection is the watch use case (`internal/scanner/usecase/watch`).
- **Work set:** the repositories in the scanner's own `watched_repos` with `subscriber_count > 0`
  (the count is maintained from `subscriptions.confirmed` / `subscriptions.removed` events).
- **Concurrency:** an `errgroup` with `SetLimit(SCAN_WORKERS)` fetches the latest release per repo
  (shared Redis cache).
- **Per-repo error isolation:** one repo's error is logged and counted
  (`scanner_errors_total{reason}`) but does not fail the pass.
- **Rate-limit / auth abort:** `ErrRateLimited` / `ErrUnauthorized` cancel the whole pass — they
  affect every repository.
- **One call per repo:** `last_seen_tag` is stored once per repo (not per subscriber), so each pass
  makes a single GitHub call per repo regardless of subscriber count.
- **Detection → event:** NULL `last_seen_tag` → seed silently (no event); unchanged → skip; changed →
  publish `releases.detected` (with a per-release `sagaID`) and **advance the tag only later**, when a
  `releases.notified` with `sentCount > 0` comes back (the lost-batch guarantee, now cross-service —
  see [ADR-012](012-event-driven-services.md)).

## Consequences

**Pros:**

- Resilient — a broken repository never aborts the pass.
- Bounded, rate-limit-aware concurrency; one GitHub call per repo per cycle.
- Observable via metrics and per-pass scan IDs.

**Cons:**

- Detection latency is up to `SCAN_INTERVAL` plus the GitHub cache TTL (see
  [ADR-003](003-redis-caching-strategy.md)).
- Delivery is at-least-once: a redelivered `releases.detected` could re-send, mitigated by the
  notifier's `processed_releases` dedupe (see [ADR-012](012-event-driven-services.md)).
