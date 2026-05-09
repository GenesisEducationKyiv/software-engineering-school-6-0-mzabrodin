# ADR-003: Redis Caching for GitHub API Responses

**Status:** Accepted

**Author:** Zabrodin Maksym

## Context

GitHub API allows 60 requests/hour without a token and 5 000 with one. The service calls GitHub on every subscription (repo existence check) and on every scanner execution. Redis is used to reduce the number of real API calls.

## Decision

Cache GitHub API responses in Redis with a 10-minute TTL using a cache-aside pattern.

| Key                                    | Value                        | Checked when            |
|----------------------------------------|------------------------------|-------------------------|
| `github:repo_exists:{owner}/{repo}`    | `"1"` (exists) / `"0"`       | Every subscribe call    |
| `github:latest_release:{owner}/{repo}` | JSON release data / `"none"` | Every scanner execution |

`"none"` is stored explicitly for repos with no releases. Without it a cache miss would be indistinguishable from "not cached yet," causing an API call on every tick.

On any Redis error the request falls through to GitHub, so a Redis outage increases API traffic but does not stop the service.

## Consequences

**Pros:**
- The same repo costs one GitHub API call per 10 minutes regardless of request volume
- Redis failure degrades performance, not availability

**Cons:**
- A repo that just published its first release may not be noticed for up to 10 minutes (while `"none"` is still cached)
