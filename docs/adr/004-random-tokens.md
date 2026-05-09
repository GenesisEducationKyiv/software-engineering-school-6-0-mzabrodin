# ADR-004: Random Tokens for Email Links

**Status:** Accepted

**Author:** Zabrodin Maksym

## Context

Confirmation and unsubscribe links are sent by email. Anyone with the link must be able to use it without logging in.

## Candidates

1. **Random token** — 32 random bytes, hex-encoded, stored in DB
   - Pro: Reveals nothing about the subscription, revoked by deleting the row
   - Con: Requires a DB lookup on every use

2. **JWT** — signed token encoding subscription ID and action, validated without a DB lookup
   - Pro: Stateless
   - Con: Needs key management, leaks subscription ID, requires handling expiry

## Decision

Use two independent 32-byte tokens per subscription, hex-encoded to 64 characters, stored as UNIQUE columns.

## Consequences

**Pros:**
- No expiry — confirmation links don't go stale
- Revocation is a simple row delete
- 256 bits of entropy make brute-force infeasible

**Cons:**
- Every confirmation or unsubscribe request costs a DB lookup
