# Runtime and operations contract

## Status

Accepted for version one.

## Startup

Before readiness, the Go process:

1. Validates configuration and session/setup secrets.
2. Creates required `/data` children and confirms writability.
3. Opens SQLite, enables pragmas, and applies migrations.
4. Confirms FFmpeg and ffprobe are executable.
5. Reconciles stale reservations and interrupted jobs.
6. Starts processing and retention workers.
7. Marks readiness successful and begins normal traffic.

A required-step failure prevents readiness and exits with a clear, secret-free log.

The retention worker runs once at startup and then hourly. Each pass permanently
purges top-level trash items whose original deletion time is at least 90 days old.
Asset removal is retry-safe: a failed pass retains the database records for a
later attempt, and already-absent media directories are treated as successfully
removed.

Reconciliation changes every interrupted `processing` job back to `queued`,
deletes its partial outputs, retains its completed temporary source, and preserves
FIFO order for reprocessing. Startup deletes temporary directories with no
matching active job/reservation but never guesses about final media directories.
If expected final media is missing, the clip becomes unavailable and a structured
error is logged; its database record is retained for diagnosis.

## Shutdown

On Docker termination, the process fails readiness, stops accepting new work,
allows ordinary requests a short grace period, asks owned FFmpeg processes to
stop, leaves durable interrupted jobs recoverable, closes SQLite, and exits within
a 30-second container stop timeout.

An upload has a one-hour total timeout. Timeout deletes its source/partials,
releases its reservation, and reports failure.

## Logging

Structured JSON goes to stdout/stderr with timestamp, severity, event, request ID,
safe internal IDs, outcome, duration, and safe error category. Never log passwords,
cookies, setup/session tokens, full query strings, media, or public clip URLs.

Log startup/migration, login outcome, account administration, upload admission,
job transitions, admin move/copy/purge, retention purge, and shutdown. There is no
database audit subsystem or audit UI.

## Configuration and backup

Environment variables use a `CLIP_SHARE_` prefix. Secrets come from Komodo/Docker
secret configuration; `.env.example` contains placeholders only.

Two secrets are required initially:

- A session-signing secret containing at least 32 cryptographically random bytes.
- A setup token containing at least 32 cryptographically random bytes; remove it
  after the first super-admin is created.

Failure to migrate, open SQLite, or access writable `/data` prevents startup; the
service never advertises partial readiness.

Cold backup stops the app, snapshots all of `/data`, and restarts. Restore stops
the app, replaces all of `/data` with one snapshot, restarts, checks readiness,
logs in, and plays a known sample clip. Host tooling owns scheduling and retention.
