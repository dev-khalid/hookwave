# Sprint 5 - Containerize fully + one-command compose stack

Estimated: 6-10h. Study first: Study Guide Phase G (Docker multi-stage + compose).

## Goal & outcome

Each service has its own small, multi-stage Docker image. A single `deploy/compose/docker-compose.yaml`
plus `make up` brings the entire stack online: ElasticMQ, MinIO, otel-lgtm, producer, processor,
subscriber - all wired with healthchecks and graceful shutdown. `make down` tears it down.

## Study first (why)

- Multi-stage builds: compile in a `golang` image, copy only the static binary into a tiny final image
  (distroless or scratch) so images are small and have minimal attack surface.
- Compose: `depends_on` with `condition: service_healthy`, healthchecks, env, named volumes, networks.
- How a Go binary becomes static (CGO disabled) so it can run on a minimal base.

## Build steps (in order)

1. **Write a multi-stage Dockerfile per service** in `deploy/docker/` (or one parameterized Dockerfile
   that takes the target via build arg). Build stage: `go build` with `CGO_ENABLED=0`. Final stage: a
   distroless/static (or scratch) image containing just the binary and, if needed, CA certs. Run as a
   non-root user.
2. **Add `.dockerignore`** so the build context excludes `.git`, local data dirs, and build artifacts.
3. **Add healthchecks.** Give the subscriber (and optionally producer/processor) a lightweight
   `GET /healthz` endpoint; wire compose healthchecks to it. For ElasticMQ/MinIO use their documented
   health endpoints or a TCP check.
4. **Complete the compose file:** all six+ services on one network, ordered with `depends_on` +
   `service_healthy`, env for endpoints/creds/queue name/OTLP endpoint, named volumes for ElasticMQ and
   MinIO data, and ports mapped (mind the 3000 vs 9325 clash noted earlier).
5. **Make endpoints config-driven** so the same binaries work in compose (service names) and on your host
   (localhost). No hardcoded URLs.
6. **Add Make targets:** `up` (`docker compose up --build -d`), `down`, `logs`, `ps`, `restart`.
   Optionally `up-app` vs `up-infra` to start infra only.
7. **Verify graceful shutdown in containers:** ensure your services handle `SIGTERM` (compose/k8s send it)
   and exit within the stop grace period; set `stop_grace_period` if needed.

## Definition of Done

- `make up` from a clean state builds images and starts everything; `make ps` shows all healthy.
- The full event flow works exactly as in Sprint 3/4 but entirely inside compose (no host-run binaries).
- Traces/metrics still land in Grafana from the containerized services.
- `make down` stops everything; named volumes persist data across `up`/`down` cycles.
- Images are reasonably small (tens of MB, not hundreds) - check `docker images`.

## Pitfalls

- Forgetting `CGO_ENABLED=0` -> binary won't run on scratch/distroless (dynamic link errors).
- Missing CA certificates in the final image -> TLS calls fail (add `ca-certificates` or use distroless
  which includes them).
- `depends_on` without healthcheck conditions only waits for *start*, not *ready* - services race and
  fail on first boot. Use `condition: service_healthy`.
- Containers ignoring `SIGTERM` get `SIGKILL`ed after the grace period - make sure your shutdown is wired.
- Host vs container hostnames again: inside compose use service names (`elasticmq`, `minio`, `otel-lgtm`).

## Commit

Commit as "build: containerize all services and complete one-command compose stack".
