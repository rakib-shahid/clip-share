# Version-one completion record

## Status

Reconciled September 5, 2026. The version-one implementation and its acceptance
criteria are complete. This document is retained as a historical completion
record; future product work is tracked in its owning proposed feature spec.

## Completed implementation slices

### 1. Media-input validation hardening

- [x] Explicitly reject encrypted media rather than relying on a later FFmpeg error.
- [x] Add table-driven FFprobe coverage for corrupt input, attachment-only video,
  missing/invalid duration, malformed frame rate, over-8K dimensions, and over-240
  FPS sources.
- [x] Test the normal-user 110% stored-output threshold (including the exact
  boundary and administrator bypass); the processing path applies this check
  before assets can be published as ready.
- [x] Reconciled the upload validation and limit acceptance criteria.

### 2. Separately retained library deletion

- [x] Add an administrator-only action for deleting an archived account's retained
  library separately from account archival.
- [x] Move the complete root subtree into administrator-visible recoverable trash
  without deleting the account record immediately.
- [x] Disable every public URL in that library at the same transaction boundary.
- [x] Define and test restore/purge behavior for a whole retained library, then expose
  the deliberately confirmed action in user administration.

### 3. Responsive and accessibility completion

- [x] Add a browser UI test harness with automated accessibility checks.
- [x] Add focus trapping/restoration and Escape handling to nested and ordinary modals.
- [x] Verify keyboard access, visible focus, touch targets, status text, and non-color
  state communication across every primary flow.
- [x] Exercise narrow mobile and desktop layouts and fill any missing loading, empty,
  and retryable error states.
- [x] Complete a manual keyboard review after automated checks pass.

## Completed verification and reconciliation

### 4. Clip-management invariants

- [x] Add API/store tests for rename, move, title conflicts, normal-user ownership,
  administrator cross-library moves, and confirmed clip deletion.
- [x] Prove rename/move preserve storage identity and public URL.
- [x] Prove cross-owner folder moves recursively change ownership without changing
  existing public URLs.

### 5. Public sharing and authorization contract

- [x] Add HTTP tests covering anonymous API rejection, random public IDs, generic 404
  behavior for every unavailable state, deletion invalidation, and ready-only
  reachability.
- [x] Assert `Cache-Control: no-store`, robots headers, byte ranges, escaped titles,
  absence of owner/folder/download UI, and the accepted native player defaults.
- Verify Home routing for anonymous, normal-user, and administrator sessions.

### 6. Identity, session, and log-safety contract

- [x] Extend setup tests with an incorrect-token case and cookie attribute assertions.
- [x] Test explicit logout, non-persistent cookie behavior, generic unlimited login
  failures, 24-hour expiry, and signature tampering. Shared-tab behavior remains
  documented as a browser limitation.
- [x] Capture structured logs in tests and prove passwords, setup tokens, session
  cookies, and CSRF tokens never appear.
- Reconcile already-implemented identity acceptance criteria after those checks.

### 7. UI-state acceptance reconciliation

- [x] Upload transfer and processing are visually distinct, destructive actions use
  the accepted confirmation levels, and administrator library context is
  unmistakable in the shared UI states.
- [x] Loading, empty, retryable-error, progress, and failure states now expose
  explicit status/alert semantics and readable labels.
- [x] Add browser assertions and a manual keyboard review after a browser harness is
  added; these remain release verification rather than runtime behavior.

## Completed compatibility and release work

### 8. Discord/browser compatibility pass

- [x] Exercise representative small and near-limit outputs in Discord desktop, web,
  iOS, and Android where available.
- [x] Verify embed title refresh expectations, poster display, playback, seeking, and
  the accepted H.264/AAC MP4 profile in real clients.
- [x] Run the final production Compose smoke test through the Komodo-style deployment,
  document backup/restore practice for SQLite plus media, and remove the one-time
  setup token after installation.

## Completed successor work

The upload editor and multi-track audio workflow was subsequently implemented;
its final contract is recorded in
[`05-video-editing-and-audio.md`](../05-video-editing-and-audio.md).
