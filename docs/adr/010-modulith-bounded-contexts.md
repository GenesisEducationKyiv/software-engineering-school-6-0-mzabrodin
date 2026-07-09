# ADR-010: Modulith — Vertical Slices by Bounded Context

**Status:** Accepted.

**Author:** Zabrodin Maksym

## Context

The single binary was originally sliced horizontally by technical role
(`internal/{entity,usecase,adapter,infrastructure}`), so one feature was smeared across five top-level
folders. The hexagonal architecture ([ADR-008](008-hexagonal-architecture.md)) already isolates concerns
behind ports, but the directory layout did not reflect that — there was no structural unit that
corresponded to a future deployable service.

The aim is to realize the premise that "ports already isolate these concerns" structurally, so that
extracting a microservice later is a drop-in adapter swap behind an existing port rather than a
rewrite. That extraction is now done: all three modules run as independent, event-driven services
([ADR-012](012-event-driven-services.md)).

## Decision

Reorganize the single binary into a modulith — vertical slice by bounded context. The layout is now
`internal/{subscription,scanner,notifier}` modules over a `shared` kernel (cross-context domain types and
the GitHub client) and domain-agnostic `infrastructure`. The clean-arch layers
([ADR-008](008-hexagonal-architecture.md)) are preserved inside each module.

### Domain-type placement (usage-driven)

The shared kernel holds only what is genuinely cross-module; everything else lives in its owning
module's `domain` package:

- `shared/domain` — only `Release` (returned by `shared/github`, used by scanner and notifier) and the
  generic sentinel errors (`ErrNotFound`, `ErrAlreadyExists`).
- `shared/github` — the GitHub REST client, github-specific errors, and `ParseRepo`.
- `subscription/domain` — `Repository`, `Subscription` (+ `NewSubscription`), `SubscriptionView`, and
  the confirm/removed/expired result types.
- `scanner/domain` — `WatchedRepo`, `ObservedRelease`.
- `notifier/domain` + `notifier` — the read-model, retry, and `ReleaseNotification` / `BatchResult`
  types.

(All `domain` packages — shared and per-module — are package `domain`; a file importing both aliases
the shared one as `shareddomain`.)

## Key properties

These are what made the split a drop-in:

- No module imports another. They communicate through consumer-defined ports wired only in the
  composition roots (`internal/bootstrap/<service>`, `cmd/<service>`). `shared` and `infrastructure`
  never import a module. These boundaries are enforced in CI by golangci-lint `depguard`.
- Each cross-module edge was a consumer-defined port. Splitting a module into its own service meant
  re-satisfying that port with a NATS adapter (`eventpublisher` / `eventconsumer`) instead of an
  in-process call — no business logic changed. The in-process scan-and-notify ports became the
  `subscriptions.*` / `releases.*` event flow (see [ADR-012](012-event-driven-services.md)).
- Each module owns its tables and the `domain` types it maps; the shared kernel owns only the
  genuinely cross-module `Release` + sentinels.

## Consequences

**Pros:**

- Module independence is checked, not merely conventional.
- Each cross-module edge is a consumer-defined port that can be re-satisfied by different transport
  without touching use cases.
- A feature now lives in one place, not scattered across five technical folders.

**Cons:**

- A single Go module does not enforce boundaries on its own — any package *can* import any
  `internal/...` path. The `depguard` rules in `.golangci.yml` are what make the independence real, and
  they must be maintained as modules are added or split.
- The services still share one Go module, so the shared kernel (`shared/domain`, `shared/github`) and
  `infrastructure` are compiled into all three binaries. Cross-service ports are now event-fed read
  models, not RPC — which trades the old in-process consistency for eventual consistency
  (see [ADR-012](012-event-driven-services.md)).
