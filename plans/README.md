# Hookwave - Build Plan (study & implement by hand)

This directory is your roadmap for building a **webhook delivery platform** in Go, by hand,
one sprint at a time. Nothing here is meant to be code-generated for you - each sprint tells you
*what to learn first*, *what to build in order*, *how to verify it works*, and *what usually goes wrong*.

Project name placeholder: **`hookwave`** (change it everywhere if you pick another name before Sprint 0).

## How to use these plans

1. Read [`architecture.md`](architecture.md) once, end to end, so you have the whole picture.
2. Skim [`study-guide.md`](study-guide.md) and bookmark the resources for the current sprint.
3. Work the sprints **in order**. Each sprint file is self-contained and has:
   - Goal & outcome (what exists when you're done)
   - Study first (concepts to learn before coding, with why)
   - Build steps (ordered, small, checkable tasks)
   - Definition of Done (commands to run + expected result)
   - Pitfalls (the mistakes beginners actually hit)
   - References (official docs only)
4. Do not jump ahead. Each sprint depends on the previous one compiling and running.
5. Commit at the end of every sprint (and ideally after every Build step).

## Sprint sequence

- [`sprint-0-foundations.md`](sprint-0-foundations.md) - repo, module, layout, tooling. (~6-9h)
- [`sprint-1-producer.md`](sprint-1-producer.md) - producer sends fake events to ElasticMQ. (~8-12h)
- [`sprint-2-processor.md`](sprint-2-processor.md) - processor consumes the queue + matches subscriptions. (~10-14h)
- [`sprint-3-subscriber-storage.md`](sprint-3-subscriber-storage.md) - HTTP delivery to a subscriber that stores to MinIO. (~10-14h)
- [`sprint-4-observability.md`](sprint-4-observability.md) - OpenTelemetry traces + metrics + Grafana. (~10-14h)
- [`sprint-5-containerize.md`](sprint-5-containerize.md) - Dockerfiles + one-command compose stack. (~6-10h)
- [`sprint-6-kubernetes-keda.md`](sprint-6-kubernetes-keda.md) - kind + Helm + KEDA autoscaling demo. (~12-18h)

Core total: roughly **62-91 hours** at a learning pace. Nice-to-have items live at the bottom of
[`architecture.md`](architecture.md) and are explicitly optional.

## What you should have installed before Sprint 0

Install these and confirm each version prints. (You'll add language-level deps with `go get` later.)

- **Go 1.26+** - `go version`
- **Docker + Docker Compose** - `docker version` and `docker compose version`
- **Git** - `git --version`
- **make** - `make --version` (preinstalled on macOS via Xcode CLT)
- **golangci-lint** - `golangci-lint --version` (installed in Sprint 0)
- A SQS/S3-aware CLI is optional; you can inspect ElasticMQ via its web UI and MinIO via its console.

Installed later, only when their sprint arrives (don't front-load these):

- **kind** (or k3d) + **kubectl** + **helm** - Sprint 6
- **KEDA** - Sprint 6 (installed into the cluster via Helm)

## Mental model in one line

`producer -> SQS/ElasticMQ -> processor -> (HTTP delivery) -> subscriber endpoint -> S3/MinIO`,
with every hop emitting OpenTelemetry traces/metrics to Grafana.

## Ground rules for learning by hand

- Type the code yourself. Do not paste large blocks. The point is to learn Go.
- After each Build step, run it and observe the result before moving on.
- When something breaks, read the actual error first. Go errors are usually precise.
- Keep functions small and packages focused; refactor when a file passes ~200 lines.
- Write at least one test per package as you go (Sprint 0 sets up the habit).
