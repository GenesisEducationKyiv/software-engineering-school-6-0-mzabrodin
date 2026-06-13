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
extracting a microservice later (the direction in [ADR-009](009-microservices-split.md)) is a
drop-in adapter swap behind an existing port rather than a rewrite — exactly how `cmd/emailer` was split
out.

## Decision

Reorganize the single binary into a modulith — vertical slices by bounded context. The layout is now
`internal/{subscription,scanner,notifier}` modules over a `shared` kernel (cross-context domain types and
the GitHub client) and domain-agnostic `infrastructure`. The clean-arch layers
([ADR-008](008-hexagonal-architecture.md)) are preserved inside each module.

### Entity placement (usage-driven)

The shared kernel holds only what is genuinely cross-context — verified by grepping each type's call
sites across the would-be modules:

- `shared/entity` — types used by all three modules: `Repository`, `Subscription`,
  `Release`, their constructors (`NewRepository`, `NewSubscription`), and the generic sentinel errors
  (`ErrNotFound`, `ErrAlreadyExists`). Only external dependency is `uuid`.
- `shared/github` — the GitHub REST client, github-specific errors, and `ParseRepo` (used by both
  subscribe and scanner).
- `subscription/domain` — `SubscriptionView`, the one type used by the subscription module alone.
- `notifier` — the `ReleaseNotification` / `BatchResult` DTOs (and `NewReleaseNotification`),
  used only by the notifier module's adapters.

## Key properties

These are what make the future split a drop-in:

- No module imports another. They communicate through consumer-defined ports
  wired only in the composition roots (`cmd/server`, `cmd/emailer`). `shared` and `infrastructure` never
  import a module. These boundaries are enforced in CI by golangci-lint `depguard`.
- The scanner depends only on its own
  `gitHubRepoRepository` / `subscriptionRepository` port, satisfied
  today by subscription's in-process repositories. Extracting the scanner = swap that port's adapter for
  a gRPC client — the same move that turned the in-process mailer into `notifierclient`. No scanner
  business logic changes.
- The repository adapters (`GitHubRepoRepository`, `SubscriptionRepository`)
  live in the subscription module (it owns the tables); the entity types they map live in the `shared`
  because the scanner and notifier also speak them. The module owns the noun's logic; the kernel
  owns the noun.

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
- At a real process split, in-process shared code (`shared/entity`, `shared/github`) is vendored per
  service or becomes proto-defined; request/response ports either stay RPC or become an event-fed read
  model (eventual consistency). These are deferred costs, recorded for when each split is taken.
