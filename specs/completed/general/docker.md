# Docker development and deployment

## Status

Accepted for version one and implemented for the initial application slice. See
the [development and deployment guide](../../../docs/development.md) for commands.

## Goals

- `docker compose up --build` is the normal local entry point.
- A production Compose file can be pasted or imported as a Komodo stack with
  explicit environment, volumes, networks, health checks, and restart policy.
- The same images and paths behave consistently in development and production.
- Source-code bind mounts and hot reload are development-only.
- Persistent media and SQLite data survive container replacement.

## Services

During development:

```text
frontend-dev   framework dev server with source bind mount and hot reload
api            Go dev process with source bind mount (optional Go reload tool)
```

In production:

```text
app            Go API serving embedded compiled React assets, with FFmpeg/ffprobe
data volume    SQLite database, originals, processed media, thumbnails
```

There is no Node runtime or internal reverse-proxy container in production. The
existing home-server reverse proxy terminates TLS and routes the public origin to
the Go container over the Komodo network. Browser UI/API/media therefore share
one origin and do not require permissive CORS.

## Required conventions

- Use multi-stage builds and pin deliberate base-image versions/digests during
  implementation review.
- Run the application as a non-root user where mounted-volume permissions allow.
- Use one canonical in-container data path, proposed as `/data`, everywhere.
- Keep development data in a named volume by default; document how to inspect and
  reset it. Never make reset part of ordinary startup.
- Mount a clear host directory or named volume at `/data` in production.
- Put secrets in Komodo environment/secret management or an uncommitted `.env`;
  commit only `.env.example` with non-secret placeholders.
- Do not put passwords in image build arguments, source, or Compose defaults.
- Add health checks that test actual process readiness.
- Set upload-size/time limits consistently at both reverse proxy and Go server.
- Log to stdout/stderr and let Docker manage log collection/rotation.
- Gracefully stop the Go HTTP server and any active media work.
- Run persistent processing and retention workers within the Go service initially;
  do not add Redis or a separate queue container without a demonstrated need.
- Give FFmpeg temporary storage enough headroom for an incoming original, pass
  files, and final output even though only the final output is retained.

## Development topology

- One development Compose file starts a Vite/React service with hot reload and a
  Go API service with a small Go reload tool.
- Source is bind-mounted into the respective containers; development data uses a
  named volume mounted at `/data`.
- Vite proxies API/media requests to Go so application code uses same-origin paths.
- Production-like behavior remains testable by building the single final image.

## Persistence and migrations

- Version one uses SQLite and the local `/data` filesystem only: no PostgreSQL,
  Redis, message broker, or object storage.
- Numbered SQL migration files are embedded in the Go binary.
- Startup applies pending migrations in order before accepting traffic and records
  them in a small `schema_migrations` table.
- A migration failure prevents server startup and reports the failing migration.

## Backup and restore

Use a simple consistent cold backup:

1. Stop the application container.
2. Copy/snapshot the complete `/data` directory as one unit.
3. Restart the application.

Restore by stopping the container, replacing `/data` with one complete snapshot,
and restarting. The runbook will include a post-restore health and sample-playback
check. Scheduling and retention of snapshot copies belongs to host backup tooling,
not the application.

## Configuration contract

| Variable | Purpose | Secret |
| --- | --- | --- |
| `CLIP_SHARE_ADDR` | Go listen address, e.g. `:8080` | No |
| `CLIP_SHARE_BASE_URL` | Canonical public URL used for share/embed metadata | No |
| `CLIP_SHARE_DATA_DIR` | Persistent path, default `/data` in containers | No |
| `CLIP_SHARE_SESSION_SECRET` | Signs the short-lived session cookie | Yes |
| `CLIP_SHARE_SETUP_TOKEN` | Authorizes one-time admin creation | Yes |
| `CLIP_SHARE_ALLOWED_ORIGIN` | Optional exact LAN/browser origin for NAT-loopback-free access | No |

The setup token is not an admin password. Plaintext passwords are accepted only
over the setup/login APIs and are stored exclusively as Argon2id hashes.

## Local workflow target

The eventual workflow should be no more surprising than:

```text
copy .env.example to .env
docker compose up --build
open the documented localhost URL
docker compose run --rm api-test
```

PowerShell-compatible commands will be documented when files exist. Direct host
Go/Node commands may be offered as faster optional paths, but Docker behavior is
the source of truth.

## Komodo handoff checklist

The ready-to-import Komodo stack is `compose.komodo.yaml`. It publishes the
container's port 8080 on host port **43138**, matching the reverse-proxy route.
The Docker Hub image, example HTTPS base URL, and persistent host data path are
hardcoded there so the stack has only two required Komodo secrets:
`CLIP_SHARE_SESSION_SECRET` and `CLIP_SHARE_SETUP_TOKEN`. Change the documented
domain and data path in the Compose file if this installation uses different
values.

The Komodo stack explicitly allows the direct LAN origin
`http://192.168.1.66:43138` for installations without router NAT loopback. This
is an exact-origin exception, not a wildcard. The application also uses this
address as its fallback when the optional variable is omitted; keep it aligned
with the server's actual LAN address if that address changes.

The Komodo stack includes `user: "568:568"`, matching the standard TrueNAS
SCALE `apps` UID/GID used by dataset ACLs. If a different host identity is used,
change that one value to the UID/GID granted Modify access on the mounted data
dataset; keep the container non-root.

The stack always pulls `rakibshahid/clip-share:latest` on deployment. The
repository `Makefile` provides `image` for a local production build and `release`
for an authenticated Docker Hub build/push. Use semantic-style version tags such
as `0.0.1`, `0.0.2`, or `0.0.1-build2` for manual rollback; the Komodo stack
intentionally tracks `latest`.

- Set the public base URL and secret values in Komodo.
- Map an existing, backed-up host directory to the exact `/data` path.
- Join the expected reverse-proxy network and configure the hostname/TLS route.
- Confirm proxy and API upload limits agree.
- Confirm container user permissions on the data directory.
- Run health/readiness checks before exposing traffic.
- Test upload, processing, playback, thumbnail, embed, restart, and restore.
- Record backup locations for both the SQLite database and media tree; they must
  represent a consistent point in time.

## Implementation checks still needed

- Verify bind-mount hot reload on Windows and production behavior on the Linux host.
- Measure CPU/memory behavior when uncapped FFmpeg jobs run concurrently.
- Test cold backup and restore with queued, ready, and trashed records.
