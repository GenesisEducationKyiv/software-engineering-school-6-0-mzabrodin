# ADR-013: Orchestrated Saga for the Subscribe Distributed Transaction

**Status:** Accepted.

**Author:** Zabrodin Maksym

## Context

[ADR-012](012-event-driven-services.md) made the system fully event-driven and planted a deliberate
seam — every event carries a `sagaID` — so that a future orchestrator could subscribe to all
streams without any service change. No orchestrator existed yet.

The one true distributed transaction in the system is Subscribe, which spans two services:
subscription-svc owns the `subscriptions` row, and notifier-svc sends the confirmation email. Today
these are purely choreographed — there is no coordinator and no rollback. If the confirmation email
permanently fails (the notifier exhausts `MAX_RETRIES` and emits
`notifications.confirmation.dead`), the unconfirmed `subscriptions` row just sits there until the
daily pending-cleanup ticker expires it. There is no transactional view of "subscribe succeeded or
was rolled back."

We want to realize the seam: track the transaction's state across both services and compensate
(delete the pending subscription) when the confirmation step is exhausted.

## Decision

Add a hand-rolled orchestrated saga-orchestrator service (`cmd/saga-orchestrator`,`internal/saga`), owning its own
Postgres, that subscribes to the existing streams, records the Subscribe saga's state in a `sagas` table, and publishes
a compensating command when the confirmation step dies.

### The saga (state machine)

```
START      subscriptions.pending            (subscription-svc)  → PENDING
STEP 1 ok  notifications.confirmation.sent  (notifier-svc)      → CONFIRMATION_SENT
COMPLETE   subscriptions.confirmed          (subscription-svc)  → COMPLETED
FAIL       notifications.confirmation.dead  (notifier-svc)      → COMPENSATED (+ enqueue
                                                                   saga.compensate via outbox)
EXPIRE     subscriptions.expired            (subscription-svc)  → EXPIRED
```

Compensation triggers on `notifications.confirmation.dead`, not `.failed`: `.failed` is a
transient sent error that the notifier's retrier handles on its own. Only `.dead` (retries
exhausted) is a permanent failure that warrants rolling the transaction back.

### Correlation (dual key)

The same `sagaID` flows `subscriptions.pending` → `notifications.confirmation.*` (the notifier
forwards `ev.SagaID`, and `failed_confirmations` persists it, so `.dead` carries it too) — so those
events correlate by `sagaID`. But `subscriptions.confirmed`/`expired` mint fresh sagaIDs
(confirm is a stateless JWT-link click that carries no sagaID; cleanup mints its own) — so those
correlate by `(email, repoName)`, which the `sagas` table enforces with
`UNIQUE (type, email, repo_name)`.

### Compensation

When `OnConfirmationDead` fires for a saga in `PENDING` or `CONFIRMATION_SENT`, the coordinator does
one transaction against the saga DB: mark the saga `COMPENSATED` and enqueue the
`saga.compensate` command into a transactional outbox. A relay then publishes it durably on the `SAGAS` stream. Because
the state write and the command are committed together, the rollback can never be lost between them — there is no
intermediate `COMPENSATING` state to strand. Subscription-svc consumes `saga.compensate` and runs a `compensate` use
case that deletes the pending row — `DeletePendingByEmailAndRepo`, scoped to `confirmed = false` so it can never touch a
confirmed subscription. No event is re-published after the delete: a never-confirmed subscription left no downstream
state to undo.

Delivery is at-least-once (JetStream), so the orchestrator must be idempotent: transitions are
race-safe conditional `UPDATE`s gated on the allowed source states; `OnConfirmationDead` is a
`Get` + state switch that no-ops on already-terminal sagas, so a redelivered `.dead` event after the
saga is already `COMPENSATED` does nothing; the compensate consumer's delete is idempotent (zero rows
is not an error).

### End-to-end durability: the notifier outbox

Compensation can only fire if the saga actually receives `notifications.confirmation.dead`, and that
event is produced by notifier-svc's retrier. The retrier originally deleted the
`failed_confirmations` row and then published `.dead` best-effort — so a lost publish (or a crash
between the two) removed the row *and* dropped the event, and the saga would never compensate. To
close that gap, notifier-svc gained its **own transactional outbox** (the same `internal/infrastructure/outbox`
relay used here and by subscription-svc): the retrier now does `tx{ delete failed_confirmations;
enqueue confirmation.dead }` and the relay publishes durably. The same treatment covers the other
state-coupled notifier publishes (`confirmation.sent`, `release.{sent,dead}`, and
`release.notified` + the `processed_releases` mark in `notifyrelease`). This updates ADR-012's
"transactional outbox (subscription only)" note — subscription, saga, and notifier now each own an
outbox; only scanner-svc still publishes directly (its `releases.detected` is self-healing — a lost
one is re-detected on the next scan).

### Why hand-rolled, and not Temporal or Watermill

| Option               | Verdict   | Reasoning                                                                                                                                                                                                                                                                                                                                          |
|----------------------|-----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Hand-rolled (chosen) | +         | Fits the existing NATS modulith, adds no new infrastructure, and keeps the state machine / compensation / correlation visible in code. The orchestrator looks exactly like the other three services (hexagonal, consumer-defined ports, `ConsumerSpec` wiring).                                                                                    |
| Temporal             | - for now | The production-grade orchestrator. Would let us delete the transactional outbox, the notifier's retrier, and the `sagas` table (durable execution + activity retry policies replace them). But it adds a Temporal server + its own datastore + worker processes, is a multi-service rewrite, is not a message bus (release fan-out stays in NATS). |
| Watermill            | -         | Would replace the messaging glue across all services with its router/middleware, but ships no saga orchestrator — the state machine is still hand-written. Large churn for little saga-specific gain.                                                                                                                                              |

## Consequences

**Pros:**

- The one true distributed transaction now has a coordinator and a rollback — a permanently failed
  confirmation no longer leaves an orphan pending row until the daily sweep.
- The `sagaID` seam from ADR-012 is realized with only additive wiring in subscription-svc (its first
  consumer) and notifier-svc (the outbox below).
- The orchestrator is a drop-in module like the others, so the saga's state is observable in one place.

**Cons:**

- Another service and another Postgres to operate.
- At-least-once delivery means every step must be idempotent (handled by conditional transitions and the
  `confirmed = false` scoped delete).
- The saga state is eventually consistent with the `subscriptions` row — the orchestrator observes
  the transaction, it does not hold a lock across it.