# ADR-012: Event-Driven Services over NATS JetStream

**Status:** Accepted.

Supersedes ADR-002, ADR-005, and ADR-009; updates the durability/semantics of
[ADR-006](006-mailer-dispatch-queue.md) and [ADR-007](007-background-scanner.md).

**Author:** Zabrodin Maksym

## Context

The system was a modulith with one shared Postgres, an in-app scan loop that owned the notify
decision, mTLS gRPC links between the API, scanner, and emailer, and an `EMAIL` NATS stream
carrying protobuf email commands. The synchronous links meant subscribe depended on scanner/emailer
uptime, and the single database coupled the services' schemas.

We want three autonomous services that can fail and deploy independently, with no synchronous
dependency between them.

## Decision

Three event-driven services, each owning its own Postgres, communicating only over NATS JetStream.

- **JSON events**, not protobuf — the `internal/shared/events` package owns versioned event structs
  with `go-playground/validator` tags. Protobuf is kept only for the public API.
- **Every event carries a `sagaID`** — a correlation seam for a future saga-orchestrator. The orchestrator can subscribe to all streams without any service change.
- **Confirmation token is a stateless JWT** ([ADR-004](004-tokens.md)) — no DB token row.
- **No inter-service gRPC, no mTLS, no certs** — the bus is the only link.

### Streams, subjects, producers/consumers

| Stream          | Subjects                                                  | Producer     | Consumers         |
|-----------------|-----------------------------------------------------------|--------------|-------------------|
| `SUBSCRIPTIONS` | `subscriptions.{pending,confirmed,removed,expired}`       | subscription | scanner, notifier |
| `RELEASES`      | `releases.detected`                                       | scanner      | notifier          |
|                 | `releases.notified`                                       | notifier     | scanner           |
| `NOTIFICATIONS` | `notifications.{confirmation,release}.{sent,failed,dead}` | notifier     | (saga, future)    |

### Choreographed flow (no orchestrator)

- **subscribe** → save pending (or write-free reissue) + publish `subscriptions.pending`. notifier
  sends the confirmation email.
- **confirm** (verify JWT) → mark confirmed + publish `subscriptions.confirmed`. scanner does
  `subscriber_count++` on `watched_repos`; notifier upserts its `subscriptions_read` model. **No
  welcome email.**
- **scan** (scanner schedule) → on a new tag publish `releases.detected`. notifier dedupes on
  `{repo_name, tag}`, sends, then publishes `releases.notified{sentCount, failedEmails}`. scanner
  advances `last_seen_tag` **only if `sentCount > 0`**.
- **unsubscribe** → delete + publish `subscriptions.removed` (scanner decrements, notifier deletes the
  read row).
- **pending cleanup** (subscription daily ticker) → publish `subscriptions.expired` per stale row.

### Delivery guarantees

- **At-least-once** with explicit ack: success → `Ack`; a poison message (validation failure,
  `broker.ErrTerminal`) → `Term` (dropped); any other error → `Nak` (redelivered, bounded by
  `MaxDeliver`). Counted by `events_consumed_total{subject,result}`.
- **Idempotency:** the notifier's `processed_releases` table dedupes a redelivered/re-detected
  `releases.detected` so a release is emailed once.
- **Transactional outbox (subscription only):** lifecycle events are enqueued in the same tx as
  the row change and relayed after commit, so a `subscriptions.*` event is never lost or published for
  an uncommitted change. The scanner and notifier publish directly — their flows are self-healing
  (a missed `releases.detected` is re-detected next scan; the read model re-converges on the next
  event), so they need no outbox.
- **Lost-batch guarantee:** the scanner advances the tag only after `releases.notified` with
  `sentCount > 0`; a failed/lost batch is re-detected and retried.
- **Retry / dead-letter:** the notifier persists send failures to `failed_notifications` /
  `failed_confirmations` and retries on a ticker, emitting `notifications.*.dead` when exhausted.

## Consequences

**Pros:**

- Services are decoupled — subscribe no longer depends on scanner/notifier uptime.
- Delivery is crash-durable end to end (outbox + JetStream redelivery + retry/dead-letter), replacing
  the old at-most-once email path.
- Each service owns its schema and scales/deploys independently.

**Cons:**

- Subscriber state is replicated across three stores → eventual consistency.
- At-least-once → possible duplicate work, mitigated by `processed_releases` dedupe.
- A message broker to operate, and more moving parts than the modulith.
