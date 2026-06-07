# ADR-002: Asynchronous Confirmation Email Sending

**Status:** Superseded by [ADR-006](006-mailer-dispatch-queue.md)

**Author:** Zabrodin Maksym

## Context

After a user subscribes, a confirmation email must be sent. The subscription is already saved in the database at this point. The question is whether the HTTP response should wait for the email to be sent.

## Candidates

1. **Synchronous** — block the handler until the email is sent
   - Pro: Simple error handling
   - Con: SMTP latency is unpredictable

2. **Goroutine** — save to DB, start a goroutine to send, return immediately
   - Pro: Response time depends only on the DB write
   - Con: Email failures are silent to the caller

3. **Message queue** — enqueue the email job, consume it in a worker
   - Pro: Guaranteed delivery with retries
   - Con: Requires extra infrastructure and a separate worker

## Decision

Use goroutine. The subscription is written to the database first, then a goroutine is launched to send the email. The handler returns immediately after the DB write. SMTP failures are logged but not returned to the caller.

## Consequences

**Pros:**
- `POST /api/subscribe` response time is not affected by SMTP

**Cons:**
- Email failures are only visible in logs
- If the process crashes right after the DB write, the confirmation email is lost — the user must re-subscribe
- No retry logic, adding it would require a persistent queue
