# ADR-006: Mailer with Background Dispatch Queue and Connection Reuse

**Status:** Accepted (supersedes [ADR-002](002-async-confirmation-email.md))

**Author:** Zabrodin Maksym

## Context

The service sends email over SMTP on two paths:

- **Confirmation emails** — one message, triggered synchronously by a subscription request.
- **Release notifications** — a batch of messages per repository, triggered by the scanner.

ADR-002 chose a goroutine-per-confirmation: save to the DB, spawn a goroutine to send, return immediately. That works
for a single confirmation but does not generalize once batched release notifications and
the [go-mail](https://github.com/wneessen/go-mail) library entered the picture. Spawning a goroutine per email dials a
fresh SMTP connection every time, allows unbounded goroutines under load, and offers no way to drain in-flight emails on
shutdown.

## Candidates

1. **Goroutine per send** (ADR-002)
    - Con: a new SMTP dial per email, unbounded goroutines, nothing to drain on shutdown

2. **External durable queue** (Redis / RabbitMQ + worker)
    - Pro: durable delivery with retries
    - Con: extra infrastructure and a separate worker process

3. **In-process buffered channel + single dispatcher goroutine**
    - Pro: bounded, reuses one SMTP connection per batch, drains on shutdown
    - Con: still in-memory — no durability across a crash

## Decision

Use an in-process dispatch queue (candidate 3). The `Mailer` owns a buffered `jobs` channel and a single `dispatch()`
goroutine that delivers jobs serially.

- **Connection reuse:** `smtpSender.sendBatch` dials once per job, sends every message on that connection, then closes
  it — one SMTP round-trip setup per batch instead of per message.
- **Confirmations** (`SendConfirmation`): render + enqueue **fire-and-forget**. The job carries
  `context.WithoutCancel(ctx)` so the sending outlives the HTTP/gRPC request that triggered it.
- **Release notifications** (`SendReleaseNotifications`): enqueue the batch and block on a result channel, returning an
  `entity.BatchResult` (sent count and failed recipients) to the scanner.
- **Library:** go-mail (`github.com/wneessen/go-mail`), which is context-aware (`DialWithContext`, per-message `Send`),
  replacing gomail.v2.
- **Shutdown** (`Shutdown(ctx)`): close the `jobs` channel and wait for the dispatcher to drain, bounded by the passed
  context.
- **Templates:** HTML templates per email kind, parsed once at package init.
- **Metrics:** `email_sends_total{type,status}` and `email_send_duration_seconds{type}`.

## Consequences

**Pros:**

- One SMTP connection per batch instead of per message
- Bounded resource use — a single dispatcher, not unbounded goroutines
- Graceful shutdown drains queued mail (the best effort)
- Confirmations remain non-blocking; the scanner still gets per-recipient delivery results
- Delivery is behind a `sender` interface, so unit tests inject a fake

**Cons:**

- The queue is in memory — a crash still loses queued mail; there is no retry (same durability issue as
  ADR-002). [ADR-009](009-future-microservices-split.md) proposes moving delivery behind a durable queue.
- The release path blocks the calling scan goroutine until the batch completes (bounded by `sendTimeout`)
- A fixed buffer (64 jobs) can apply backpressure under bursts
