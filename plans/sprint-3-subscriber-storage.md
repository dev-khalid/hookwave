# Sprint 3 - Subscriber endpoint + HTTP delivery + S3/MinIO storage

Estimated: 10-14h. Study first: Study Guide Phase D (net/http server + client), Phase E (S3 portion).

## Goal & outcome

A full end-to-end flow: producer -> queue -> processor -> **HTTP delivery** -> `subscriber` endpoint ->
**stores payload to MinIO**. The processor now actually sends events; the subscriber receives them and
writes each payload as an object in a bucket. You can open the MinIO console and see stored objects.

## Study first (why)

- `net/http` server: `http.ServeMux`, method-based routes, an `http.Server` with read/write timeouts.
- `net/http` client: `http.Client` with a timeout, constructing a request with a body and headers,
  and always closing/draining the response body.
- Webhook receiver semantics: return 2xx fast, be idempotent (you may get duplicates), validate input.
- S3 with MinIO: `UsePathStyle = true`, `BaseEndpoint` override, `CreateBucket`, `PutObject`, object keys.

## Build steps (in order)

1. **Add MinIO to the compose file** (API 9000, console 9001) with root creds via env. Confirm the
   console loads and you can create a bucket manually once to understand the model.
2. **Build the `storage` package** as an interface + S3 implementation:

```go
type ObjectStore interface {
    Put(ctx context.Context, key string, body []byte, contentType string) error
}
```

- Construct the S3 client with `BaseEndpoint` (MinIO URL) and `o.UsePathStyle = true`.
- Add `EnsureBucket(ctx, name)`; ignore "already exists" errors.

3. **Build the `subscriber` service (`cmd/subscriber`)**:
   - HTTP server with a route like `POST /webhooks`.
   - Handler: read the body (limit its size), optionally validate it's JSON, build an object key
     (e.g. `events/{type}/{eventID}.json` - derive from headers/body), call `storage.Put`, then return 200. Return 4xx on bad input, 5xx only on real server errors.
   - Use `internal/httpx` for shared server setup (timeouts, logging middleware) so all HTTP servers
     look the same.
   - Graceful shutdown: `server.Shutdown(ctx)` on signal.
4. **Add an HTTP delivery client to the processor.** Create a `Deliverer` (in `internal/httpx` or a new
   `internal/delivery`) with `Deliver(ctx, sub Subscription, event []byte) error` that:
   - builds a request to `sub.URL` using `sub.Method`,
   - sets headers (content-type, an event-id header, an event-type header),
   - uses a shared `http.Client` with a timeout,
   - treats 2xx as success; non-2xx or transport error as failure (return an error).
5. **Wire processor delivery (replaces Sprint 2's log-only step):** for each matched subscription,
   call `Deliver`. Decide the ack rule: delete the SQS message only if all deliveries for it succeeded;
   otherwise leave it for redelivery (this is your at-least-once behavior, retries/DLQ come in Nice-to-have).
   Document this rule in code comments.
6. **Point `subscriptions.json` at the subscriber.** Re-run `make generate-subscriptions` (or
   `GENERATE_SUBSCRIPTIONS=N go run ./cmd/subscriber`) so at least one generated entry's `url` is the
   subscriber service (`http://subscriber:8080/webhooks` in compose, `http://localhost:PORT/...` when
   running locally) and `events` lists the types you produce. This file is generated, not hand-edited -
   see the Sprint 2 decision note: subscriptions load from `configs/subscriptions.json`, not a
   `subscriptions.yaml`.

## Definition of Done

- With producer + processor + subscriber + ElasticMQ + MinIO running, events flow all the way through.
- The MinIO console shows objects appearing under your bucket with sensible keys; downloading one shows
  the original event JSON.
- Killing the subscriber causes deliveries to fail, the processor does NOT delete those messages, and
  when the subscriber comes back the messages are redelivered and stored (demonstrating at-least-once).
- `make lint` passes.

## Pitfalls

- Not closing `resp.Body` leaks connections; always `defer resp.Body.Close()` and drain it.
- MinIO without `UsePathStyle = true` fails with virtual-host-style addressing errors.
- No client timeout -> a hung subscriber hangs the whole processor. Always set a timeout.
- Returning 5xx for client-side bad data causes pointless redelivery storms; map errors to the right code.
- Inside compose, services talk via service names and container ports, not `localhost` - keep URLs configurable.

## Commit

Commit as "feat: end-to-end HTTP delivery to subscriber with S3/MinIO storage".
