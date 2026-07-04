# Sprint 0 - Foundations & repo skeleton

Estimated: 6-9h. Study first: Study Guide Phase A (Go fundamentals) + logging basics.

## Goal & outcome

A clean monorepo that compiles, has the agreed layout, three empty-but-runnable binaries, a `Makefile`,
and linting. No business logic yet. When done, `make build` and `make lint` both succeed and each of the
three services starts and prints a log line.

## Study first (why each matters)

- Modules & packages: you'll create one module and many packages; understand import paths.
- Exported vs unexported identifiers (capitalization) - decides your package APIs.
- `log/slog`: every service logs structured lines from day one.

## Build steps (in order)

1. **Decide the name.** Default `hookwave`. Whatever you pick is the module path base. Write it down.
2. **Initialize git** (if not already): `git init`, add a `.gitignore` for Go (ignore built binaries,
   `.env`, local data dirs like `*/data/`).
3. **Set the module path.** Update `go.mod`'s module line to your name (e.g. `module github.com/<you>/hookwave`).
   Keep `go 1.26`. (You can use a bare name like `hookwave` if you won't publish, but a GitHub-style
   path is the convention.)
4. **Create the directory skeleton** exactly as in `architecture.md` Section 5: `cmd/{producer,processor,subscriber}`,
   `internal/{events,queue,storage,config,subscriptions,httpx,observability}`, `configs/`, `deploy/{docker,compose,helm}`,
   `docs/`. Put a `.gitkeep` in dirs that are still empty so git tracks them.
5. **Write three minimal `main.go` files**, one per `cmd/*`. Each should:
   - create an `slog` JSON logger,
   - log a startup line with the service name,
   - set up a `signal.NotifyContext` for `SIGINT`/`SIGTERM`,
   - block until that context is cancelled, then log a shutdown line and exit 0.
     This gives you the graceful-shutdown skeleton you'll reuse everywhere.
6. **Create a shared logger helper** in `internal/observability` (e.g. `NewLogger(service string) *slog.Logger`)
   so all three binaries build the logger the same way. Wire each `main.go` to use it.
7. **Add a Makefile** with at least: `build` (build all three), `run-producer`/`run-processor`/`run-subscriber`,
   `lint` (`golangci-lint run`), `fmt` (`gofmt -w .`), `tidy` (`go mod tidy`).
8. **Install and configure golangci-lint.** Add a minimal `.golangci.yml` enabling at least
   `govet`, `staticcheck`, `errcheck`, `ineffassign`, `gofmt`.
9. **Create `configs/subscriptions.json`** as a placeholder with the shape you'll use in Sprint 2
   (a list of subscribers, each with `events: [order.created, ...]`, `url`, and `company_id`).
   This file is not hand-written - `cmd/subscriber` generates it (`GENERATE_SUBSCRIPTIONS=N` /
   `make generate-subscriptions`) with random fake entries. Run it once to commit the shape;
   you won't load it yet - that's Sprint 2.
10. **Copy a trimmed architecture into `docs/ARCHITECTURE.md`** (the repo's own doc, separate from `plans/`).

## Suggested package responsibilities (so you don't blur boundaries later)

- `internal/events`: the event types only. No I/O.
- `internal/observability`: logger + (later) OTel setup. No business logic.
- `cmd/*`: wiring/composition only - parse config, construct dependencies, run. Keep it thin.

## Definition of Done (run these)

- `go build ./...` -> no errors.
- `make build` -> produces three binaries (or builds cleanly).
- `make lint` -> no findings (fix or justify any).
- Running each binary prints a startup log line, and Ctrl+C prints a shutdown line and exits cleanly.

## Pitfalls

- Import path mismatches: your imports must match the `go.mod` module path exactly. If you rename the
  module later, every internal import changes - decide the name now.
- Git won't track empty dirs; use `.gitkeep`.
- Don't put logic in `main.go`. `main` wires things; packages do the work.
- Running a binary that immediately exits usually means you forgot to block on the shutdown context.

## Commit

Commit as "chore: repo skeleton, tooling, graceful-shutdown scaffolding".
