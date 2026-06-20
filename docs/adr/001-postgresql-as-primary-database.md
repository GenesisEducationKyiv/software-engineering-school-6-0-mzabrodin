# ADR-001: PostgreSQL as the Primary Database

**Status:** Accepted (per-service ownership added by [ADR-012](012-event-driven-services.md))

**Author:** Zabrodin Maksym

## Context

The system needs persistence for relational data: subscriptions belong to repositories, tokens are
unique, and `(email, repository)` pairs are unique. Referential integrity and unique constraints
should be enforced by the store, not the application.

## Decision

Use PostgreSQL with `pgxpool`. Each service owns its own database — no shared tables (see
[ADR-012](012-event-driven-services.md)). Schema migrations are embedded in each binary and applied on
startup via `golang-migrate`.

| Database     | Tables                                                                                                                                                                                                                                                                         |
|--------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| subscription | `repositories{id, name, created_at}`; `subscriptions{id, repository_id→repositories, email, unsubscribe_token, confirmed, created_at}` with `UNIQUE(email, repository_id)` and an index on `email`; `outbox_messages{id, subject, payload, created_at}` (transactional outbox) |
| scanner      | `watched_repos{repo_name PK, last_seen_tag, subscriber_count}`                                                                                                                                                                                                                 |
| notifier     | `subscriptions_read{PK(email, repo_name), unsub_token}` (read model); `failed_notifications` / `failed_confirmations` (retry/dead-letter); `processed_releases{PK(repo_name, tag)}` (dedupe)                                                                                   |

Notes:

- The confirmation token is a stateless JWT ([ADR-004](004-tokens.md)) — there is no
  `confirm_token` column.
- `last_seen_tag` lives on the scanner's `watched_repos` (the scanner owns release tracking), not
  on `repositories`.
- `confirmed` is a boolean — a subscription has exactly two states (pending / confirmed).

## Consequences

**Pros:**

- Unique constraints and foreign keys enforce correctness in the store.
- The email index keeps `GET /api/subscriptions?email=` off a full scan.
- Database-per-service keeps each service independently deployable and lets the schemas evolve apart.

**Cons:**

- Three PostgreSQL instances to run instead of one.
- Subscriber state is replicated across three stores (eventual consistency —
  see [ADR-012](012-event-driven-services.md)).