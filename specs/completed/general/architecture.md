# Architecture

## Status

Accepted for version one. Exact API contracts are specified separately before
implementation.

## Initial architecture

Use one Go API process, SQLite for metadata, a mounted filesystem for original
and derived media, and FFmpeg/ffprobe for media inspection and transformation.
Keep the initial topology single-instance because SQLite and local files are an
excellent fit for a personal home server and are easy to understand and back up.
The production Go binary embeds and serves the compiled React application. The
existing external home-server proxy owns TLS; no application proxy or production
Node service is added.

Possible repository shape:

```text
cmd/server/main.go          program entry point and wiring
internal/httpapi/           routes, request parsing, responses, middleware
internal/clips/             clip types and application operations
internal/store/             SQLite implementation and migrations
internal/media/             paths, ffprobe, FFmpeg, thumbnails
web/                        frontend application (after UI decision)
specs/                      accepted behavior and future change proposals
```

Packages should be introduced only when there is real code to place in them.

## Request flow

For a future upload, the likely flow is:

1. The handler authenticates the request and applies a body-size limit.
2. The upload streams to a temporary file; it is not loaded fully into memory.
3. `ffprobe` validates the media and records trustworthy attributes.
4. The application assigns an opaque ID and atomically moves the media into its
   persistent location.
5. A SQLite transaction records the clip and a durable pending processing job.
6. The API returns the clip in a queued state without holding the upload request
   open for encoding.
7. A bounded background worker claims the job, runs FFmpeg, verifies the H.264/AAC
   MP4 and size, creates a poster, and atomically marks the clip ready.
8. After durable success, the worker deletes the incoming original.

Job claims, attempts, errors, and next-run times live in SQLite so interrupted
work can safely resume. Start with workers inside the Go service and no
application-level concurrency cap, as accepted; a separate queue service is
unnecessary for one home server.

Keep this deliberately small: one SQLite `jobs` table, workers in the Go process,
two-second client polling, and no Redis, message broker, WebSockets, retained
failed inputs, or automatic encode retries. Failed media is removed immediately;
only a small error record remains long enough for the UI to explain the failure.

A scheduled retention worker also finds trashed records older than 90 days and
removes their physical assets and metadata idempotently. Restore and purge must
coordinate with processing so no worker publishes a deleted clip.

## Go dependency policy

- Start with `net/http`, `encoding/json`, `io`, `os`, `os/exec`, and `context`.
- Use the standard `http.ServeMux` unless route requirements demonstrate a need
  for a third-party router.
- Prefer explicit SQL and a small SQLite driver over an ORM for the first version.
- Wrap external processes and storage at narrow seams so tests can substitute
  deterministic implementations.
- Add dependencies because they solve an accepted requirement, not preemptively.

## Implementation conventions

- Use small functions with ordinary control flow and meaningful names.
- Add comments for intent or surprising rules, not line-by-line narration.
- Prefer table-driven tests when several inputs follow the same rule.
- Record the exact `go test` and Docker commands used for verification.
- Add concurrency only for a concrete requirement. HTTP requests are already
  handled concurrently by `net/http`.

## Detailed contracts

- `api.md` defines resources, errors, pagination, uploads, and health routes.
- `data-and-storage.md` defines tables, invariants, IDs, paths, and transactions.
- `operations.md` defines startup/recovery, shutdown, health, logs, and backup.

Version one uses embedded numbered SQL migrations and a `schema_migrations` table.
It deliberately excludes PostgreSQL, Redis, object storage, and an application
backup subsystem. The accepted cold-backup runbook snapshots all of `/data` while
the container is stopped.

Concrete SQL and Go function signatures are implementation details constrained by
these accepted contracts and their acceptance criteria.
