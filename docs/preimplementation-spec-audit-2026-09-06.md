# Pre-implementation specification audit — 2026-09-06

## Outcome

The editor/audio, backdrop-dismissal, password-change, and engineering-quality
specifications are directionally compatible with the product. Implementation
must not start until the **blocking decisions** below are confirmed and the
editor data/job lifecycle is reconciled with the existing upload pipeline.

The completed media-layout migration is a hard compatibility baseline: public
URLs remain keyed by `public_id`, while on-disk paths are now human-readable.
No new feature may reintroduce a direct `media/<storage-id>` assumption.

## Evidence checked

- Routing and authentication in `internal/httpapi/api.go`.
- Upload/job lifecycle in `internal/store/uploads.go`, `internal/store/processing.go`,
  and `internal/media/processor.go`.
- Media layout, public serving, trash, and folder copy in `internal/media`,
  `internal/httpapi`, and `internal/store`.
- All current dialog implementations in `web/src`.
- The accepted pending feature specifications and the proposed engineering
  quality specification.

## Compatibility blockers

### 1. Centralize the readable media layout before feature work

`internal/media/copy.go` still opens and publishes assets using the former flat
`media/<storage-id>` location. A folder copy will therefore fail after the
completed layout migration. Folder copy is the confirmed instance, but the
invariant needs to apply to every finalized-media operation.

Required contract:

1. A single layout-aware asset reference resolves the current source directory
   from a clip's stable storage ID and logical owner/folder/title path.
2. Every operation that creates, copies, deletes, serves, or moves finalized
   assets uses that reference: processing publish, folder copy, public serving,
   trash preview/purge, restore, and reconciliation.
3. Any successful database operation that creates a ready clip or changes its
   owner, folder ancestry, or title triggers reconciliation before reporting
   success. In particular this includes copy and restore, not only rename/move.
4. A copy stages outside `media/`, then publishes to the resolved destination;
   rollback removes only the staged/destination assets for that request.
5. Public `/c/<public-id>` and `/m/<public-id>/*` continue to look up by public
   ID and never expose the physical path.

This is the first engineering-quality implementation item and a prerequisite to
the editor because final editor publication must use the same primitive.

### 2. Resolve the editor's relationship to existing uploads/jobs

Today `POST /api/uploads` immediately creates a normal clip/job, the worker
claims FIFO queued jobs, and a successful job is retained as `ready`. The editor
spec repurposes the route into a temporary session and says final processing-job
records are deleted after success. Those models cannot be combined implicitly.

The implementation needs one explicit model:

- **Recommended:** introduce editor-session and preview-job records separate
  from the existing `jobs` table. Upload creates a non-library placeholder plus
  an editor session. Finalize creates one normal final-render job containing an
  immutable recipe snapshot. On success, delete editor/session/preview/source
  data but preserve the normal final job according to the existing job contract.
  The ready clip is then indistinguishable from a current ready upload.
- If final jobs must be deleted, change the existing jobs/UI/API retention
  contract deliberately and document how current queued/failed/ready job views
  behave. This is not recommended for the editor alone.

The exact state mapping, foreign keys, reservation owner, cancellation path,
and cleanup transaction must be specified with the selected model.

### 3. Define queue priority and resource bounds

The current dispatcher claims FIFO work and starts available claims without an
application-level worker concurrency limit. "Final renders take priority over
previews" needs a measurable rule.

Required decisions:

- Is priority claim-order only (recommended), or may a final render cancel an
  in-progress preview? Recommended: never preempt an FFmpeg process; cancel
  only queued/replaced previews.
- What is the maximum number of concurrent FFmpeg processes and previews? A
  recommended safe initial rule is one process total, with a final render chosen
  before queued previews. This can be made configurable later after measurement.
- What bounded preview reservation is included in admission control? Define a
  fixed maximum (for example 25 MB) in addition to source/final reservation so
  a preview cannot exceed free-space accounting.

### 4. Make the non-resumable session rule precise

Resolved contract: editor sessions are short-lived and resumable, not retained
drafts. The same authenticated actor can re-select Trim with the same normalized
title, destination, and source byte size to restore saved edits during the
five-minute heartbeat lease. Different identities may have concurrent sessions.
Explicit discard removes one immediately; otherwise the once-per-minute server
sweep is authoritative and removes it after five minutes without a heartbeat.

### 5. Separate initial upload time from final-render time

Current processing derives its one-hour deadline from `clips.created_at`. The
accepted editor behavior grants up to one hour for source transfer and a fresh
one hour after finalization. Store an explicit `finalized_at`/job deadline and
have final rendering use it; it must never inherit the source-upload timestamp.

### 6. Complete the render recipe contract

The editor spec has the correct intent, but the following details need to be
made implementation-testable:

- Define an exact filter graph policy: `trim`/`atrim`, timestamp reset,
  per-track `volume`, `amix` with explicit `normalize=0`, then a `-1 dBFS`
  limiter. State the concrete limiter setting supported by the shipped FFmpeg.
- Specify the stereo downmix matrix/FFmpeg policy for mono, stereo, and
  multichannel input, and whether an input whose selected stream cannot be
  decoded fails finalization or is marked unusable during analysis.
- Define the final duration authority (video trim) and the rules for shorter
  selected audio: silence/black fill is already accepted; state whether final
  output uses `-shortest` (recommended: no) and how audio is padded/truncated.
- Define the audio-free output explicitly: omit an audio stream, retain video
  duration, and size-target without audio allowance.
- Pin/document the FFmpeg version/capabilities used by the production image and
  add fixtures for supported containers, codecs, orientations, stream counts,
  silence, and multichannel audio.

### 7. Make editor HTTP lifecycle idempotent

Define response bodies and repeat behavior for upload analysis, preview,
finalize, cancel, and expired sessions. At minimum:

- Preview should expose its revision/state/availability rather than overloading
  the media byte endpoint for status.
- Finalize needs an idempotency rule: a repeated request for the same locked
  revision returns the existing final job; a different/stale revision returns a
  documented conflict.
- Cancel and cleanup must safely race with preview/final workers, source probing,
  and client aborts without publishing an orphaned clip or returning a 500.
- The initial analysis result needs a terminal invalid-source state and retry
  policy; all failure/cancel paths delete the session rather than producing the
  existing failed-library notice.

## Dialog and accessibility gaps

The shared `Modal` currently implements focus/Escape only. Upload is a custom
overlay and dialogs in `App.tsx` include additional custom overlays. Before
backdrop dismissal:

1. Migrate all of them to `Modal` so there is exactly one backdrop/focus
   implementation.
2. Add pointer-ID-aware backdrop dismissal (down and up both on the same
   backdrop) and test it for mouse/touch, nested dialogs, and exactly-once close.
3. Define busy behavior. Recommended: a modal may be dismissed while editing or
   uploading only through its existing `requestClose` path; when a non-cancellable
   mutation is already committing, ignore Escape/X/backdrop until it finishes.
4. When a nested confirmation is open, make the parent dialog inert/hidden from
   assistive technology and restore it when the child closes. Merely increasing
   z-index leaves two `aria-modal` dialogs exposed.

The editor must use this same close contract: edited state opens its discard
confirmation; an entire upload cancel discards immediately as already accepted.

## Password-change gaps

The endpoint, fields, CSRF requirement, and intentionally unchanged sessions
are specified. The remaining choices are small but should be settled before
implementation:

- **Placement:** the current shell has a Logout action but no account menu.
  Recommended: add a compact account menu containing Change password and Logout
  for every active user, with admin controls remaining separate.
- **Same password:** recommended to reject `newPassword == currentPassword` with
  a safe `new_password_unchanged` validation error; this avoids a misleading
  success and unnecessary hash work.
- **Concurrent change/reset:** use a compare-and-swap update on the verified
  stored hash. If an administrator reset or another self-change wins first,
  return the safe current-password failure instead of overwriting it.
- **Success response:** recommended `204 No Content`; clear browser fields and
  transient UI state before closing. Use `autocomplete="current-password"` and
  `autocomplete="new-password"`.

## Engineering-quality spec corrections

- The release gate was updated to remove "cold-start migration compatibility":
  the one-time media-layout migration is complete and its command was removed.
  The ongoing requirement is readable-layout compatibility.
- `internal/testutil` is a convention rather than a compiler-enforced
  test-only boundary. Keep it limited to test files by code review, or rename it
  `internal/testkit` if that better reflects Go's build rules.
- Choose the supported test runner environment for Vitest/Playwright. Recommended:
  run unit/component tests in the web Node environment and Playwright in a
  separate CI/Compose test image, never in the production image.
- The UI spec currently marks automated accessibility checks complete, but the
  repository has no established frontend component/E2E runner. Treat that check
  as a historical/manual assertion until the new test tooling proves it in CI;
  do not claim a browser suite exists before it is added.

## Implementation order after confirmation

1. Characterize and repair readable-layout operations (folder copy, restore,
   publish, purge) with tests; preserve all public URLs.
2. Introduce frontend testing and the shared dialog primitive; migrate overlays
   and implement backdrop behavior.
3. Implement self-service password change with store CAS, HTTP, component, and
   browser tests.
4. Add the editor schema, explicit lease/job model, queue limits/priority, and
   cleanup worker.
5. Implement editor upload/analysis, recipe validation, preview, final renderer,
   and end-to-end media fixtures.

No implementation item may bypass the readable-layout resolver or leave editor
source/preview/session data after cancellation, expiry, failure, or successful
finalization.
