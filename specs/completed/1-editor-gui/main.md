# Main spec 1: Editor GUI

## Status

**Completed September 6, 2026.** This is the implemented pre-finalization trim
and audio-mix editor. It does not edit already-published clips.

## Outcome

An uploader can optionally open a private editor, select one continuous trim
range, configure up to eight source audio tracks, render a private preview, and
finalize one browser-compatible MP4. Temporary sources, previews, recipes, and
session records are cleaned after completion, cancellation, failure, or expiry.

## Boundaries

- **In scope:** upload-time editing, accurate trim, per-track include/gain,
  private previews, revision-safe autosave/finalize, recovery, and cleanup.
- **Out of scope:** edits to ready clips, multiple cuts, joining, captions,
  crop/rotation controls, effects, external audio, published alternate tracks,
  and retained edit history.
- The ready clip keeps its existing public-link, poster, size-policy, and storage
  contracts from main spec 0.

## Child slices

Implementation knowledge is indexed as focused completed slices:

1. [`1.1`](slices/1.1-editor-entry-and-transfer.md) — optional editor entry and source transfer
2. [`1.2`](slices/1.2-source-analysis.md) — source probing and descriptors
3. [`1.3`](slices/1.3-session-lifecycle.md) — private session lifecycle
4. [`1.4`](slices/1.4-recipe-validation.md) — canonical recipe validation
5. [`1.5`](slices/1.5-autosave-and-conflicts.md) — revision-safe autosave
6. [`1.6`](slices/1.6-editor-shell-accessibility.md) — modal shell and accessibility
7. [`1.7`](slices/1.7-timecode-and-trim-controls.md) — precise trim controls
8. [`1.8`](slices/1.8-filmstrip-and-timeline-navigation.md) — filmstrip and timeline navigation
9. [`1.9`](slices/1.9-source-playback-and-fallback.md) — local playback and proxy fallback
10. [`1.10`](slices/1.10-audio-track-controls.md) — audio-track controls
11. [`1.11`](slices/1.11-private-render-preview.md) — private rendered previews
12. [`1.12`](slices/1.12-finalization-lock.md) — preview warning and finalize lock
13. [`1.13`](slices/1.13-final-media-render.md) — authoritative final render
14. [`1.14`](slices/1.14-cleanup-recovery-and-capacity.md) — cleanup, recovery, and capacity
15. [`1.15`](slices/1.15-compatibility-verification.md) — compatibility verification

The preserved [`accepted-contract.md`](accepted-contract.md) contains the full
decision record and remains the detailed fallback when a retrospective slice is
silent. Any future editor change belongs to a new main spec or a newly numbered
slice; do not silently rewrite the completed contract.
