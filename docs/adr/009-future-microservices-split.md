# ADR-009: Future Split into API, Scanner, and Emailer Services

**Status:** Proposed

**Author:** Zabrodin Maksym

## Context

The service is a single binary that bundles three concerns with very different
scaling and failure profiles: request-serving (the API), GitHub polling (the
scanner), and SMTP delivery (the emailer). The in-process mailer also has no
delivery durability — queued mail is lost on a crash (see the cons in
[ADR-006](006-mailer-dispatch-queue.md)).

The hexagonal architecture ([ADR-008](008-hexagonal-architecture.md)) already
isolates these concerns behind ports, so extracting them into separate services is
low friction when the need arises. This ADR records the intended direction; it is
**not yet implemented**.

## Candidates

1. **Stay a modular monolith** — keep everything in one binary
    - Pro: simplest to develop, deploy, and operate
   - Con: it concerns scale together; no delivery durability

2. **Extract the emailer first, then the scanner** — phased split
    - Pro: incremental; each step is independently valuable
    - Con: introduces distributed-system concerns gradually

3. **Full per-concern split up front**
    - Pro: clean target state
    - Con: large, risky change before it is necessary

## Decision

**For now, this ADR commits to one step only: extract the Emailer into its own
microservice**, leaving the API and scanner together in the existing binary. The
mailer already sits behind ports ([ADR-008](008-hexagonal-architecture.md)), so the
app swaps its in-process `Mailer` for a thin client adapter implementing the same
interfaces — no use-case changes. The ADR-006 dispatch design (templates, batch
connection reuse) moves into the new service, which is where durable retries would
later live. The app talks to the Emailer over **gRPC**, consistent with
[ADR-005](005-grpc-api-alongside-rest.md) and the existing buf/protovalidate tooling.

This is the smallest change that delivers value — it isolates SMTP concerns and
keeps the app unblocked by mail delivery — at low risk.

### Future direction (not adopted yet)

This is *for now* only. Longer term we intend to split into three services — **API**,
**Scanner**, and **Emailer** — with:

- **API ↔ Scanner** over **gRPC** (synchronous request/response).
- **Scanner → Emailer** **asynchronously through a message queue**, with a broker/relay
  service in between, so release detection is decoupled from delivery and email jobs
  survive emailer downtime.

```mermaid
flowchart LR
    Client -->|HTTP / gRPC| API
    API <-->|gRPC| Scanner
    API --> DB[(PostgreSQL)]
    Scanner -->|GitHub REST| GitHub[(GitHub API)]
    Scanner -->|publish email jobs| Queue[[Message Queue]]
    Queue --> Emailer
    Emailer -->|SMTP| SMTP[(SMTP provider)]
```

That full split remains *Proposed*; the Emailer extraction above is the only part
decided in this ADR.

## Consequences

Of the decided step — extracting the Emailer over gRPC:

**Pros:**

- Isolates SMTP/provider concerns; email can scale and deploy independently
- The app is no longer blocked by mail delivery
- A natural home for durable retries later, without touching use cases (ports unchanged)

**Cons:**

- A network hop and a new failure mode — the app must tolerate the Emailer being
  unavailable (confirmations stay fire-and-forget; release sends still return a
  `BatchResult` over gRPC)
- Another service plus a proto contract to build and operate

The wider three-service split adds further distributed-system complexity (a message
broker, cross-service data consistency, more to operate); those costs are deferred
until that step is actually taken. Revisit and promote this ADR from *Proposed* to
*Accepted* as each step is adopted.
