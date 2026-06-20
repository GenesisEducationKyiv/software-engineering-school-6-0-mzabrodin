# ADR-006: Mailer with Background Dispatch Queue and Connection Reuse

**Status:** Accepted.

**Author:** Zabrodin Maksym

## Context

The notifier service sends two kinds of email over SMTP: single confirmation emails and batched
release notifications. Within the service, the last hop to SMTP needs to reuse connections, bound
concurrency, and drain in-flight mail on shutdown — without dialing a fresh SMTP connection per
message or spawning unbounded goroutines.

## Decision

The `Mailer` (in `internal/notifier/adapter/mailer`) owns a buffered `jobs` channel and a single
`dispatch()` goroutine that delivers jobs serially. It is fed by the service's NATS event consumers
(`subscriptions.pending` → confirmation, `releases.detected` → release batch), not by any in-process
caller.

- **Connection reuse:** one SMTP dial per batch — connect, send every message, close — instead of one
  dial per message.
- **Library:** [go-mail](https://github.com/wneessen/go-mail), context-aware (`DialWithContext`,
  per-message `Send`).
- **Shutdown** (`Shutdown(ctx)`): close `jobs` and wait for the dispatcher to drain, bounded by the
  context.
- **Templates:** HTML templates per email kind, parsed once at package init.
- **Metrics:** `email_sends_total{type,status}` and `email_send_duration_seconds{type}`.

## Consequences

**Pros:**
- One SMTP connection per batch; bounded resource use; graceful drain on shutdown.
- Delivery sits behind a `sender` interface, so tests inject a fake.

**Cons:**
- The queue is in memory — a crash loses whatever is queued in that instant. This is acceptable
  now because durability lives upstream: JetStream redelivers undelivered events, and failures are
  persisted to `failed_notifications` / `failed_confirmations` and retried (then dead-lettered) by the
  notifier's retry ticker (see [ADR-012](012-event-driven-services.md)). The in-memory queue is just
  the final SMTP hop, no longer the system's durability boundary.
