# Specification map and implementation queue

Specifications are organized by implementation status. Read this file first,
then read only the folder relevant to the current step.

## 1. Completed baseline

[`completed/`](completed/) is the implemented version-one contract and its
completion record. It is authoritative regression context, not work to redo.

- Numbered feature specs live directly in `completed/`.
- General product/API/storage/deployment/operations contracts live in
  [`completed/general/`](completed/general/).
- Completed feature behavior lives in `completed/features/`.
- The completed one-time media-layout migration is recorded in
  `completed/general/data-and-storage.md`.
- The completed pre-finalization trim and multi-track audio editor is recorded in
  `completed/05-video-editing-and-audio.md`.

## 2. Ready implementation queue

[`to-be-implemented/`](to-be-implemented/) contains only work whose behavior is
fully decided. Implement files in numeric order, with their stated tests before
advancing. It is currently empty; all accepted implementation slices are recorded
in [`completed/`](completed/).

## 3. Decisions required

[`undecided/`](undecided/) contains proposals or features with unresolved choices.
Do not implement them yet. Resolve the listed decisions, update the document to
**Ready for implementation**, and move it into `to-be-implemented/` at the next
appropriate numbered position.

It is currently empty: every active feature specification has been resolved and
is in the numbered implementation queue.

## Working rule

Never change a completed contract incidentally. If a new feature needs one,
record the compatibility change in its ready specification and add a regression
test before implementation.

## Optional future work

[`optional/`](optional/) records deliberately deferred improvements that are
already scoped but are not part of the current implementation queue. It currently
contains the future GitHub Actions CI/CD quality gate; local testing and manual
Docker Hub publishing remain the active release flow.
