# Sprint 1 - Event producer + SQS publisher

Estimated: 8-12h. Study first: Study Guide Phase B (JSON/config), Phase C (context/shutdown),
Phase E (AWS SDK v2 - SQS portion).

## Goal & outcome

The `producer` service generates fake order events and publishes them to SQS using the AWS SDK. In local
development, the SDK is pointed at a local SQS-compatible endpoint, but the application code should not
care whether that endpoint is local infrastructure or real AWS SQS.

The `queue` package becomes the single place that knows how to talk to SQS. Everything outside that
package should depend on a narrow publisher interface, not on the AWS SDK and not on any local queue
implementation.

## Study first (why)

- AWS SDK v2 config + optional `BaseEndpoint` override: this is the trick that lets one codebase target
  both local SQS-compatible infrastructure and real SQS without changing application code.
- SQS basics: `GetQueueUrl`/`CreateQueue`, `SendMessage`, message bodies and message attributes.
- `time.Ticker` for emitting on an interval; a burst mode for load testing later.
- JSON marshaling of your event structs.

## Build steps (in order)

1. **Provide a local SQS-compatible endpoint** via a compose file in `deploy/compose/` (you'll expand
   this compose file each sprint). The app should receive normal SQS configuration: queue name, AWS
   region, credentials, and optionally an endpoint override. The producer should not contain branches or
   concepts specific to the local implementation.
2. **Define event types in `internal/events`.** A single `Event` struct with fields like `ID` (UUID),
   `Type` (e.g. `order.created`), `OccurredAt` (time), and a `Data` payload (order id, amount, status).
   Add a constructor and a `MarshalJSON`/helper to serialize. Add a small `[]string` of valid types.
3. **Build the `queue` package** as an interface + an SQS-backed implementation:
   - Define a narrow interface so callers don't depend on the SDK directly, e.g.:

```go
type Publisher interface {
    Publish(ctx context.Context, body []byte, attrs map[string]string) error
}
```

- Implement it with the SQS client. In the constructor, load AWS config with `config.LoadDefaultConfig`,
  then build the SQS client with an optional `BaseEndpoint` override when an env var is set.
- Add an `EnsureQueue(ctx, name)` that creates the queue if missing and caches its URL.

4. **Read configuration in `internal/config`**: queue name, SQS endpoint, AWS region (a dummy value is
   fine for local development), produce interval, and a burst count. Provide sensible defaults; fail fast
   if a truly required value is missing.
5. **Wire `cmd/producer`**: construct config -> queue publisher -> ensure the queue exists -> start a
   ticker loop that builds a random valid event, marshals it, and publishes it. Set the event type as a
   message attribute (you'll use it in Sprint 2 for routing without parsing the body). Respect the
   shutdown context so Ctrl+C stops the loop cleanly.
6. **Add a burst mode** (e.g. `PRODUCE_BURST=1000`) that sends N messages quickly then exits - you'll
   use this to drive KEDA scaling in Sprint 6.

## Definition of Done

- `make up` (or a compose `up`) starts a local SQS-compatible endpoint; optional UI reachable if the
  chosen local tool provides one.
- Running the producer creates the queue (if missing) and sends messages on the interval.
- You can verify messages are being sent; inspecting a message shows your JSON body and the event-type
  attribute.
- Ctrl+C stops the producer without panics; in-flight send completes or is cancelled cleanly.
- `make lint` passes.

## Pitfalls

- Local SQS-compatible tools still need a region and credentials to satisfy the SDK even if they ignore
  them; set dummy values so config loading doesn't fail.
- The queue URL may use a different hostname depending on whether you call it from inside compose or from
  your host. Keep the endpoint configurable.
- Don't construct a new SQS client per message - build it once and reuse it.
- Always pass `ctx` into SDK calls so shutdown cancels in-flight requests.

## Commit

Commit as "feat(producer): emit fake order events to SQS".
