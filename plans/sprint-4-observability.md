# Sprint 4 - Observability (OpenTelemetry + Grafana)

Estimated: 14-20h. Study first: Study Guide Phase I (OpenTelemetry concepts + Go SDK wiring).

## Goal & outcome

All three services emit OpenTelemetry **traces**, **metrics**, and **logs** to the `grafana/otel-lgtm`
container. In Grafana you can follow a single event from producer -> processor -> subscriber as one
connected trace, drill into *which stage* was slow (queue wait vs fake API call vs DB update vs HTTP
delivery vs S3 write), jump from a trace straight to its correlated log lines in Loki, and read
throughput/latency/error dashboards. This is how you answer "where is the bottleneck?" **and** "show me
everything that happened around this failed delivery."

## Study first (why)

- Trace vs span vs context propagation: a trace is the whole journey; spans are steps; propagation is how
  the trace id travels across SQS and HTTP so the steps link up.
- `TracerProvider` / `MeterProvider` / `LoggerProvider`, the OTLP exporter, resource attributes
  (`service.name`), and clean `Shutdown` to flush on exit.
- Propagation carriers: HTTP headers (standard) and SQS message attributes (manual carrier).
- Log-trace correlation: why a log line needs `trace_id`/`span_id` fields to be useful, and why that means
  `ctx` (the thing carrying the active span) has to reach every function that logs.

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
6. **Instrument the producer:** start a span around "produce event" and **inject** the trace context into
   SQS message attributes (write a small carrier that maps the propagator to the attributes map). Add a
   counter metric for events produced.
7. **Instrument the processor with the full per-message span tree**, not just one span per message:
   - `queue.receive` - wraps the `ReceiveMessages` call (one span per batch poll, not per message, since
     it's a shared network call). Histogram: `queue_receive_duration_seconds`.
   - `processor.handle_message` - the parent span for one message, started right after
     `extract`-ing the trace context from the message attributes so it's a **child of the producer span**.
   - `processor.fake_api_call` - child span wrapping `fakeApiCall`. Histogram:
     `fake_api_call_duration_seconds`. Counter: `fake_api_call_timeouts_total`.
   - `processor.fake_db_update` - child span wrapping `fakeDbUpdate`. Histogram:
     `fake_db_update_duration_seconds`.
   - On any error in the chain: `span.RecordError(err)` + `span.SetStatus(codes.Error, msg)` on the
     relevant span, **and** log via `observability.FromContext(ctx, logger).Error(...)` so the failure is
     visible in both Tempo and Loki, correlated by the same `trace_id`.
   - Metrics: `messages_received_total`, `messages_processed_total{status=success|error}`, and the
     existing end-to-end `message_processing_duration_seconds` histogram.
8. **Instrument the subscriber:** wrap the HTTP handler so the incoming request continues the trace
   (extract from headers - the deliverer must inject into headers in step 9). Add a span around the
   S3 `Put` and a histogram for store duration.
9. **Propagate over HTTP:** in the processor's deliverer, inject the trace context into the outgoing
   request headers; in the subscriber, extract from the incoming headers. Use the otelhttp helpers or do
   it manually with the global propagator.
10. **Provision Grafana instead of hand-importing.** Add a datasource provisioning file (Tempo,
    Prometheus, Loki - otel-lgtm may already auto-provision these; verify) and a dashboard provisioning
    file under `deploy/grafana/` that otel-lgtm mounts on startup, with panels for: produce rate, delivery
    success/error rate, delivery duration p50/p95, store duration, and the three new processor-stage
    histograms (queue receive / fake API call / fake DB update). `docker compose up` should bring the
    dashboard up with no manual steps.

## Best practices baked into this sprint (not just "nice to have")

- **Semantic conventions**: use OTel's semantic attribute names where one exists
  (`messaging.system`, `messaging.destination.name`, `http.method`, `http.status_code`) instead of
  inventing your own - this is what makes Grafana's built-in dashboards/exemplars work without extra config.
- **Span naming convention**: `<component>.<action>` (`queue.receive`, `processor.handle_message`,
  `processor.fake_api_call`, `subscriber.store`) - consistent, greppable, and matches the tree above.
- **Bounded cardinality everywhere** - not just metric labels (existing pitfall) but **log fields too**:
  `event_type`/`status` are fine as fields/labels, `message_id`/`event_id` are fine as **log** fields
  (not metric labels) since Loki, unlike Prometheus, doesn't explode on high-cardinality values indexed as
  content rather than labels.
- **ctx threading is not optional cleanup here** - it's the mechanism correlation depends on. Any new
  function that does work worth a span or a log line takes `ctx context.Context` as its first parameter.
- **Ordered, bounded shutdown** - flush logs, then traces, then metrics, under one `context.WithTimeout`,
  so a dead/unreachable collector can't hang process exit indefinitely.
- **Sampling is a conscious choice, not a default** - `AlwaysSample` is correct at this project's volume;
  the code should have a one-line comment saying so, noting `ParentBased(TraceIDRatioBased(...))` as what
  you'd switch to under real production volume, so a future reader doesn't mistake "always sample" for an
  oversight.
- **Dashboards as code** - the dashboard JSON lives in `deploy/` and loads via provisioning, so a fresh
  `docker compose up` reproduces the exact same dashboard with zero manual Grafana clicking.

## Definition of Done

- Producing events then opening Tempo in Grafana shows a single trace spanning producer -> processor ->
  subscriber, with per-stage span durations - including the three processor sub-stages (queue receive,
  fake API call, fake DB update) as distinct child spans.
- From any span in Tempo you can jump to its correlated log lines in Loki via `trace_id`.
- Prometheus (via Grafana) shows your custom metrics changing as load changes, including the three new
  per-stage histograms.
- You can point at a panel and say "the slow part is X" (e.g. fake API call vs DB update vs queue wait vs
  S3 write) with no manual log-grepping needed.
- A SIGTERM during an in-flight `fakeApiCall` actually cancels it (verifies the context-bug fix in step 5).
- Services still shut down cleanly (all three providers flush on exit, no dropped spans/logs/metrics on
  Ctrl+C, and shutdown doesn't hang if the collector is unreachable).
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

## Commit

Commit as "feat(observability): OpenTelemetry traces + metrics + correlated logs across all services".
