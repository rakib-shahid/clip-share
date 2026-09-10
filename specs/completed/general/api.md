# API contract

## Status

Accepted structure. Field-level schemas may be refined with implementation without
changing the accepted behavior.

## Conventions

- Authenticated endpoints live under `/api` and use JSON except uploads, which use
  `multipart/form-data`.
- JSON uses `camelCase`; timestamps are UTC RFC 3339 strings; sizes are integer bytes.
- Mutating authenticated requests must be same-origin and pass CSRF checks.
- CORS is disabled; frontend, API, public pages, and media share one origin.
- Creation returns `201`; successful deletion returns `204`.
- Validation uses `400`, unauthenticated `401`, unauthorized/not-owned `404`,
  conflicts `409`, oversized bodies `413`, and unexpected failures `500`.
- Cross-user resources return 404 to normal users rather than revealing existence.

```json
{
  "error": {
    "code": "title_conflict",
    "message": "A clip with this title already exists in that folder.",
    "field": "title"
  }
}
```

`code` is stable for frontend logic and `field` is optional. Responses never
expose internal paths, SQL, command lines, stack traces, or secrets.

CSRF protection combines the `SameSite=Lax` session cookie, strict same-origin
`Origin` validation, and a signed CSRF token in a custom request header for every
mutation.

Responses apply a restrictive Content Security Policy,
`X-Content-Type-Options: nosniff`, a privacy-preserving Referrer Policy, and frame
restrictions. Production cookies are Secure behind the HTTPS-aware trusted proxy
configuration.

## Browser routes

| Route | Access | Purpose |
| --- | --- | --- |
| `/setup` | Only before setup, plus setup token | Create sole super-admin |
| `/login` | Anonymous | Login screen |
| `/app/*` | Authenticated | React application fallback |
| `/c/{publicID}` | Public when ready | Minimal player HTML and OG metadata |
| `/m/{publicID}/video` | Public when ready | MP4 with byte ranges |
| `/m/{publicID}/poster` | Public when ready | Poster JPEG |

Unavailable public IDs/states use one generic 404. `/` sends authenticated users
to their applicable explorer and others to `/login`.
Direct `/app/*` requests serve the embedded React fallback when authorized and
redirect to login when not authorized.

## Resources

### Setup and authentication

- `GET /api/setup/status`
- `POST /api/setup`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

Login sets the accepted signed, non-persistent, 24-hour cookie. Setup succeeds
exactly once.

### User administration

- `GET /api/users`
- `POST /api/users`
- `PATCH /api/users/{userID}`
- `POST /api/users/{userID}/password`
- `POST /api/users/{userID}/disable`
- `POST /api/users/{userID}/enable`
- `POST /api/users/{userID}/archive`
- `POST /api/users/{userID}/restore`
- `DELETE /api/users/{userID}/library`

Only the super-admin may call these. The protected super-admin cannot be demoted,
disabled, archived, or deleted.

Authentication responses also include `publicBaseURL`, the canonical configured
origin used by the frontend when copying share links. This remains the public
domain even when the browser is connected through the LAN origin.

`PATCH /api/users/{userID}` accepts `username` and `storedFileLimitMb` together
and returns the updated user. The password endpoint accepts `password` and returns
`204`. Disable, enable, and archive return the updated user. Restore accepts a
required replacement `password`, makes the account active, and returns the
updated user. Password values and hashes are never returned.

Deleting a library is allowed only while a normal account is archived. It returns
`204`, moves the account root to administrator-only recycle bin, and invalidates
all public clip URLs in that subtree atomically. Restore the root from recycle bin
before restoring the account.

### Libraries

- `GET /api/folders/{folderID}` returns details and the first 60 children.
- `POST /api/folders` creates a folder.
- `PATCH /api/folders/{folderID}` renames it.
- `POST /api/folders/{folderID}/move` moves its subtree.
- `POST /api/folders/{folderID}/copy` makes an independent subtree; admin only.
- `GET /api/folders/{folderID}/deletion-summary` returns recursive deletion totals.
- `DELETE /api/folders/{folderID}` trashes its subtree.

Pagination uses an opaque `cursor` and returns `nextCursor` or `null`. Folders
sort alphabetically before newest-first clips.

Folder copy accepts `{ "destinationFolderId": 123 }` and synchronously returns
the new top-level folder after its complete subtree and media have committed. A
name collision returns `409 copy_conflict` with a `conflicts` array of logical
display paths. A changing source returns `409 copy_changed`, a source containing
non-ready clips returns `409 copy_not_ready`, and copying into the source or one
of its descendants returns `409 invalid_folder_copy`. Account library roots are
not copy sources.

The deletion-summary response is `{ "folderCount", "clipCount", "totalItems",
"storedBytes" }`. `folderCount` includes the selected folder and all active
descendants. Clip totals include every non-cancelled active clip in that subtree;
a clip without a finalized stored size contributes zero bytes. Trashed content is
excluded. The endpoint applies the same ownership and protected-root rules as
folder deletion, so a normal user cannot inspect another user's subtree.

### Uploads, clips, and jobs

- `POST /api/uploads` accepts one video plus title, destination, and compression.
- `GET /api/uploads/resumable?destinationFolderId={id}&sourceSizeBytes={bytes}&title={title}` returns a
  matching editor session saved within its five-minute inactivity lease, or
  `204` when the client should start a new upload.
- `GET /api/jobs/{jobID}` returns state, progress, and a safe error.
- `DELETE /api/jobs/{jobID}` cancels non-ready work.
- `DELETE /api/jobs/{jobID}/failure` dismisses a media-free failure record.
- `GET /api/clips/{clipID}` returns an authorized management record.
- `PATCH /api/clips/{clipID}` changes its title.
- `POST /api/clips/{clipID}/move` changes its one parent.
- `DELETE /api/clips/{clipID}` trashes a ready clip.

Upload returns a job ID and reserved clip summary. The client polls about every two
seconds until ready, failed, or cancelled.

The Trim flow checks the resumable endpoint before transferring the selected
file. Matching is scoped to the authenticated acting user, exact destination,
normalized title, and exact source byte size. It refreshes that session's heartbeat and returns its
saved trim/audio recipe. Users may hold multiple editor sessions for different
folder/title identities; stale editing sessions are removed after five minutes
without a heartbeat.

The upload's one-hour wall-clock deadline begins with its initial server-side
reservation and is carried into durable processing. Timeout failure uses the safe
`upload_timeout` error code and normal failed-notice response; no internal command
or path details are exposed.

`DELETE /api/jobs/{jobID}` accepts queued, validating, or processing work and is
idempotent after cancellation so filesystem cleanup can be retried. It marks the
job and reserved clip cancelled, releases its capacity reservation, requests that
an active FFmpeg process stop, and removes source, partial, and unpublished final
assets. Ready and failed jobs return `409 job_not_cancellable`.

`DELETE /api/jobs/{jobID}/failure` accepts only a failed, media-free job. It
deletes that private notice and its reserved failed clip record. Other states
return `409 job_not_failed`. Both endpoints return another user's job as `404` to
a normal user; the super-admin may control any job.

### Trash and search

- `GET /api/trash` returns role/retention-visible trash.
- `GET /api/trash/clip/{id}/video` and `/poster` stream authenticated preview
  media only while that actor may still see the trash item.
- `POST /api/trash/{kind}/{id}/restore` restores a clip or folder subtree.
- `DELETE /api/trash/{kind}/{id}` permanently purges; admin only.
- `GET /api/search?q=...` returns at most 100 authorized matches.

Search returns `{ "results": [...], "truncated": false }`. Each result includes
`kind` (`folder` or `clip`), its display `name`, owner identity, a logical
slash-separated display `path`, and the `folderId` the explorer should open.
Clip results additionally include their public ID, processing state, and stored
size. The display path is library metadata, never a physical storage path. Queries
are trimmed and limited to 200 characters; an empty query returns an empty result
array without scanning the library. `truncated` is true when more authorized
matches exist than the 100 returned.

`kind` is explicitly `clip` or `folder`. Every operation rechecks owner/role.
Trash preview does not reactivate or reuse the public share URL.

## Health

- `GET /healthz`: process responds.
- `GET /readyz`: configuration, SQLite, `/data`, FFmpeg, and ffprobe are ready.

Anonymous health responses never disclose paths, versions, counts, or secrets.
