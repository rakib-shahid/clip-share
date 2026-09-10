# Development and deployment

## What works in this slice

- startup configuration validation and automatic SQLite migrations;
- a one-time, token-protected super-admin setup flow;
- Argon2id password hashing and signed, non-persistent 24-hour browser cookies;
- login, logout, current-user, and complete admin user-lifecycle APIs;
- CSRF and same-origin checks on authenticated writes;
- an initial dark, charcoal-and-sky-blue responsive interface;
- admin setup, login, user management, and a placeholder library screen;
- nested folder browsing with breadcrumbs and 60-item pagination;
- validated folder creation and rename operations;
- ownership-safe folder moves, including admin cross-library moves;
- streamed single-video upload intake with a strict 500 MB source ceiling;
- FFprobe validation for duration, resolution, frame rate, and real video streams;
- durable queued clips/jobs, temporary-capacity reservations, and failure cleanup;
- browser upload progress, compression rules, and the two approved controls;
- restart-safe FIFO job claiming with uncapped concurrent FFmpeg workers;
- MP4 remuxing or H.264/AAC normalization, poster creation, and atomic publishing;
- two-pass target-size encoding and the accepted 110% final-size threshold;
- separate hot-reload development services and one production image;
- health and readiness endpoints plus graceful Go server shutdown.
- authenticated recycle-bin video previews that do not reactivate public links;
- administrator-only permanent trash purge with retry-safe media removal;
- a retention pass at startup and hourly automatic purge after 90 days.
- recursive, case-insensitive, authorization-scoped folder and clip search.
- administrator-only independent folder-subtree copies with staged media publishing.
- recursive folder-deletion summaries with authorization-scoped totals.
- confirmed in-flight upload abort, durable job cancellation, and failed-notice dismissal.
- a restart-stable one-hour total upload/processing deadline with immediate cleanup;
- optional pre-finalization accurate trimming, multi-track audio mixing, private
  rendered previews, and editor-session cleanup.

Public clip pages, clip management, the complete recoverable trash lifecycle, and
account administration, library search, folder copying, recursive
folder-deletion summaries, upload/job cancellation controls, and media-validation
criteria are implemented, including the one-hour total deadline and the
pre-finalization trim and multi-track audio editor. Its completed contract is in
`specs/completed/05-video-editing-and-audio.md`.

## Local development with Docker

Docker Compose is the source-of-truth development workflow. Go and Node do not
need to be installed on the host.

1. Create the local environment file:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Replace `CLIP_SHARE_SESSION_SECRET` and `CLIP_SHARE_SETUP_TOKEN` with two
   independent random values of at least 32 characters. Do not reuse a password.

3. Build and start both hot-reload services:

   ```powershell
   docker compose up --build
   ```

4. Open <http://localhost:5173>. Vite serves the React app and proxies `/api`,
   `/healthz`, and `/readyz` to the Go service at port 8080.

The first screen asks for the setup token, an admin username, and an admin
password. Setup permanently closes after that admin is created. Development data
lives in the `clip-share-dev_dev-data` Docker volume and survives rebuilds.

Stop the services with `docker compose down`. This preserves data. Removing the
volume is an explicit destructive reset and should only be done when its contents
are no longer needed.

## Production and Komodo

### Optional Komodo confirmation helper

The userscript [`komodo-confirm-copy.user.js`](komodo-confirm-copy.user.js) is a
local convenience for the confirmation dialogs at
`http://192.168.1.66:30160`. Install it in Tampermonkey, then click Komodo's
“Please enter **clip-share** below…” text to copy the required value. It is
scoped to that exact HTTP host and does not alter the confirmation or deploy
action itself.

The production Compose file builds a single non-root image containing the Go
server, embedded compiled React assets, FFmpeg, and ffprobe. Node and Vite are not
present at runtime.

Configure these values in Komodo rather than committing a production `.env`:

- `CLIP_SHARE_BASE_URL`: the public HTTPS origin;
- `CLIP_SHARE_DATA_PATH`: an absolute, backed-up host directory;
- `CLIP_SHARE_SESSION_SECRET`: a private random secret;
- `CLIP_SHARE_SETUP_TOKEN`: a separate private bootstrap token;
- optionally `CLIP_SHARE_PORT` and `CLIP_SHARE_IMAGE`.

For a source-built deployment, use `compose.prod.yaml`:

```powershell
docker compose --env-file .env -f compose.prod.yaml up -d --build
docker compose --env-file .env -f compose.prod.yaml ps
```

For Docker Hub/Komodo deployment, publish the production image with the root
`Makefile`, then use `compose.komodo.yaml`. For example, from PowerShell or a
shell with GNU Make installed:

```text
make release DOCKERHUB_USERNAME=yourname VERSION=0.0.1
```

This runs interactive `docker login`, builds the production target, and pushes
both the version tag and `latest`. Use semantic-style tags such as `0.0.1`,
`0.0.2`, or `0.0.1-build2` when publishing multiple builds in one day. Set
`CLIP_SHARE_IMAGE=yourname/clip-share:latest`
in Komodo. The Komodo stack has no build context and uses `pull_policy: always`,
so redeploying pulls the image rather than compiling on the server. Prefer an
immutable version tag in Komodo when you want deliberately controlled upgrades.

The host data directory is mounted at `/data`; it contains the database and will
later contain media. Back up that directory as one unit while the app is stopped.
The external home-server proxy should terminate TLS and route the public hostname
to container port 8080. Secure cookies are always enabled in production.

## Verification

The focused host-side checks are useful while editing, while the image build
proves the supported deployment artifact:

```powershell
go test ./cmd/... ./internal/...
npm.cmd --prefix web run lint
npm.cmd --prefix web run test
npm.cmd --prefix web run build
docker compose --env-file .env.example config
docker compose --env-file .env.example -f compose.prod.yaml build
```

## Go code tour

Start at `cmd/server/main.go`. A Go executable uses `package main` and begins at
`func main()`. This file loads configuration, opens the store, constructs the HTTP
handler, and shuts the server down when Docker sends a termination signal.

`internal/config/config.go` introduces a `struct`, a named group of fields:

```go
type Config struct {
    Addr    string
    DataDir string
}
```

Functions commonly return a value and an error. The caller handles failure
explicitly:

```go
cfg, err := config.Load()
if err != nil {
    return err
}
```

`:=` declares and initializes local variables. `if err != nil` is Go's ordinary
error-handling style; exceptions are not used for routine failures.

`internal/store/store.go` owns SQLite and transactions. Methods such as
`CreateUser` have a receiver (`s *Store`), which means the function operates on a
store value. `defer rows.Close()` schedules cleanup when the surrounding function
returns, including error paths.

`internal/store/users.go` keeps account lifecycle changes explicit. Methods return
`(User, error)`, and the caller must check the error before using the user. Rename
keeps the numeric account and root-folder IDs unchanged, while archive changes only
account state. Restore writes the new password hash and active state together.

`internal/httpapi/api.go` uses the standard library's `http.ServeMux`. Handlers
receive an `http.ResponseWriter` interface and an `*http.Request` pointer. Small
middleware functions wrap handlers to add authentication and admin checks. Go's
HTTP server already handles independent requests concurrently, so this slice does
not add unnecessary goroutines.

`internal/auth` keeps password and cookie details out of handlers. Capitalized
names such as `HashPassword` are exported to other packages; lowercase names are
private to their package. Tests beside each package demonstrate normal usage and
boundary cases.

`internal/store/migrations/001_initial.sql` is embedded into the Go binary. On
startup, unapplied numbered migrations run once and are recorded in
`schema_migrations`. Schema changes should be new migration files; an applied
migration must never be edited in place.

## Current folder slice

`internal/library/names.go` shows why display and comparison values are separate.
The UI keeps the capitalization entered by a user, while the normalized value is
used by SQLite to reject case-insensitive and canonically equivalent duplicates.

`internal/store/folders.go` demonstrates recursive common table expressions and
transactions. A cross-library admin move changes a complete subtree's ownership
as one unit; an error rolls back every change. Normal users cannot discover or
mutate another user's IDs even if they manually alter a request.

## Current implementation slice

The public sharing and playback slice is now implemented: ready MP4s
with byte ranges, expose posters and minimal server-rendered clip pages, generate
Discord/Open Graph metadata from immutable public IDs, with authenticated
preview/copy-link actions. Non-ready and failed clips remain publicly invisible.

Clip title editing is available through the authenticated clip API, with normal
ownership checks and duplicate-title validation. Ready clips can be moved into
the recoverable state with `DELETE /api/clips/{id}`.
The trash API and compact recycle-bin screen support clips and complete folder
subtrees. Normal users see their own last 30 days; the administrator sees all
owners' last 90 days. Restoring a folder restores its nested folders and clips,
and public clip links remain disabled until restoration succeeds.

## Current retention slice

`internal/retention/retention.go` demonstrates a small long-running Go worker.
It runs a function once, then uses a `time.Ticker` and `select` to choose between
the next hourly pass and Docker's cancellation signal. No extra worker framework
is needed.

Permanent deletion passes a small callback into the store transaction. In Go, a
function can be passed as a value just like other data. Holding the transaction
while that callback removes media prevents a restore request from racing the
purge. If file removal fails, the transaction rolls back; retrying is safe because
removing an already-absent directory is considered successful.

## Reusable frontend interactions

Repeated library interactions live in focused React components rather than being
copied into each screen:

- `components/clip-preview.tsx` owns the thumbnail play affordance and video dialog;
- `components/folder-picker-dialog.tsx` owns destination browsing for uploads and moves;
- `components/item-management-bar.tsx` owns the shared Rename/Move/Delete card bar
  and its administrator-only Copy variant;
- `components/search-dialog.tsx` owns global search, result cards, and clip previews;
- `components/ui/modal.tsx` provides the common portal-based modal shell.

Each parent screen supplies data and action callbacks. This is ordinary React
composition: the shared component owns presentation and local interaction state,
while API mutations remain in the feature that knows what is being changed.

## Current user-administration slice

The administrator user manager supports username/limit edits, password reset,
disable/enable, archive/restore, and opening retained libraries. Archived accounts
are hidden from the main library chooser but remain visible in this admin-only
manager. Disable and archive are checked on every authenticated request, so an
already-open normal-user session stops working immediately; no session table or
background invalidation machinery is needed.

## Current search slice

`internal/store/search.go` uses one recursive SQLite common-table expression to
build logical display paths and search nested folder names and clip titles. A
normal request supplies an owner ID pointer; the super-admin supplies `nil` to
search all active and archived libraries. The query excludes trash before matching
and reads one extra row to determine whether the 100-result response was truncated.

The app-shell Search action is available from every authenticated view. Folder
results open that folder, clip results open their containing folder, and ready
clips can be previewed or copied without leaving the search dialog.

## Current folder-copy slice

`internal/store/folder_copy.go` prepares a stable description of the source tree,
then rechecks that description inside the commit transaction. `reflect.DeepEqual`
compares nested structs, slices, and pointer values so a concurrent rename, move,
upload, or deletion aborts the copy instead of committing a stale partial tree.

`CommitFolderCopy` accepts a `func() error` callback. In Go, functions are values,
so the HTTP layer can pass the media-publish operation into the store without the
store package learning filesystem paths. Media is copied to `/data/temporary`
before the transaction; the callback atomically renames it into `/data/media`
immediately before SQLite commits. Error paths roll back records and remove all
new files. Every copied clip receives a new random public ID and storage ID.

## Current folder-deletion summary slice

`internal/store/folder_deletion.go` uses one recursive SQLite common-table
expression to count the selected folder, all active descendants, every active
clip in that tree, and their finalized stored bytes. `COALESCE` converts an empty
SQL sum into zero; this is the SQL equivalent of supplying a default value.

The delete dialog requests those totals only when it opens. Until the request
succeeds, both the acknowledgement checkbox and destructive action remain
disabled. Loading and failure states stay inside the styled modal, including an
inline Retry action. Store and HTTP tests cover deep nesting, excluded trash,
outside-subtree content, the protected account root, admin access, normal-user
ownership boundaries, and the single-connection timeout regression.

## Current job-control slice

`internal/store/job_controls.go` keeps cancellation transactional: one SQLite
transaction changes both the job and its reserved clip to `cancelled` and releases
the disk reservation. The HTTP layer then calls the existing retry-safe
`media.RemoveStoredAssets` function. Calling Cancel again is allowed so an earlier
filesystem cleanup failure can be retried.

The processor treats `store.ErrJobCancelled` as a control signal rather than a
processing failure. Each FFmpeg progress write checks that the job is still
active; cancellation kills the subprocess. The final database commit also checks
affected-row counts, preventing an output directory from becoming ready if
cancellation wins the race immediately before publication.

Before a durable job exists, the upload dialog uses an `AbortController`. This is
a browser API whose signal tells `XMLHttpRequest` to close the in-flight request.
The Go request context is then cancelled, and the upload handler's deferred cleanup
removes its temporary source, reserved clip, and capacity reservation. Both this
path and card-level job cancellation require an in-app confirmation.

## Current upload-timeout slice

`ProcessingJob.UploadStartedAt` is loaded from the clip timestamp created when
the upload reservation begins. The processor derives an absolute deadline by
adding one hour to that timestamp, so queueing and service restarts cannot reset
the clock. `context.WithDeadlineCause` supplies both a cancellation signal for
FFmpeg and a specific timeout cause the cleanup path can distinguish from normal
Docker shutdown.

An expired context cannot be reused for SQLite writes. The timeout handler
therefore creates a short, fresh cleanup context, removes temporary and final
media, releases the reservation through `FailProcessing`, and stores only the
safe `upload_timeout` notice. Tests use an already-expired timestamp rather than
waiting an hour, and verify that recovery preserves the original start time.
