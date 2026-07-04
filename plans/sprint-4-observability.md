# Sprint 4 - Observability (OpenTelemetry + Grafana)

Estimated: 16-24h (expanded scope: queue-depth gauges + full per-stage failure breakdown).
Study first: Study Guide Phase I (OpenTelemetry concepts + Go SDK wiring).

## Goal & outcome

All three services emit OpenTelemetry **traces**, **metrics**, and **logs** to the `grafana/otel-lgtm`
container. In Grafana you can follow a single event from producer -> processor -> subscriber as one
connected trace, drill into *which stage* was slow or failing (queue wait vs subscription lookup vs fake
API call vs DB update vs webhook delivery vs S3 write), watch queue depth and in-flight/DLQ backlog live,
jump from a trace straight to its correlated log lines in Loki, and read throughput/latency/error
dashboards. This is how you answer "where is the bottleneck?", "is the queue backing up?", and "show me
everything that happened around this failed delivery" - all without grepping logs.

## Metrics catalog (granular - maps directly to your observability checklist)

Every row below is a "Time vs Line/heatmap chart" in the final Grafana dashboard (step 11). Counters and
histograms come from OTel `Meter` instruments exported via OTLP to Prometheus; gauges marked
**(polled)** are `ObservableGauge` callbacks reading live SQS state, not derived from spans.

| # | Your checklist item | Metric name | Type | Labels (bounded) | Emitted from |
|---|---|---|---|---|---|
| 1 | Latency on event publishing to a queue | `queue_publish_duration_seconds` | Histogram | `event_type` | producer, wrapping `client.Publish` |
| 2 | Available number of items in a queue | `queue_messages_visible` **(polled)** | Gauge | `queue_name` (`webhook-events`, `webhook-events-dlq`) | new small poller (step 7) calling `GetQueueAttributes` |
| 3 | In-flight queue items | `queue_messages_in_flight` **(polled)** | Gauge | `queue_name` | same poller, `ApproximateNumberOfMessagesNotVisible` |
| 3b | *(extension)* Delayed queue items | `queue_messages_delayed` **(polled)** | Gauge | `queue_name` | same poller, `ApproximateNumberOfMessagesDelayed` - free from the same API call, useful once redrive backoff is in play |
| 4 | Successfully processed queue items | `messages_processed_total{status="success"}` | Counter | `status`, `event_type` | processor, end of `processEvent` |
| 5 | Failed queue items | `messages_processed_total{status="error"}` | Counter | `status`, `event_type`, `failure_stage` | processor, every error return path in `processEvent` |
| 5b | *(extension)* Items moved to the DLQ | `messages_moved_to_dlq_total` | Counter | `event_type` | processor - increment when a receive shows `ApproximateReceiveCount == DefaultMaxReceiveCount` right before SQS redrives it (best-effort local signal; the DLQ gauge in #2 is the source of truth) |
| 6 | Average processing time per item | derived from `message_processing_duration_seconds` (Prometheus `rate(sum)/rate(count)`) | - | - | dashboard panel, no new instrument |
| 7 | Processing time distribution per item (poll -> success/failure) | `message_processing_duration_seconds` | Histogram | `status` | processor, spans `processor.handle_message` start (right after `Receive`) to terminal Delete/DLQ-eligible-error |
| 7b | *(extension)* End-to-end latency (publish -> terminal), distinct question from #7 | `message_end_to_end_latency_seconds` | Histogram | `status` | processor, using the SQS `SentTimestamp` attribute (or the producer's injected publish-time trace attribute) minus `time.Now()` at terminal state |
| 8a | Breakdown: subscription/webhook-details lookup | `subscription_match_duration_seconds` / `subscription_match_total{result="matched"\|"unmatched"}` | Histogram / Counter | `event_type`, `result` | processor, new `processor.match_subscriptions` span around `registry.Match` |
| 8b | Breakdown: fake upstream API call | `fake_api_call_duration_seconds` (existing) + `fake_api_call_total{status="success"\|"timeout"\|"error"}` (extension - was timeout-only) | Histogram / Counter | `status` | processor, `processor.fake_api_call` span |
| 8c | Breakdown: DB update | `fake_db_update_duration_seconds` (existing) + `fake_db_update_total{status="success"\|"error"}` (extension) | Histogram / Counter | `status`, `event_type` | processor, `processor.fake_db_update` span |
| 8d | Breakdown: webhook delivery (HTTP to subscriber) | `webhook_delivery_duration_seconds` + `webhook_delivery_total{status="success"\|"error", http_status_class="2xx"\|"4xx"\|"5xx"\|"timeout"}` | Histogram / Counter | as listed | processor, new `processor.deliver_webhook` span around `Deliverer.Deliver` (Sprint 3) |
| 8e | Breakdown: S3 storage | `s3_store_duration_seconds` + `s3_store_total{status="success"\|"error"}` | Histogram / Counter | `status` | subscriber, `subscriber.store` span around `storage.Put` (Sprint 3) |

`failure_stage` (row 5) is a closed enum: `decode`, `match_subscriptions`, `fake_api_call`,
`fake_db_update`, `deliver_webhook`. It lets one top-level "why did this fail" panel slice by stage
without joining five separate counters; the per-stage counters in 8a-8e exist for drilling into *that
stage's own* success/latency profile once you already know which stage is the culprit.

**Why both #7 and #7b:** #7 answers "is my processor slow" (receive-to-terminal, what you control).
#7b answers "is the event slow from the customer's perspective" (publish-to-terminal, includes queue
wait time). They will diverge under load - that divergence *is* queue wait time, and is worth its own
derived panel (`message_end_to_end_latency_seconds` avg minus `message_processing_duration_seconds` avg).

## Study first (why)

- Trace vs span vs context propagation: a trace is the whole journey; spans are steps; propagation is how
  the trace id travels across SQS and HTTP so the steps link up.
- `TracerProvider` / `MeterProvider` / `LoggerProvider`, the OTLP exporter, resource attributes
  (`service.name`), and clean `Shutdown` to flush on exit.
- **Async instruments**: `Meter.Int64ObservableGauge` + `meter.RegisterCallback` - the pattern for
  metrics that reflect external state (queue depth) rather than something your code did (a counter/histogram
  on a code path). This is new territory beyond the original sprint scope - read the OTel Go metrics docs
  section on asynchronous instruments before step 7.
- Propagation carriers: HTTP headers (standard) and SQS message attributes (manual carrier).
- Log-trace correlation: why a log line needs `trace_id`/`span_id` fields to be useful, and why that means
  `ctx` (the thing carrying the active span) has to reach every function that logs.
- SQS `GetQueueAttributes`: which attribute names give visible/in-flight/delayed counts, and that these are
  **approximate** (SQS eventual consistency) - don't expect exact numbers, expect trends.

## Build steps (in order)

1. **Add `grafana/otel-lgtm` to the compose file.** Map Grafana 3000, OTLP gRPC 4317, OTLP HTTP 4318.
   Open Grafana (admin/admin) and confirm Tempo, Prometheus, and Loki data sources exist.
2. **Build OTel setup in `internal/observability`.** Add `SetupTelemetry(ctx, service string)` that:
   - configures a `Resource` with `service.name` (and `service.version`/`deployment.environment` if you
     want to distinguish local vs later environments),
   - creates OTLP exporters (trace, metric, **and log**) pointing at the collector (endpoint from env,
     default the otel-lgtm host),
   - installs a `TracerProvider`, a `MeterProvider`, **and a `LoggerProvider`**, and sets the global
     propagators (tracecontext + baggage),
   - returns a `shutdown(ctx)` func that flushes/closes all three providers **in order: logs, then
     traces, then metrics** (flush logs last-mile info before the traces that reference it disappear;
     order metrics last since they're least likely to need the others), wrapped in a bounded
     `context.WithTimeout` so a dead collector can't hang shutdown forever.
     Call it from all three `main.go` files; defer the shutdown.
3. **Bridge the existing zap logger into OTel Logs.** In `internal/observability/logger.go`, add a second
   `zapcore.Core` built from `go.opentelemetry.io/contrib/bridges/otelzap` (`otelzap.NewCore(service,
   otelzap.WithLoggerProvider(provider))`), and combine it with the current stdout JSON core via
   `zapcore.NewTee(stdoutCore, otelCore)`. Every existing `logger.Info(...)`/`logger.Error(...)` call now
   goes to both stdout (for `docker compose logs`) and, via OTLP, to Loki - no call-site changes needed for
   the bridge itself (see step 4 for the correlation piece, which does need call-site changes).
4. **Add `observability.FromContext(ctx, logger) *Logger` for trace-log correlation.** It should read
   `trace.SpanContextFromContext(ctx)` and, if a span is active, return `logger.With("trace_id",
   sc.TraceID().String(), "span_id", sc.SpanID().String())`; otherwise return `logger` unchanged. This is
   an explicit, visible correlation step (not implicit bridge magic) so it's obvious at each call site
   whether a log line is trace-correlated. **This requires threading `ctx` into every function that logs**
   -  note `fakeDbUpdate` doesn't take `ctx` today; add it here.
5. **Fix `fakeApiCall`'s context bug before instrumenting it.** It currently calls
   `context.WithTimeout(context.Background(), DefaultTimeout)`, discarding the caller's `ctx` entirely -
   this breaks both cancellation (a SIGTERM won't stop an in-flight fake call) and, once you add spans,
   trace propagation (a span started from `context.Background()` has no parent). Change it to
   `context.WithTimeout(ctx, DefaultTimeout)` and add `ctx` as its first parameter.
6. **Instrument the producer:**
   - start a span `producer.publish_event` around the publish call and **inject** the trace context into
     SQS message attributes (write a small carrier that maps the propagator to the attributes map),
   - record `queue_publish_duration_seconds` (histogram, labeled `event_type`) around the same call,
   - add a counter for events produced, labeled `event_type` (existing scope, unchanged).
7. **Add a queue-depth exporter (new - covers checklist items 2, 3, 3b, and the DLQ gauge from 5b).**
   In `internal/queue` (or a small new `internal/queue/metrics.go`), register an OTel callback via
   `meter.RegisterCallback` that, every poll interval (default 15s, configurable - don't poll faster than
   a few seconds; SQS attribute reads are eventually consistent and this is a background loop, not a hot
   path), calls `GetQueueAttributes` for **both** the main queue and the DLQ with attribute names
   `ApproximateNumberOfMessages`, `ApproximateNumberOfMessagesNotVisible`,
   `ApproximateNumberOfMessagesDelayed`, and sets `queue_messages_visible`, `queue_messages_in_flight`,
   `queue_messages_delayed` (each labeled `queue_name`). Run this from the processor's `main.go` (it's the
   long-lived consumer-side service) so it starts/stops with the same lifecycle and shuts down cleanly on
   SIGTERM.
8. **Instrument the processor with the full per-message span tree**, not just one span per message:
   - `queue.receive` - wraps the `ReceiveMessages` call (one span per batch poll, not per message, since
     it's a shared network call). Histogram: `queue_receive_duration_seconds`.
   - `processor.handle_message` - the parent span for one message, started right after
     `extract`-ing the trace context from the message attributes so it's a **child of the producer span**.
     Start your `message_processing_duration_seconds` and `message_end_to_end_latency_seconds` timers here
     (the latter using the message's `SentTimestamp` attribute as its start point, not `time.Now()`).
   - `processor.match_subscriptions` - child span wrapping the subscription registry `Match` call
     (Sprint 2). Histogram: `subscription_match_duration_seconds`. Counter:
     `subscription_match_total{result=matched|unmatched}` - an `unmatched` result is not an error (don't
     count it in `messages_processed_total{status=error}`), but it's operationally useful to see if it
     spikes (e.g. a new event type shipped with no subscribers yet).
   - `processor.fake_api_call` - child span wrapping `fakeApiCall`. Histogram:
     `fake_api_call_duration_seconds`. Counters: `fake_api_call_timeouts_total` (existing) and the fuller
     `fake_api_call_total{status=success|timeout|error}`.
   - `processor.fake_db_update` - child span wrapping `fakeDbUpdate`. Histogram:
     `fake_db_update_duration_seconds`. Counter: `fake_db_update_total{status=success|error}`.
   - `processor.deliver_webhook` - child span wrapping the Sprint 3 `Deliverer.Deliver` call, one per
     matched subscription. Histogram: `webhook_delivery_duration_seconds`. Counter:
     `webhook_delivery_total{status=success|error, http_status_class=2xx|4xx|5xx|timeout}` - derive
     `http_status_class` from the response status (bounded to 4 values, never the raw status code as a
     label).
   - On any error in the chain: `span.RecordError(err)` + `span.SetStatus(codes.Error, msg)` on the
     relevant span, **and** log via `observability.FromContext(ctx, logger).Error(...)` so the failure is
     visible in both Tempo and Loki, correlated by the same `trace_id`. Also set `failure_stage` to the
     stage name (`decode|match_subscriptions|fake_api_call|fake_db_update|deliver_webhook`) before
     recording `messages_processed_total{status=error, failure_stage=...}` - this is the one label that
     ties the aggregate "failed items" chart (item 5) to the per-stage breakdown (item 8).
   - If a message's `ApproximateReceiveCount` (message attribute) equals `DefaultMaxReceiveCount` on this
     receive, increment `messages_moved_to_dlq_total{event_type}` right before returning the error that
     will leave it unacked for SQS to redrive - this is a best-effort local signal; `queue_messages_visible
     {queue_name="webhook-events-dlq"}` from step 7 is the authoritative depth.
   - Metrics: `messages_received_total`, `messages_processed_total{status=success|error, event_type,
     failure_stage}`, `message_processing_duration_seconds{status}`, `message_end_to_end_latency_seconds
     {status}`.
9. **Instrument the subscriber:** wrap the HTTP handler so the incoming request continues the trace
   (extract from headers - the deliverer must inject into headers in step 10). Add a span
   `subscriber.store` around the S3 `Put` with histogram `s3_store_duration_seconds` and counter
   `s3_store_total{status=success|error}`.
10. **Propagate over HTTP:** in the processor's deliverer, inject the trace context into the outgoing
    request headers; in the subscriber, extract from the incoming headers. Use the otelhttp helpers or do
    it manually with the global propagator.
11. **Provision Grafana instead of hand-importing.** Add a datasource provisioning file (Tempo,
    Prometheus, Loki - otel-lgtm may already auto-provision these; verify) and a dashboard provisioning
    file under `deploy/grafana/` that otel-lgtm mounts on startup. `docker compose up` should bring the
    dashboard up with no manual steps. Panels, one per checklist row above:
    - Publish latency (`queue_publish_duration_seconds`, p50/p95 lines) - item 1.
    - Queue depth: visible + in-flight + delayed, main queue and DLQ overlaid or two panels - items 2, 3, 3b.
    - Processed items: success rate and failure rate as two lines (or stacked), by `event_type` - items 4, 5.
    - Failure breakdown: `messages_processed_total{status=error}` stacked by `failure_stage` - item 5 drill-down.
    - DLQ moves over time (`messages_moved_to_dlq_total`) - item 5b.
    - Average processing time (derived avg line) - item 6.
    - Processing time distribution: heatmap of `message_processing_duration_seconds` + p50/p90/p99 - item 7.
    - End-to-end latency vs processing-time overlay (the gap = queue wait) - item 7b.
    - Per-stage panels: subscription match, fake API call, fake DB update, webhook delivery, S3 store -
      each as duration histogram + outcome counter - items 8a-8e.

## Best practices baked into this sprint (not just "nice to have")

- **Semantic conventions**: use OTel's semantic attribute names where one exists
  (`messaging.system`, `messaging.destination.name`, `http.method`, `http.status_code`) instead of
  inventing your own - this is what makes Grafana's built-in dashboards/exemplars work without extra config.
- **Span naming convention**: `<component>.<action>` (`queue.receive`, `processor.handle_message`,
  `processor.match_subscriptions`, `processor.fake_api_call`, `processor.fake_db_update`,
  `processor.deliver_webhook`, `subscriber.store`) - consistent, greppable, and matches the tree above.
- **Bounded cardinality everywhere** - not just metric labels (existing pitfall) but **log fields too**:
  `event_type`/`status`/`failure_stage`/`http_status_class` are fine as fields/labels (all closed, small
  enums); `message_id`/`event_id` are fine as **log** fields (not metric labels) since Loki, unlike
  Prometheus, doesn't explode on high-cardinality values indexed as content rather than labels.
- **Gauges reflect external state, not code paths** - `queue_messages_visible`/`_in_flight`/`_delayed` are
  `ObservableGauge`s on a poll loop, not incremented/decremented inline; don't try to derive queue depth
  by counting `Publish`/`Delete` calls in your own code - that drifts from reality under redelivery, DLQ
  moves, and multi-instance deployments. Ask SQS.
- **A closed `failure_stage` enum, not free-text** - five known values, used as both a log field and a
  metric label. This is what makes item 5 (aggregate failures) and item 8 (per-stage breakdown) the same
  underlying data sliced two ways, instead of two things you have to keep in sync by hand.
- **ctx threading is not optional cleanup here** - it's the mechanism correlation depends on. Any new
  function that does work worth a span or a log line takes `ctx context.Context` as its first parameter.
- **Ordered, bounded shutdown** - flush logs, then traces, then metrics, under one `context.WithTimeout`,
  so a dead/unreachable collector can't hang process exit indefinitely. The queue-depth poller (step 7)
  must also stop cleanly on the same shutdown signal - don't leave it running past `ctx.Done()`.
- **Sampling is a conscious choice, not a default** - `AlwaysSample` is correct at this project's volume;
  the code should have a one-line comment saying so, noting `ParentBased(TraceIDRatioBased(...))` as what
  you'd switch to under real production volume, so a future reader doesn't mistake "always sample" for an
  oversight.
- **Dashboards as code** - the dashboard JSON lives in `deploy/` and loads via provisioning, so a fresh
  `docker compose up` reproduces the exact same dashboard with zero manual Grafana clicking.

## Definition of Done

- Producing events then opening Tempo in Grafana shows a single trace spanning producer -> processor ->
  subscriber, with per-stage span durations - including all five processor sub-stages (queue receive,
  subscription match, fake API call, fake DB update, webhook delivery) as distinct child spans.
- From any span in Tempo you can jump to its correlated log lines in Loki via `trace_id`.
- Prometheus (via Grafana) shows your custom metrics changing as load changes, including all per-stage
  histograms and the polled queue-depth gauges (main queue and DLQ, visible/in-flight/delayed).
- Forcing a message to fail repeatedly (e.g. a bad subscriber URL) shows it exceed `DefaultMaxReceiveCount`,
  `messages_moved_to_dlq_total` increments, and `queue_messages_visible{queue_name="...-dlq"}` rises -
  proving the DLQ is both functioning and observable, not just implemented.
- You can point at a panel and say "the slow/failing part is X" (e.g. fake API call vs DB update vs queue
  wait vs webhook delivery vs S3 write vs subscription lookup) with no manual log-grepping needed.
- A SIGTERM during an in-flight `fakeApiCall` actually cancels it (verifies the context-bug fix in step 5).
- Services still shut down cleanly (all three providers flush on exit, the queue-depth poller stops, no
  dropped spans/logs/metrics on Ctrl+C, and shutdown doesn't hang if the collector is unreachable).
- `docker compose up` alone reproduces the dashboard - no manual Grafana import step.
- `make lint` passes.

## Pitfalls

- Forgetting to call the shutdown/flush func -> spans/logs/metrics buffered in memory are lost on exit.
- Context not propagated -> you get three disconnected traces instead of one. Verify the parent/child link.
- Building a span/log context from `context.Background()` partway through a call chain (the
  `fakeApiCall` bug) silently breaks both propagation and cancellation - it won't error, it'll just produce
  a disconnected trace that's easy to miss.
- Wrong OTLP protocol/port: gRPC is 4317, HTTP/protobuf is 4318. Match the exporter to the port.
- Creating a new tracer/meter/logger provider per request is wasteful; create providers once at startup.
- High-cardinality **metric** labels (e.g. event id as a label) will blow up Prometheus - keep labels
  bounded. This does NOT apply the same way to Loki log fields, which are fine with high-cardinality values
  (message_id, event_id) as long as they're not promoted to indexed labels.
- Bridging zap to OTel logs without the correlation helper (step 4) gives you logs in Loki that *look*
  structured but have no `trace_id` - defeating the actual point of putting them in OTel at all.
- `ApproximateNumberOfMessages*` attributes are **approximate** (SQS/ElasticMQ eventual consistency) -
  don't alert on exact thresholds derived from a single poll; trend, don't nitpick single data points.
- Polling `GetQueueAttributes` too frequently adds needless API load for no visual benefit at this
  project's scale - 15s is plenty; the queue-depth panel doesn't need per-second resolution.
- Treating an `unmatched` subscription result as a processing failure double-counts it in the "failed
  items" chart - keep `subscription_match_total{result=unmatched}` separate from
  `messages_processed_total{status=error}`.

## Commit

Commit as "feat(observability): OpenTelemetry traces + metrics + correlated logs across all services".
