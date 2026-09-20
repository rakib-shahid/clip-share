# Specification map and implementation queue

Read this file first. Every feature group has one permanent integer ID and one
`main.md`; implementation work is decomposed into decimal child specs.

## Required topology

```text
specs/
  completed/
    1-editor-gui/
      main.md
      slices/
        1.1-editor-entry-and-transfer.md
        1.2-source-analysis.md
  to-be-implemented/
    N-feature-name/
      main.md
      slices/
  undecided/
    N-feature-name/
      main.md
  optional/
    90-feature-name/
      main.md
```

The integer is a hard, permanent main-spec ID. A child ID repeats its parent
integer (`2.3` belongs only to main spec `2`). Never renumber an accepted or
completed spec, recycle its number, or use a decimal ID for a separate feature.
Use short kebab-case names. Deferred optional ideas use reserved high numbers so
they do not consume the next active main-spec ID.

Each main spec defines the user outcome, goals, non-goals, compatibility
boundary, global decisions, child-slice order, and overall acceptance criteria.
Its `slices/` directory contains the executable detail. Make every slice the
smallest independently implementable and testable unit that leaves the tree
working. Prefer one behavior, state transition, endpoint, component boundary,
or helper function. A slice may intentionally add only one function to an
existing file.

Every ready slice must state:

- status and parent main-spec ID;
- exact outcome and non-goals;
- accepted choices and invariants;
- affected files, APIs, data, UI states, and dependencies where known;
- errors, authorization, accessibility, cleanup, and compatibility edge cases
  that apply;
- focused automated tests and observable acceptance criteria.

If those details do not fit cleanly, split the slice again. Do not use a broad
task such as “build the UI,” and do not hide an unresolved product choice inside
an implementation slice.

## Current map

- [`0-core-platform`](completed/0-core-platform/main.md) — completed version-one
  platform and historical feature contracts.
- [`1-editor-gui`](completed/1-editor-gui/main.md) — completed upload-time trim
  and audio-mix editor, decomposed retrospectively into focused slices.
- [`2-file-explorer-ui`](completed/2-file-explorer-ui/main.md) — completed
  file-explorer and application-wide shadcn interaction overhaul.
- [`90-ci-cd`](optional/90-ci-cd/main.md) — optional future CI/CD quality gate.
- [`91-explorer-multi-select`](optional/91-explorer-multi-select/main.md) —
  optional future multi-selection and bulk-action discovery.

## Lifecycle

- `undecided/` contains a proposal with material unresolved choices. Do not
  implement it.
- `to-be-implemented/` contains fully decided work. Implement decimal slices in
  dependency order, completing their focused tests before advancing.
- `completed/` is authoritative regression context, not work to redo.
- `optional/` contains deliberately deferred work outside the active queue.

Move the whole numbered feature directory when its lifecycle changes; its hard
ID and internal links remain unchanged. Preserve completed contracts. If new work
must change one, record the compatibility change and regression coverage in the
new active main spec and its smallest affected slice.

## Decision and completion rule

Discuss goals, non-goals, user flows, edge cases, and acceptance criteria before
moving a spec out of `undecided/`. A slice is done only when implementation,
proportionate automated tests, supported Docker Compose verification, and its
documentation agree. Intentional discoveries update the active slice; they never
silently rewrite completed history.
