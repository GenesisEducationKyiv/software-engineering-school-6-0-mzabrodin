# ADR-005: gRPC API Alongside the REST API

**Status:** Superseded by [ADR-011](011-connect-transcoding.md)

**Author:** Zabrodin Maksym

## Context

The service exposes its functionality over an HTTP/JSON REST API. We want a
second, contract-first transport – gRPC with Protocol Buffers – offering the
same operations (subscribe, confirm, unsubscribe, list subscriptions) for
clients that prefer generated stubs and a strongly typed schema.

The use-case layer (`internal/usecase/*`) is already transport-agnostic: each
operation is an `Execute(ctx, In) (Out, error)` wrapped by `metrics.NewMetered`.
The HTTP handler consumes those use cases purely through interfaces, so a gRPC
handler can reuse the exact same instances.

## Candidates

1. **Replace REST with gRPC** – single transport
   - Con: breaks existing HTTP clients and the browser-opened confirm/unsubscribe
     links, which cannot easily speak gRPC

2. **gRPC-gateway (REST transcode from gRPC)** – one definition, two surfaces
   - Con: couples the existing, already-shipped REST shape to the proto contract
     and adds a transcoding layer

3. **gRPC on a separate port, reusing the use cases** – both transports, independent
   - Pro: REST is untouched; both share business logic and metrics
   - Con: two listeners and two shutdown paths to manage

## Decision

Run gRPC on its own port (`GRPC_PORT`, default `50051`) in a parallel goroutine,
reusing the same metered use cases as the HTTP server. `PORT` is renamed to
`HTTP_PORT` for symmetry.

- **Definition:** `proto/notifier/v1/subscription.proto`, generated with **buf**
  (`make proto`). Generated code lives under `internal/adapter/grpc/gen/` and is
  gitignored.
- **Validation:** declarative `buf.validate` rules in the proto, enforced at
  runtime by a **protovalidate** interceptor `InvalidArgument`.
- **Auth:** an interceptor checks `x-api-key` metadata on the same operations the
  REST API protects (subscribe, list), mirroring the HTTP API-key middleware.
- **Errors:** domain sentinels map to gRPC status codes, mirroring the HTTP error
  table (`ErrNotFound` to `NotFound`, `ErrAlreadyExists` to `AlreadyExists`, etc.).
- **Responses:** write RPCs return empty messages; success is the `OK` status.
- **Extras:** the standard health service, server reflection, plus metrics and
  logging interceptors parallel to the HTTP middleware.

## Consequences

**Pros:**
- REST clients are unaffected; gRPC clients get a typed, generated contract
- Business logic, metrics, and auth semantics are shared across both transports
- Validation lives in the schema, close to the contract

**Cons:**
- Two listeners and a slightly more involved graceful shutdown
- A code-generation step (`buf`) is now part of the build; CI must run it because
  generated code is not committed
- The proto contract must be evolved compatibly (buf breaking-change checks help)
