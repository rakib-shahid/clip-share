# Readable media-layout compatibility repair

## Status

**Completed 2026-09-06.** The layout-aware copy and restore reconciliation
repair is implemented and covered by targeted regression tests.

## Objective

Keep the human-readable on-disk hierarchy synchronized with the logical
owner/folder/title hierarchy while preserving every public clip URL, including
Discord embeds. Physical paths are never public identifiers.

## Required behavior

- Resolve finalized assets through one layout-aware reference; no new or changed
  operation may construct `media/<storage-id>` directly.
- Folder copy stages outside `media/` and publishes into the resolved readable
  destination. Its source can be in any existing readable hierarchy.
- Reconcile after every successful operation that creates a ready clip or changes
  owner, folder ancestry, or title: processing publication, folder/clip rename or
  move, copy, and trash restore.
- Public `/c/<public-id>` and `/m/<public-id>/*` continue resolving by stable
  public ID and remain valid after every physical relocation.
- Purge and rollback delete only assets belonging to their explicit storage IDs
  and request staging roots.

## Verification

- Regression tests cover copy, restore, rename, move, publish, public playback,
  trash preview, and purge against readable paths.
- Existing public IDs resolve the same clips before and after each operation.
- No flat `media/<storage-id>` construction remains outside the central legacy
  fallback/resolver implementation.
