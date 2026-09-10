# Shared-dialog backdrop dismissal

## Status

**Completed 2026-09-06.** All custom overlays now use the shared modal, whose
topmost dialog handles safe backdrop dismissal, Escape, focus restoration, and
nested-dialog accessibility.

## Intended behavior

All in-app dialogs close through the same `onClose` path when the user clicks or
taps the blurred backdrop. This is dismissal only: it never submits, deletes,
uploads, or purges.

- Only the frontmost dialog responds.
- Pointer down and pointer up must both occur on that dialog's backdrop.
- Escape, X, and backdrop use the same close path, preserving any existing
  discard/cancel confirmation.
- Every existing custom overlay migrates to the shared `Modal` before this is
  enabled.

## Decisions required

1. **Confirmed:** while a non-cancellable mutation is committing, ignore Escape,
   X, and backdrop. While an upload/edit is cancellable, route all three through
   its existing cancel/discard confirmation.
2. **Confirmed:** when a nested confirmation is open, make its parent inert and
   hidden from assistive technology until the child closes.

## Verification

- Component/browser tests cover mouse and touch drag boundaries, nested dialogs,
  busy dialogs, focus restoration, and exactly-once dismissal.
- No destructive action runs because of a backdrop click.
