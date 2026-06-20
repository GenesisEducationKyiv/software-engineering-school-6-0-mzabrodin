# ADR-011: Unify REST and gRPC on connect-go and Vanguard

**Status:** Accepted

**Author:** Zabrodin Maksym

## Context

The public API originally ran two handwritten adapters: a Chi REST adapter (port 8080) and a grpc-go
adapter (port 50051), each with its own handler methods, validation, error mapping, auth and
observability. Every change meant editing both, and the surfaces had drifted apart — REST returned
`{"message": ...}`, gRPC returned empty bodies.

We want one definition and one handler, while keeping the existing RESTful URLs
(notably the browser-opened `GET /api/confirm/{token}` and
`GET /api/unsubscribe/{token}` links produced by `urlbuilder`).

## Candidates

1. **Keep the two adapters** — status quo; ongoing double maintenance and drift.
2. **connect-go alone** — one handler, but Connect's JSON is RPC-style
   (`POST /app.v1.SubscriptionService/Confirm`), not REST; it would break the
   emailed confirm/unsubscribe links and the documented REST shape.
3. **connect-go + Vanguard transcoding** — one connect handler; Vanguard reads
   `google.api.http` annotations and transcodes the existing REST URLs onto it,
   while also serving Connect, gRPC and gRPC-Web.

## Decision

Replace both adapters with a single connect-go handler
(`internal/subscription/adapter/connectrpc`) generated from
`proto/subscription/v1/subscription.proto`, fronted by Vanguard
(`connectrpc.com/vanguard`). One `http.Server` serves the whole public surface on
one port over h2c (cleartext HTTP/2), via Go 1.26's
`http.Server.Protocols` (`SetUnencryptedHTTP2`); `GRPC_PORT` is removed.

- REST shape preserved by `google.api.http` annotations on each RPC
  (`/api/subscribe`, `/api/confirm/{token}`, `/api/unsubscribe/{token}`,
  `/api/subscriptions?email=`). `urlbuilder` and the emailed links are unchanged.
- Response messages gained a `message` field, so REST and gRPC
  return the same body; entity errors map to a single `*connect.Error` table that
  Vanguard turns into the matching HTTP status for REST callers.
- Validation consolidated on protovalidate (one connection interceptor). The
  proto's repo pattern is the same regex `github.ParseRepo` uses, so there is no
  behavior change; violations are remapped to the previous friendly messages.
- Thin HTTP-edge middleware tags each request's wire protocol
  into the context; a single connection interceptor
  emits one request log and one `requests_total{protocol,procedure,code}` metric.
- The gRPC health service and server reflection are
  served by `connectrpc.com/grpchealth` and `connectrpc.com/grpcreflect`; a plain
  HTTP `/health` and Prometheus `/metrics` stay on the same mux.

## Consequences

**Pros:**

- One handler, one validation path, one error table, one observability path.
- REST and gRPC are consistent by construction (same proto, same handler).
- A single port and a single graceful-shutdown path.

**Cons:**

- REST error bodies change to Connect's status JSON, and `GET /api/subscriptions`
  returns the proto-wrapped `{"subscriptions": [...]}` rather than a bare array.

This decision covers the public API only. Protobuf is used just here; inter-service
communication moved to JSON events over NATS ([ADR-012](012-event-driven-services.md)), so there is no
internal gRPC or mTLS anywhere in the system.
