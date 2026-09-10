# Data and storage contract

## Status

Accepted for version one.

## SQLite model

- `users`: normalized/display username, Argon2id hash, role, state, stored-file
  limit, timestamps.
- `folders`: owner, optional parent, normalized/display name, trash metadata.
- `clips`: owner, one parent, storage ID, random public ID, normalized/display
  title, probed media metadata, state, trash metadata.
- `jobs`: upload/clip, state, progress, claim time, safe error, timestamps.
- `storage_reservations`: upload/job, reserved bytes, timestamps.
- `schema_migrations`: applied migration number and time.

Internal relationships use SQLite integer IDs. Public IDs are 32 random bytes
encoded as unpadded URL-safe base64, uniquely indexed, and never derived from an
internal ID or metadata.

## Invariants

- Exactly one super-admin exists and every account has one protected library root.
- Every non-root folder has one same-owner parent; depth is at most 20; cycles fail.
- Every clip has exactly one same-owner parent folder.
- Active sibling folder names and clip titles are independently unique using their
  case-insensitive normalized forms.
- Username normalization is ASCII case folding. Folder/title comparison uses
  Unicode normalization plus case folding after outer whitespace trimming.
- Protected account roots cannot participate in ordinary rename/move/trash calls.
- Cross-owner folder moves recursively change ownership in one transaction.
- Copies receive new folder, clip, public, and storage IDs and become visible only
  after every media copy and database change succeeds.
- Public clip/media URLs resolve by immutable `public_id`, never by a filesystem
  path. A media-layout move, user/folder rename, or cross-library move therefore
  never changes a public URL or Discord embed target.
- Trashed records do not appear in active uniqueness, library, or public queries.
- Limit changes do not mutate existing clips. Concurrent conflicts use database
  constraints/transactions: first commit wins and later operations fail cleanly.
- A missing expected final media asset makes its clip unavailable but does not
  silently erase metadata. Startup cleanup removes only provably orphaned temporary
  directories, never unmatched final-media directories.

## Physical layout

```text
/data/
├── database/
│   └── clip-share.db
├── media/
│   └── <user-name>--<user-id>/
│       └── <folder-name>--<folder-id>/
│           └── <clip-title>--<storage-id>/
│               ├── video.mp4
│               └── poster.jpg
└── temporary/
    └── <upload-id>/
```

The physical hierarchy mirrors the current user/folder hierarchy in SQLite. Each
human-readable component is transformed to a Windows-safe filesystem component
and suffixed with its stable ID (or the clip's opaque storage ID). This prevents
collisions, reserved-name failures, and path traversal without making the layout
opaque. A clip directory retains a readable title even after storage ID lookup.

Renaming or moving a user, folder, or clip reconciles finalized media to its new
physical location. Final media becomes visible through atomic rename only after
verification. The resolver falls back to the previous known asset directory while
a reconciliation is in progress, so public IDs remain continuously resolvable.

There is no automatic filesystem migration at runtime. This installation's
existing flat layout has already been migrated; future uploads and metadata moves
use the readable hierarchy directly.

The one-time production media-layout migration was completed on September 6,
2026. Existing public URLs and Discord embeds were verified to remain stable.

## Database behavior

- Enable WAL mode, foreign keys on every connection, and a busy timeout.
- The store currently uses one SQLite connection. Query result sets must be fully
  consumed and closed before issuing another query; otherwise a nested count query
  can block forever. Folder-count regression tests use context timeouts to catch
  this class of connection starvation.
- Transactions wrap multi-record moves, copies, trash, restore, jobs, and capacity
  reservations.
- Embedded numbered migrations apply in order before readiness.
- Index ownership/parent listing, normalized uniqueness, public ID, trash age, and
  job state/order.

## Trash and backup

Trash preserves original parent information. Owners restore through day 30;
super-admin restores through day 90; purge after 90 removes media and rows
idempotently. The complete `/data` directory is the supported cold-backup unit.
