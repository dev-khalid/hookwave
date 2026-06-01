# Sprint 1 - Event producer + local queue (ElasticMQ)

Estimated: 8-12h. Study first: Study Guide Phase B (JSON/config), Phase C (context/shutdown),
Phase E (AWS SDK v2 - SQS portion).

## Goal & outcome

ElasticMQ runs locally; the `producer` service generates fake order events and sends them to an SQS
queue; you can see the messages arrive in the ElasticMQ web UI. The `queue` package becomes the single,
testable place that knows how to talk to SQS.

## Study first (why)

- AWS SDK v2 config + `BaseEndpoint` override: this is the trick that lets one codebase target both
  ElasticMQ (local) and real SQS (cloud).
- SQS basics: `GetQueueUrl`/`CreateQueue`, `SendMessage`, message bodies and message attributes.
- `time.Ticker` for emitting on an interval; a burst mode for load testing later.
- JSON marshaling of your event structs.

## Build steps (in order)

1. **Run ElasticMQ via a compose file** in `deploy/compose/` (you'll expand this compose file each
   sprint). Expose the SQS API (9324) and the web UI (use 9325 on the host to avoid colliding with
   Grafana's 3000 later). Confirm the UI loads and you can see queues.
2. **Define event types in `internal/events`.** A single `Event` struct with fields like `ID` (UUID),
   `Type` (e.g. `order.created`), `OccurredAt` (time), and a `Data` payload (order id, amount, status).
   Add a constructor and a `MarshalJSON`/helper to serialize. Add a small `[]string` of valid types.
3. **Build the `queue` package** as an interface + an SQS-backed implementation:
   - Define a narrow interface so callers (and tests) don't depend on the SDK directly, e.g.:

```go
type Publisher interface {
    Publish(ctx context.Context, body []byte, attrs map[string]string) error
}
```

   - Implement it with the SQS client. In the constructor, load AWS config with `config.LoadDefaultConfig`,
     then build the SQS client overriding `BaseEndpoint` to the ElasticMQ URL when an env var is set.
   - Add an `EnsureQueue(ctx, name)` that creates the queue if missing and caches its URL.
4. **Read configuration in `internal/config`**: queue name, SQS endpoint, AWS region (a dummy value is
   fine for ElasticMQ), produce interval, and a burst count. Provide sensible defaults; fail fast if a
   truly required value is missing.
5. **Wire `cmd/producer`**: construct config -> queue publisher -> ensure the queue exists -> start a
   ticker loop that builds a random valid event, marshals it, and publishes it. Set the event type as a
   message attribute (you'll use it in Sprint 2 for routing without parsing the body). Respect the
   shutdown context so Ctrl+C stops the loop cleanly.
6. **Add a burst mode** (e.g. `PRODUCE_BURST=1000`) that sends N messages quickly then exits - you'll
   use this to drive KEDA scaling in Sprint 6.
7. **Test the `queue` package logic** that doesn't need AWS (e.g. attribute building, error wrapping).
   The SDK call itself can be covered later with `testcontainers-go` (Nice-to-have).

## Definition of Done

- `make up` (or a compose `up`) starts ElasticMQ; UI reachable.
- Running the producer creates the queue (if missing) and sends messages on the interval.
- You can refresh the ElasticMQ UI and watch the message count climb; inspecting a message shows your
  JSON body and the event-type attribute.
- Ctrl+C stops the producer without panics; in-flight send completes or is cancelled cleanly.
- `make test`, `make lint` still pass.

## Pitfalls

- ElasticMQ needs a region and credentials to satisfy the SDK even though it ignores them; set dummy
  values (e.g. region `elasticmq`, static dummy creds) so config loading doesn't fail.
- The queue URL from ElasticMQ uses the host the server advertises; if you call it from inside compose
  vs from your host, the hostname differs (`elasticmq` vs `localhost`). Keep the endpoint configurable.
- Don't construct a new SQS client per message - build it once and reuse it.
- Always pass `ctx` into SDK calls so shutdown cancels in-flight requests.

## Commit

Commit as "feat(producer): emit fake order events to SQS/ElasticMQ".
