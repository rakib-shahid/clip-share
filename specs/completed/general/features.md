# Version-one feature scope

## Status

Accepted on 2026-09-03. The linked feature specifications are authoritative for
detailed behavior and acceptance criteria.

## Product summary

Clip Share is a self-hosted, dark-themed, multi-user video library. Authenticated
users organize clips in private folder trees. Every ready clip has an opaque
link-public page suitable for Discord embedding, while anonymous visitors cannot
discover or browse libraries.

## Accepted features

| Feature | Priority | Specification |
| --- | --- | --- |
| One protected super-admin and normal users | Must | `../features/identity-and-access.md` |
| Admin-created credentials and signed browser sessions | Must | `../features/identity-and-access.md` |
| User lifecycle, restrictions, and archived libraries | Must | `../features/identity-and-access.md` |
| Private nested folder trees and full admin explorer | Must | `../features/library-and-folders.md` |
| Unique folder names and clip titles per parent | Must | `../features/library-and-folders.md` |
| Move, independent admin Copy, search, and grid paging | Must | `../features/library-and-folders.md` |
| Confirmed deletion and 30/90-day trash | Must | `../features/library-and-folders.md` |
| One-at-a-time upload with transfer/processing progress | Must | `../features/uploads-and-processing.md` |
| 500 MB/30-minute/8K/240 FPS source validation | Must | `../features/uploads-and-processing.md` |
| Per-user stored size and mandatory/optional compression | Must | `../features/uploads-and-processing.md` |
| H.264/AAC fast-start MP4 up to 1080p60 | Must | `../features/uploads-and-processing.md` |
| Persistent restart-safe processing jobs | Must | `../features/uploads-and-processing.md` |
| Stable public clip pages and Discord metadata | Must | `../features/sharing-and-embeds.md` |
| Private discovery with public opaque direct links | Must | `../features/sharing-and-embeds.md` |
| Dark charcoal, sky-blue, responsive grid UI | Must | `../features/user-interface.md` |
| Docker development and Komodo-ready deployment | Must | `docker.md` |
| Health, structured logs, cold backup/restore | Must | `operations.md` |

## Successor work

| Feature | Status | Specification |
| --- | --- | --- |
| Readable media-layout compatibility repair | Completed 2026-09-06 | `../01-readable-media-layout-compatibility.md` |
| Pre-finalization trim editor, multiple audio tracks, and per-track levels | Completed 2026-09-06 | `../05-video-editing-and-audio.md` |
| Backdrop dismissal for shared dialogs | Completed 2026-09-06 | `../02-shared-dialog-backdrop-dismissal.md` |
| Self-service password change | Completed 2026-09-06 | `../03-self-service-password-change.md` |
| Code simplification and rigorous test strategy | Completed 2026-09-06 | `../04-engineering-quality-and-test-strategy.md` |
| Administrator user-storage summary | Completed 2026-09-06 | `../04.5-user-storage-summary.md` |

## Explicitly deferred

- Multiple super-admins or configurable roles/permissions.
- Emails, invitations, self-service password reset, or OAuth.
- Total storage quotas.
- Batch/resumable uploads.
- Custom thumbnails or replacement of a ready clip's underlying media.
- Tags, favorites, playlists, collections, aliases, or shortcuts.
- Bulk move/delete operations and customizable sorting.
- Public galleries, user discovery, comments, reactions, or view counts.
- Light theme, list view, and drag-to-move.
- A public download button.
- PostgreSQL, Redis, a message broker, object storage, or an in-app backup system.
- A database audit subsystem or audit-log UI.

## Specification index

- `../features/identity-and-access.md`
- `../features/library-and-folders.md`
- `../features/uploads-and-processing.md`
- `../features/sharing-and-embeds.md`
- `../features/user-interface.md`

## Change control

New or changed behavior returns to Proposed status in the owning feature spec,
records its acceptance criteria and operational/security impact, and must be
accepted before implementation.
