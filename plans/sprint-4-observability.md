# Sprint 4 - Observability (OpenTelemetry + Grafana)

Estimated: 10-14h. Study first: Study Guide Phase I (OpenTelemetry concepts + Go SDK wiring).

## Goal & outcome

All three services emit OpenTelemetry **traces** and **metrics** to the `grafana/otel-lgtm` container.
In Grafana you can follow a single event from producer -> processor -> subscriber as one connected trace,
and read throughput/latency/error dashboards. This is how you answer "where is the bottleneck?".

## Study first (why)

- Trace vs span vs context propagation: a trace is the whole journey; spans are steps; propagation is how
  the trace id travels across SQS and HTTP so the steps link up.
- `TracerProvider` / `MeterProvider`, the OTLP exporter, resource attributes (service.name), and clean
  `Shutdown` to flush on exit.
- Propagation carriers: HTTP headers (standard) and SQS message attributes (manual carrier).

## Build steps (in order)

1. **Add `grafana/otel-lgtm` to the compose file.** Map Grafana 3000, OTLP gRPC 4317, OTLP HTTP 4318.
   Open Grafana (admin/admin) and confirm Tempo, Prometheus, and Loki data sources exist.
2. **Build OTel setup in `internal/observability`.** Add `SetupTelemetry(ctx, service string)` that:
   - configures a resource with `service.name`,
   - creates an OTLP exporter pointing at the collector (endpoint from env, default the otel-lgtm host),
   - installs a `TracerProvider` and a `MeterProvider` and sets global propagators (tracecontext + baggage),
   - returns a `shutdown(ctx)` func that flushes/closes providers.
     Call it from all three `main.go` files; defer the shutdown.
3. **Instrument the producer:** start a span around "produce event" and **inject** the trace context into
   SQS message attributes (write a small carrier that maps the propagator to the attributes map). Add a
   counter metric for events produced.
4. **Instrument the processor:** for each received message, **extract** the trace context from the message
   attributes so the processor span is a child of the producer span. Wrap "handle message" and each
   "deliver" in spans. Add metrics: messages received, deliveries attempted/succeeded/failed, and a
   histogram for delivery duration.
5. **Instrument the subscriber:** wrap the HTTP handler so the incoming request continues the trace
   (extract from headers - the deliverer must inject into headers in step 6). Add a span around the
   S3 `Put` and a histogram for store duration.
6. **Propagate over HTTP:** in the processor's deliverer, inject the trace context into the outgoing
   request headers; in the subscriber, extract from the incoming headers. Use the otelhttp helpers or do
   it manually with the global propagator.
7. **Build a Grafana dashboard** (or use Explore first): a panel each for produce rate, delivery
   success/error rate, delivery duration p50/p95, and store duration. Save the dashboard JSON into
   `deploy/` so it's reproducible.

## Definition of Done

- Producing events then opening Tempo in Grafana shows a single trace spanning producer -> processor ->
  subscriber, with per-stage span durations.
- Prometheus (via Grafana) shows your custom metrics changing as load changes.
- You can point at a panel and say "the slow part is X" (e.g. S3 write vs HTTP delivery vs queue wait).
- Services still shut down cleanly (telemetry flushes on exit, no dropped spans on Ctrl+C).
- `make lint` passes.

## Pitfalls

- Forgetting to call the shutdown/flush func -> spans buffered in memory are lost on exit.
- Context not propagated -> you get three disconnected traces instead of one. Verify the parent/child link.
- Wrong OTLP protocol/port: gRPC is 4317, HTTP/protobuf is 4318. Match the exporter to the port.
- Creating a new tracer/meter per request is wasteful; create providers once at startup.
- High-cardinality metric labels (e.g. event id as a label) will blow up Prometheus - keep labels bounded.

## Commit

Commit as "feat(observability): OpenTelemetry traces + metrics across all services".
