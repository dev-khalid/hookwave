# Sprint 2 - Processor consumes queue + matches subscriptions

Estimated: 10-14h. Study first: Study Guide Phase C (concurrency, context, channels, graceful
shutdown - this is the heaviest concurrency sprint), Phase B (JSON config), Phase E (SQS receive/delete).

## Goal & outcome

The `processor` service long-polls the queue, parses each event, loads `subscriptions.json`, figures out
which subscribers want that event type, logs the intended deliveries, and then deletes (acks) the message.
No HTTP delivery yet - that's Sprint 3. This isolates the consume + routing logic so you can get the
queue lifecycle right before adding network calls.

## Study first (why)

- SQS receive lifecycle: long polling (`WaitTimeSeconds`), batch receive, _visibility timeout_, and why
  you delete only after success. This is the core of at-least-once delivery.
- `context` cancellation in a long-running loop; `select` between "got messages" and "shutdown".
- Loading and validating JSON into typed structs.
- (helpful) `errgroup` or a worker pool if you want concurrent processing of a batch.

**Decision:** we load subscription data from a generated `configs/subscriptions.json`, not a
hand-written `subscriptions.yaml`. `cmd/subscriber` has a `GENERATE_SUBSCRIPTIONS=N` mode
(`make generate-subscriptions`) that writes N random subscriptions (`events`, `url`, `company_id`)
to that file via `encoding/json` - no YAML dependency needed.

## Build steps (in order)

1. **Extend the `queue` package** with the consumer side. Add to (or alongside) your interface:

```go
type Message struct {
    ReceiptHandle string
    Body          []byte
    Attributes    map[string]string
}

type Consumer interface {
    Receive(ctx context.Context, max int) ([]Message, error)
    Delete(ctx context.Context, receiptHandle string) error
}
```

- Implement with SQS `ReceiveMessage` (set `WaitTimeSeconds` for long polling, `MaxNumberOfMessages`,
  and request message attributes) and `DeleteMessage`.

2. **Model subscriptions in `internal/subscriptions`.** A `Subscription` struct (`Events []string`,
   `URL string`, `CompanyID int` - matching the shape `cmd/subscriber` generates) and a `Registry`
   that loads `subscriptions.json` and offers `Match(eventType string) []Subscription`. Validate on
   load (non-empty URL, known event types).
3. **Load the config in `internal/config`** (path to the generated subscriptions file, receive batch
   size, poll wait seconds) and have the processor read the registry at startup.
4. **Write the consume loop in `cmd/processor`:**
   - Loop until the shutdown context is cancelled.
   - Each iteration: `Receive` a batch; for each message, parse the event (use the type attribute, fall
     back to parsing the body), call `registry.Match`, and log one line per intended delivery
     (subscriber url + company id + event id).
   - On successful handling of a message, `Delete` it. On parse failure, decide: log + delete (drop) or
     leave for redelivery. Document your choice.
   - Use `select` so a long poll doesn't block shutdown beyond the poll wait.
5. **Decide concurrency.** Start sequential (simplest, correct). Once green, optionally process a batch
   concurrently with a small worker pool / `errgroup`, making sure each message is deleted only after its
   own success. Keep it correct before making it fast.

## Definition of Done

- With the producer running, the processor logs matched deliveries for each event and the ElasticMQ
  queue depth stays near zero (messages are being consumed and deleted).
- Stopping the producer drains the queue to empty; the processor then idles on long poll.
- Ctrl+C stops the processor mid-poll within ~the poll wait, with no lost or double-deleted messages in
  the happy path.
- `make lint` passes.

## Pitfalls

- **Deleting before processing** breaks at-least-once - delete only after success.
- Forgetting to request message attributes means your type attribute is empty; either request attributes
  or parse the body.
- A `Receive` with no messages is normal during idle - don't treat it as an error or busy-loop; long
  polling already paces you.
- Visibility timeout too short -> the same message gets processed twice while you're still working on it.
  Understand the default and set it deliberately.
- Blocking shutdown: if you call `Receive` with a 20s wait and ignore ctx, Ctrl+C feels frozen. Honor ctx.

## Commit

Commit as "feat(processor): consume SQS and route events to matching subscriptions".
