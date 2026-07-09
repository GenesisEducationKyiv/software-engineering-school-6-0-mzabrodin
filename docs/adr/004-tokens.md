# ADR-004: Tokens for Email Links

**Status:** Accepted

**Author:** Zabrodin Maksym

## Context

Confirmation and unsubscribe links are sent by email and must be usable from a browser without logging
in. Two options: a random token stored in the DB (revoke by deleting the row, costs a lookup) or a
signed JWT (stateless, no lookup, but carries an expiry and key management).

## Decision

The two links have different lifecycles, so they use different tokens.

- **Confirmation = JWT** (HS256, signed with `JWT_SECRET`). Claims: `email`, `repo`, `exp` (now +
  `CONFIRM_TOKEN_TTL`, default 24h). Issued at subscribe, verified once at confirm — no DB column.
  A confirmation link is naturally short-lived, and statelessness makes re-subscribe write-free:
  reissue a fresh JWT and republish `subscriptions.pending`, no row update; the old JWT simply expires.
- **Unsubscribe = random token.** 32 random bytes, hex-encoded (64 chars), stored as a unique column.
  It must work indefinitely (it ships in every release email), so it should not expire, and revocation
  is a row delete. JWT's expiry would be a liability here.

## Consequences

**Pros:**

- Confirmation needs no token storage or lookup; re-subscribe touches no rows.
- Unsubscribe links never go stale; revocation is a simple delete; 256 bits of entropy resist brute force.

**Cons:**

- `JWT_SECRET` is now a required secret; rotating it invalidates outstanding confirmation links.
- An expired confirmation link forces the user to subscribe again (by design).
