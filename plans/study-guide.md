# Study Guide - what to learn, when, and where

You're new to Go, so this guide maps concepts to the sprint where you first need them. Learn
**just-in-time**: study the topics for a sprint right before you start it, not all at once. Depth
target: enough to write the sprint's code confidently and explain it to someone else.

Legend: (must) = you cannot do the sprint without it; (helpful) = makes it smoother.

## Phase A - Go fundamentals (do before Sprint 0/1)

- (must) Go tour & syntax: variables, functions, `for`, `if`, slices, maps, structs.
- (must) Packages & modules: `go mod init`, imports, exported vs unexported (capitalization).
- (must) Errors: the `error` interface, `errors.New`, `fmt.Errorf("...: %w", err)`, error wrapping/`errors.Is`.
- (must) Pointers vs values, and when methods use pointer receivers.
- (must) Interfaces & composition - the backbone of the `queue`/`storage` wrappers you'll write.
- (helpful) `defer`, zero values, named returns.

Resources:

- A Tour of Go - https://go.dev/tour/
- Effective Go - https://go.dev/doc/effective_go
- How to Write Go Code - https://go.dev/doc/code
- Standard library `errors` - https://pkg.go.dev/errors

## Phase B - I/O, JSON, config (Sprint 1-2)

- (must) `encoding/json`: struct tags, `Marshal`/`Unmarshal`, handling unknown fields.
- (must) YAML config: `gopkg.in/yaml.v3` to load `subscriptions.yaml`.
- (must) Reading env vars: `os.Getenv`, sane defaults, fail-fast on missing required config.
- (helpful) `time` package: timers/tickers for the producer's interval/burst.

Resources:

- `encoding/json` - https://pkg.go.dev/encoding/json
- JSON and Go (blog) - https://go.dev/blog/json
- `gopkg.in/yaml.v3` - https://pkg.go.dev/gopkg.in/yaml.v3

## Phase C - concurrency & lifecycle (Sprint 1-3, critical in Sprint 2)

- (must) Goroutines and `go` keyword; what "concurrent" actually means here.
- (must) `context.Context`: cancellation, deadlines, `context.WithCancel`, passing ctx as the first arg.
- (must) Graceful shutdown: catching `SIGINT`/`SIGTERM` with `signal.NotifyContext`, draining work.
- (must) Channels and `select` (for the consumer loop and shutdown signaling).
- (must) The race detector: run the app with `-race` to _detect_ when you need synchronization
  instead of guessing.
- (helpful) `sync.WaitGroup` to wait for in-flight work before exit; `errgroup` (`golang.org/x/sync/errgroup`).
- (helpful) `sync.Mutex` / `sync.RWMutex`, `sync.Once`, and `sync/atomic` - the tools for sharing mutable
  state safely. See the note below on whether this project actually needs them.

Note on mutexes (you asked - here's the honest answer): the mandated core can be built without writing a
mutex, because the producer is a single goroutine, the processor starts sequential, and
`subscriptions.yaml` is read-only after load (read-only sharing is safe). You will, however, hit the exact
problem mutexes solve in a few natural spots, so learn them conceptually and apply when the race detector
or design demands it:

- Lazy-caching the queue URL / bucket existence across goroutines -> `sync.Once` (preferred) or a `Mutex`.
- Processing a batch concurrently (optional worker pool in Sprint 2) -> a channel is usually cleaner than a `Mutex`.
- Hot-reloading `subscriptions.yaml` at runtime (an enhancement) -> classic `sync.RWMutex` (many readers, rare writer).
- A homegrown counter would need a `Mutex`/`atomic`, but OTel metrics (Sprint 4) are already concurrency-safe, so you avoid it there.
  Go philosophy: "share memory by communicating." Reach for a channel to hand off work between goroutines;
  reach for a `Mutex` to guard a small piece of shared state. Learn both; pick per situation.

Resources:

- Go Concurrency Patterns - https://go.dev/blog/pipelines
- `context` package - https://pkg.go.dev/context
- `os/signal` (`NotifyContext`) - https://pkg.go.dev/os/signal#NotifyContext
- `sync` package (`Mutex`, `RWMutex`, `Once`) - https://pkg.go.dev/sync
- `sync/atomic` - https://pkg.go.dev/sync/atomic
- Data Race Detector - https://go.dev/doc/articles/race_detector

## Phase D - HTTP server & client (Sprint 3)

- (must) `net/http` server: `http.ServeMux`, handlers, method-based routes (Go 1.22+), `http.Server` with timeouts.
- (must) `net/http` client: `http.Client` with a timeout, building requests, reading/closing response bodies.
- (must) Status codes & idempotency: why a webhook receiver returns 2xx and tolerates duplicates.
- (helpful) Middleware pattern (wrapping handlers) for logging/tracing.

Resources:

- `net/http` - https://pkg.go.dev/net/http
- Routing enhancements (Go 1.22) - https://go.dev/blog/routing-enhancements
- Don't forget `defer resp.Body.Close()` and to drain the body.

## Phase E - AWS SDK v2 (Sprint 1 for SQS, Sprint 3 for S3)

- (must) `config.LoadDefaultConfig` and overriding `BaseEndpoint` for ElasticMQ/MinIO.
- (must) SQS: `CreateQueue`, `GetQueueUrl`, `SendMessage`, `ReceiveMessage` (long polling), `DeleteMessage`,
  and the meaning of _visibility timeout_.
- (must) S3: `CreateBucket`, `PutObject`, object keys, content type. MinIO needs `UsePathStyle = true`.
- (helpful) How the SDK does retries and timeouts.

Resources:

- aws-sdk-go-v2 - https://aws.github.io/aws-sdk-go-v2/docs/
- SQS client - https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/sqs
- S3 client - https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
- ElasticMQ - https://github.com/softwaremill/elasticmq
- MinIO (S3 compatibility) - https://min.io/docs/minio/linux/index.html

## Phase F - logging (start in Sprint 0, continue throughout)

- (must) `log/slog`: structured logs, log levels, attaching attributes (e.g. event id, subscriber).

Resources:

- `log/slog` - https://pkg.go.dev/log/slog

## Phase G - containers (Sprint 5)

- (must) Docker basics: images vs containers, `Dockerfile`, build context, `docker build`/`run`.
- (must) Multi-stage builds for Go (compile in a `golang` stage, copy the static binary into a tiny base).
- (must) docker-compose: services, ports, env, `depends_on`, healthchecks, named volumes, networks.
- (helpful) Distroless/static base images and why a Go binary can run on `scratch`.

Resources:

- Dockerfile reference - https://docs.docker.com/reference/dockerfile/
- Compose file reference - https://docs.docker.com/reference/compose-file/
- Distroless - https://github.com/GoogleContainerTools/distroless

## Phase H - Kubernetes, Helm, KEDA (Sprint 6)

- (must) K8s core objects: Pod, Deployment, Service, ConfigMap, Secret, namespaces.
- (must) `kubectl` basics: apply, get, describe, logs, port-forward.
- (must) `kind` to run a cluster locally; loading local images into kind.
- (must) Helm: chart structure (`Chart.yaml`, `values.yaml`, `templates/`), `helm install/upgrade`.
- (must) KEDA: `ScaledObject`, triggers, the `aws-sqs-queue` scaler, and `awsEndpoint` for ElasticMQ.
- (helpful) HPA vs KEDA, scale-to-zero, cooldown/polling intervals.

Resources:

- Kubernetes basics - https://kubernetes.io/docs/tutorials/kubernetes-basics/
- kind - https://kind.sigs.k8s.io/
- Helm - https://helm.sh/docs/
- KEDA - https://keda.sh/docs/latest/
- KEDA AWS SQS scaler - https://keda.sh/docs/latest/scalers/aws-sqs/

## Phase I - OpenTelemetry (Sprint 4)

- (must) Concepts: trace, span, context propagation, metrics, the OTLP exporter, resource attributes.
- (must) Go SDK wiring: `TracerProvider`, `MeterProvider`, OTLP exporter, graceful `Shutdown`.
- (helpful) Propagating trace context across SQS (via message attributes) and HTTP (via headers).

Resources:

- OpenTelemetry Go - https://opentelemetry.io/docs/languages/go/
- Getting started (Go) - https://opentelemetry.io/docs/languages/go/getting-started/
- grafana/otel-lgtm - https://github.com/grafana/docker-otel-lgtm
