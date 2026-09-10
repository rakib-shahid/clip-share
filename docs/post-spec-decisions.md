# Post-spec decisions and UI nitpicks

This record captures refinements agreed after the initial specifications:

- Use sky blue rather than violet for the accent theme.
- Keep the interface dark-only and grid-oriented.
- Ready clips need inline playback, copy-link, preview, rename, move, and delete actions.
- Clip moves use the same breadcrumb-based folder browser as folder moves. Numeric
  database folder IDs remain an internal API detail and are never requested from users.
- The complete folder-card body above its management bar opens the folder; blank
  card space must not become a dead, non-clickable area.
- Main-gallery and recycle-bin clip thumbnails use the same hover/focus play
  control and in-app mini player.
- Folder pickers, video previews, and Rename/Move/Delete card bars are shared UI
  components so their appearance and behavior cannot drift independently.
- Clip moves use the same breadcrumb-based folder browser as folder moves. Numeric
  database folder IDs remain an internal API detail and are never requested from users.
- Management dialogs must be in-app styled overlays with consistent backdrop blur;
  browser-native prompts are not acceptable.
- Folder navigation should update readable browser paths and respond to Back/Forward.
- Upload opened from a folder must default to that exact folder.
- Recycle Bin should use a compact grid and support preview, restore, and permanent delete.
- Trashed clip previews use authenticated recycle-bin media routes so inspection
  does not accidentally reactivate the public Discord/share URL.
- Permanent deletion is restricted to the super-admin and requires a checkbox
  plus confirmation. A startup-and-hourly worker enforces the 90-day retention.
- Keep implementation simple for a maximum of roughly five users.
- User administration uses styled in-app dialogs for edits and confirmations.
  Archived accounts leave the main library chooser but remain in the admin-only
  manager with an Open library action; restoration requires a replacement password.
- Search lives in the authenticated app shell rather than only inside one folder.
  It uses compact path-aware result cards and reuses the existing clip preview.
- Folder Copy is an administrator-only fourth folder-card action and uses the same
  folder picker as Move. Media is staged privately, source changes abort safely,
  and copies become visible only after the entire subtree succeeds.
- When Copy is present, the folder-card actions use a width-constrained grid so
  Delete cannot overflow the card at narrow widths.
- The folder-card Copy action is icon-only in a narrow fixed column. Rename, Move,
  and Delete divide the remaining width so their labels retain comfortable padding.
- Folder deletion loads a fresh recursive summary inside its in-app modal. It
  counts the selected folder plus descendants, clips, and stored bytes, and keeps
  acknowledgement and confirmation disabled until that summary succeeds.
- Cancelling during upload uses a styled confirmation and aborts the open request;
  cancelling a queued or processing card uses the durable job endpoint. Neither
  path creates trash, and repeated durable cancellation retries media cleanup.
- Failed upload cards retain only their private error notice and expose a confirmed
  Dismiss action. Dismissal deletes the failed job/clip records, not a trash item.
- The one-hour limit is one absolute wall-clock deadline beginning with the upload
  reservation. Queuing and restarts do not grant a new hour; normal Docker shutdown
  remains restartable unless that original deadline has actually expired.
- A folder-count implementation briefly caused an infinite loading state by running
  a nested query while SQLite's sole connection was held by open rows. The fix is
  to close result sets before count queries; this is now covered by a timeout-based
  regression test.
- Folder deletion must update nested clips in the same SQLite transaction. Merely
  hiding folder rows left nested public clip links reachable, so subtree trash and
  restore are now regression-tested as one unit.
- The production flat media layout was migrated successfully to the readable
  user/folder/clip hierarchy on September 6, 2026. The one-time migration command
  was then removed; future media moves reconcile the readable layout directly.
