# Main spec 91: Explorer multi-select and bulk actions

## Status

**Optional future work — discovery required.** Explicitly excluded from main
spec 2. Do not implement from this placeholder.

## Motivation

Selecting several folders and clips could make repetitive organization and
cleanup faster. It also introduces a distinct interaction and safety model that
should not be hidden inside the file-explorer visual overhaul.

## Candidate scope

- selection checkboxes plus desktop modifier/range selection;
- keyboard-only selection and a discoverable select-all command;
- touch selection without relying on hover, right-click, or undiscoverable
  long-press behavior;
- a bulk action bar for actions valid across the complete selection;
- move, recycle-bin deletion, and any other explicitly accepted bulk operations;
- selection behavior across pagination, sorting, view changes, folder
  navigation, refreshes, processing-state changes, and deleted items;
- administrator authorization when a selection spans acting contexts or owner
  libraries.

## Decisions required

- Whether selection may span loaded pages, folders, or owner libraries.
- Whether folders and clips may be selected together.
- Which actions support mixed selections and how partial eligibility is shown.
- Whether a folder selection implicitly includes its descendants and how
  recursive totals are calculated before destructive actions.
- Whether selection survives navigation, refresh, or a grid/list switch.
- How range selection works when the active sort changes.
- How bulk failures report partial success versus atomic rollback.
- Mobile, keyboard, focus, screen-reader, and live-region behavior.

## Safety boundary

No bulk destructive action is acceptable without an authoritative server-side
summary, explicit confirmation, per-item authorization, repeat-safe behavior,
and tests for stale or concurrently changed selections.
