# HW9 — Service Communication: gRPC & buf/proto

The comparison is event-driven (NATS) vs. gRPC. A single env var selects which transport the orchestrator uses at
runtime, proving both coexist:

```
SAGA_COMPENSATE_TRANSPORT=nats   # default — durable outbox command
SAGA_COMPENSATE_TRANSPORT=grpc   # synchronous RPC to subscription-svc
```

## The operation: saga compensation (Orchestrator → Subscription)

When the confirmation email permanently fails (`notifications.confirmation.dead`), the orchestrator must roll back the
pending subscription.

- **NATS.** The coordinator marks the saga `COMPENSATED` and enqueues a `saga.compensate` command into a transactional
  outbox in one DB transaction; a relay publishes it durably; subscription-svc consumes it and runs the `compensate` use
  case. Crash-durable, decoupled, idempotent.
- **gRPC.** The coordinator marks the saga `COMPENSATED`, then makes a synchronous unary RPC to subscription-svc, which
  runs the same `compensate` use case and returns whether a row was deleted.

Both transports drive the identical use case, so the rollback logic and its idempotency live in one
place regardless of transport.

## `proto/compensation/v1/compensation.proto`

```proto
service CompensationService {
  rpc Compensate(CompensateRequest) returns (CompensateResponse);
}

message CompensateRequest {
  string saga_id   = 1 [(buf.validate.field).string.uuid = true];
  string saga_type = 2 [(buf.validate.field).string.min_len = 1];
  string email     = 3 [(buf.validate.field).string.email = true];
  string repo_name = 4 [(buf.validate.field).string.pattern = "^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$"];
}

message CompensateResponse { bool rolled_back = 1; }
```

- Declarative request validation via `buf.validate` (the `protovalidate` rules are enforced by a connect interceptor
  that maps a violation to gRPC `InvalidArgument`).

### buf

`buf` is configured at the repo root with three files:

- `buf.yaml`
- `buf.gen.yaml`
- `buf.lock`

```bash
task proto-lint   # buf lint
task proto        # buf dep update && buf generate
```

Generated stubs land in `internal/shared/grpc/gen/compensationv1/` (gitignored).

## Module boundaries

`depguard` (`.golangci.yml`) forbids the `saga` and `subscription` modules from importing each other,
but both may import `shared`.

| Piece           | Location                                                                    | Why                                                                            |
|-----------------|-----------------------------------------------------------------------------|--------------------------------------------------------------------------------|
| Generated stubs | `internal/shared/grpc/gen/compensationv1`                                   | imports only protobuf/connect, so both modules may use it                      |
| gRPC server     | `internal/subscription/adapter/compensationserver`                          | Must mutate the `subscriptions` table via subscription's `compensate` use case |
| gRPC client     | `internal/saga/adapter/compensationclient`                                  | Lives where it is used; imports only `shared`                                  |
| Strategy wiring | `internal/saga/usecase/coordinator` (`NATSCompensator` / `GRPCCompensator`) | The coordinator depends on a `compensator` port; bootstrap injects one         |

### gRPC status codes

| Situation                                      | Code                                                      |
|------------------------------------------------|-----------------------------------------------------------|
| Malformed request (bad uuid/email/empty field) | `InvalidArgument` (3) — via the protovalidate interceptor |
| Backend/DB failure                             | `Internal` (13)                                           |
| No pending row to delete                       | `OK` (0) with `rolled_back=false` — not an error          |

## Benchmark

`test/benchmark/compensate_bench_test.go` (build tag `benchmark`), run with:

```bash
task bench   # go test -tags benchmark -run '^$' -bench 'Compensate' -benchmem -benchtime=5s -count=5 ./test/benchmark/...
```

It reuses the testcontainers harness (a NATS-only `TestMain` under `test/benchmark/`).

- Both sides do trivial, no-DB handler work. The use case's Postgres delete is stubbed out, so the
  number reflects transport cost, not the database.
- Both are measured end-to-end, single in-flight: gRPC = request → response; NATS = publish →
  consumer-processed.
- Numbers below are five runs at `-benchtime=5s` (`task bench`), not a single short burst.
- Environment: Windows 11, Docker Desktop (Linux VM), 12th-gen i7-12700H.

### Results

The gRPC client is connect-go speaking the gRPC wire protocol over h2c.

|                  | gRPC        | NATS                 |
|------------------|-------------|----------------------|
| latency median   | ~0.28 ms/op | ~1 ms/op → ~43 ms/op |
| allocations / op | ~215 allocs | ~26 allocs           |

### Why these numbers

- gRPC is flat and tight: ~0.28 ms/op across all five runs.
- NATS is bimodal. While the publish → deliver → ack loop keeps up it runs ~1 ms/op; once JetStream's durable write path
  saturates, it jumps to ~43 ms/op. Two things compound.
- gRPC allocates more per call, than a NATS publish. gRPC buys latency at the cost of per-call memory churn.

## Conclusion

- gRPC wins decisively on synchronous request/response latency and stability — flat ~0.28 ms, strongly typed, immediate
  ack.
- NATS trades latency for durability, decoupling, and buffering. Its synchronous latency looks bad
  in this benchmark precisely because synchronous request/response is not its job.

|                                      | gRPC     | NATS                    |
|--------------------------------------|----------|-------------------------|
| caller blocks until compensated?     | yes      | no                      |
| caller latency                       | ~0.28 ms | outbox db enqueue write |
| compensation guaranteed after crash? | no       | yes                     |
| both services must be up?            | yes      | no                      |