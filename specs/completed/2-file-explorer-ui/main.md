# Main spec 2: File explorer UI

## Status

**Completed September 13, 2026.** All 34 slices are implemented. The final
acceptance pass covers authenticated normal-user and administrator explorer
paths, every server sort over keyset pagination, request races, grid/list and
responsive behavior, both item-menu paths, shadcn Dialog/Alert Dialog/Drawer and
Attachment flows, editor-title focus, reduced motion, and deployment health.

## Outcome

Clip Share's private file explorer becomes a polished, responsive grid/list
experience built primarily from official Radix-backed shadcn components. It
retains the charcoal and sky-blue identity, adds restrained Motion animations,
provides correct server-wide sorting, and replaces custom focus/menu/dialog
behavior with maintained primitives wherever practical. Current ownership,
public-link, upload, processing, trash, and folder semantics remain unchanged.

## Goals

- Replace avoidable custom interaction code throughout the application with
  official Radix-backed shadcn components.
- Provide responsive grid and compact-list explorer views with user-specific
  local preferences.
- Sort the complete folder result correctly, including items not yet loaded.
- Give every item equivalent visible Dropdown Menu and right-click/keyboard
  Context Menu actions.
- Improve loading, empty, feedback, navigation, and responsive dialog states.
- Let a file dropped anywhere over the active explorer enter the existing upload
  flow with the current folder selected.
- Add attractive, restrained motion without delaying interaction or violating
  reduced-motion preferences.
- Fix the editor-title dialog focus regression as part of the shared Dialog
  migration.

## Non-goals

- Multi-select, range selection, select-all, and bulk actions. These require the
  separate discovery in [main spec 91](../../optional/91-explorer-multi-select/main.md).
- Drag-and-drop organization, folder-tree sidebar, recursive folder sizes,
  favorites, tags, thumbnails for folders, or editing already-ready media.
- Changing authorization, storage layout, public IDs/URLs, processing, retention,
  upload limits, or destructive-operation meaning.
- Replacing React, Tailwind, Lucide, or Radix; adopting Base UI or React Aria as
  the application primitive provider.
- Animating for decoration alone, parallax, looping effects, or motion that
  blocks input.

## Component-source policy

shadcn is source distribution, not a hosted runtime widget library. Copied source
still lives in this repository, but interaction design and accessibility should
remain upstream-shaped.

- Keep the existing Radix-backed registry choice.
- Use shadcn CLI **4.21.0** and record it in the registry manifest; never use an
  unrecorded moving `latest` result in committed work.
- Pin new direct dependency versions and commit the lockfile.
- Record each copied registry component and CLI version in the first slice.
- Preserve upstream composition APIs. Put Clip Share colors, radii, typography,
  and semantic action variants in shared theme tokens or the narrowest primitive
  variant.
- Do not fork focus, keyboard, dismissal, positioning, or selection behavior
  unless this spec requires behavior the upstream primitive cannot express.
- Review future shadcn refreshes deliberately; never overwrite local changes
  automatically.
- Delete superseded custom primitives and helpers only after every consumer and
  regression test has migrated.

## Accepted component map

| Need | Accepted foundation |
| --- | --- |
| General modal content and forms | shadcn Dialog |
| Consequential/destructive confirmation | shadcn Alert Dialog |
| Multi-step narrow-screen picker | Dialog on desktop, Drawer below 640 px |
| Breadcrumb hierarchy and overflow | Breadcrumb with ellipsis Dropdown Menu |
| Header and item action menus | Dropdown Menu |
| Right-click, Menu key, Shift+F10 actions | Context Menu |
| Grid/list switch | Toggle Group |
| Related creation actions | Button Group |
| Persistent folder and clip representations | Item and ItemGroup, composed for grid/list |
| Selected upload and upload lifecycle | Attachment |
| State labels | Badge |
| Empty surfaces | Empty |
| Initial placeholders | Skeleton |
| Indeterminate operations | Spinner |
| Measured transfer/processing | Progress |
| Exact timestamp and icon explanation | Tooltip |
| Completed-action feedback | Sonner |
| Form labeling/validation | Field, Label, Input, Checkbox as applicable |
| Scrollable bounded content | Scroll Area where native overflow is insufficient |

`Card` remains valid for page-level or grouped surfaces, but it is not a reason
to recreate Item, Attachment, Empty, or dialog composition. `Attachment` is used
for selected/transient uploaded files; persistent library clips use `Item`
because it is the semantically general media/title/description/action primitive.
Drawer is the official shadcn Radix-style component backed by Vaul; Attachment
is an official provider-neutral composition. Neither changes the application's
Radix choice for focus/menu/dialog primitives.

## Explorer information architecture

The page order is:

1. administrator acting-context banner when applicable;
2. collapsed Breadcrumb path;
3. folder title and authoritative total folder/clip summary;
4. responsive toolbar;
5. inline page-level error when applicable;
6. folder-first item collection;
7. explicit Load more action when a cursor exists.

The toolbar contains a Button Group for **New folder** and **Upload clip**, an
icon Toggle Group for Grid/List, and a labelled Dropdown Menu for sorting. It
wraps on narrow screens and is not sticky in this release.

## Views and item content

Grid is the first-use default. List is a compact alternative.

The grid has one column below 640 px, two from 640–1023 px, three from
1024–1279 px, and four at 1280 px and above within the existing `max-w-7xl`
content width. Items in a row use equal-height shells without stretching poster
aspect ratios.

### Grid

- Folder: folder icon, name, direct child-folder count, and direct clip count.
- Ready clip: 16:9 poster, title, stored size, relative upload date, and status.
- Queued/validating/processing/failed clip: state artwork, textual state,
  measured progress when available, and safe error summary.
- The overflow action trigger is always discoverable on touch and keyboard focus;
  hover-capable pointers may use a subtler resting treatment but never hide the
  only way to reach actions.

### Compact list

- Desktop columns: media/name, type or processing state, size/counts, uploaded
  date, and action menu.
- Folder rows show child folder/clip counts rather than a fabricated size/date.
- Below 640 px, each row collapses to media, name, one secondary status line,
  and actions. It never introduces horizontal page scrolling.
- Grid and list expose the same actions, labels, states, and ordering.

One click/tap opens a folder or a ready-clip preview. Controls inside an item do
not activate the item. Processing/failed clips have no preview activation.
Double-click and long-press are not required interactions.

The collection uses ordinary document/list semantics (`ul`/`li`), not an ARIA
`grid` or roving-tabindex composite. Tab reaches each item's primary action and
then its overflow trigger in DOM order. Arrow keys are reserved for an open
menu; they do not move between closed collection items.

## Action model

The visible three-dot Dropdown Menu and Context Menu consume one typed action
model so labels, order, eligibility, icons, and handlers cannot drift.

| Item/state | Actions in order |
| --- | --- |
| Folder, normal user | Open, Rename, Move, separator, Move to recycle bin |
| Folder, administrator | Open, Rename, Move, Copy, separator, Move to recycle bin |
| Ready clip | Preview, Copy link, Open public page, separator, Rename, Move, separator, Move to recycle bin |
| Queued/validating/processing clip | Cancel upload |
| Failed clip | Dismiss failed upload |

Open public page uses a new tab with safe `noopener`/`noreferrer` behavior. Copy
link uses the established absolute public URL. Destructive entries use the
destructive menu treatment but never execute until their accepted Alert Dialog
confirmation completes.

Right-click, the keyboard Menu key, and Shift+F10 open the same Context Menu.
Touch uses the visible Dropdown Menu trigger; no custom long-press recognizer is
added. Menu focus returns to its trigger/item after close. Disabled/inapplicable
actions are omitted rather than shown as mysterious disabled commands unless a
busy operation temporarily disables an otherwise valid command.

## Dialog and destructive behavior

- Create and rename stay in Dialogs. Enter submits a valid idle form; Escape
  cancels when idle; focus returns to the invoking control.
- Input is trimmed at submission while internal spaces and server validation are
  preserved. No rename request occurs per keystroke.
- Unsaved rename/create text is local only and may be discarded on close without
  a second confirmation.
- Folder picker is a Dialog at 640 px and above and a bottom Drawer below it.
- Ordinary move-to-trash of one ready clip uses Alert Dialog without a checkbox.
- Recursive folder deletion, permanent purge, and library deletion retain their
  authoritative summary and acknowledgement checkbox.
- Cancelling processing or discarding an upload states irreversible consequences
  but needs no checkbox.
- A busy destructive surface cannot close via Escape, backdrop, trigger, or
  close button.
- Nested editor warnings remain modal, topmost, and the only interactive layer.

### Editor rename focus regression

The current custom `Modal` reruns its focus-initialization effect whenever its
`onClose` callback identity changes. Editor dialogs pass inline callbacks, so
typing updates state, creates new callbacks, and moves focus to the first button.
Heartbeat is not causal: it sends a request every 60 seconds without setting
React state. Title metadata is also not saved per keystroke.

The migrated editor details Dialog must retain focus and selection through every
keystroke, parent rerender, heartbeat request, edit-save status change, failed
request, and nested-dialog lifecycle. Initial autofocus runs once per open. Save
Title/Enter sends one metadata request; Escape/Cancel sends none.

## Sorting contract

Sorting is server-backed and applies to the complete folder, not only loaded
items. Folders always form the first group; clips form the second.

| Sort choice | Folder order | Clip order |
| --- | --- | --- |
| Latest upload (default) | Name A–Z | `created_at DESC`, then ID descending |
| Oldest upload | Name A–Z | `created_at ASC`, then ID ascending |
| Name A–Z | Normalized name then ID ascending | Normalized title then ID ascending |
| Name Z–A | Normalized name then ID descending | Normalized title then ID descending |
| Largest size | Name A–Z | Known size descending, unknown size last, then newest/ID |
| Smallest size | Name A–Z | Known size ascending, unknown size last, then newest/ID |
| Processing state | Name A–Z | Processing, Validating, Queued, Failed, Ready; then newest/ID |

“Uploaded” is the existing clip `createdAt`, including queued or failed records.
Finishing processing does not change sort position. Folder size is not recursive
and is never invented. Human labels may say Latest/Oldest; API values are stable
documented enums.

`GET /api/folders/{id}` accepts the sort enum. An omitted sort means latest
upload. Pagination uses a versioned **keyset cursor**, never an offset. The
cursor binds folder ID, sort, folder/clip phase, the last emitted canonical sort
tuple, and stable ID. Using a cursor with a different folder/sort, invalid phase,
malformed tuple, or unsupported version returns `400 invalid_cursor`. Changing
sort clears loaded pages and starts at the first page.

Each response contains at most 60 combined items and preserves the folder-first
boundary. Inserts/deletes before a cursor do not shift later pages. Sort-key
changes from another tab may move an item across the cursor; pagination is an
eventually consistent live view rather than a database snapshot. Known local
mutations reset to page one. The client deduplicates kind+ID defensively and
offers Refresh; it must never claim snapshot consistency.

FolderPage adds `totalFolderCount`, `totalClipCount`, and `totalItemCount`.
Totals describe direct children visible in the explorer: non-deleted folders and
non-deleted clips except `uploading` and `cancelled`; failed private notices are
included because they render as items. Folder-card clip counts use the same clip
state rule. Empty arrays and existing fields remain unchanged.

## Preferences

- Store versioned JSON in `localStorage`, keyed by authenticated acting user ID.
- Explorer stores `{view: "grid" | "list", sort: <enum>}`. First-use defaults
  are grid and latest upload.
- One explorer preference applies across the user's own and administered
  libraries. Do not key it to the viewed owner.
- Search and recycle bin have separate view preferences. Search retains API
  relevance/path order; recycle bin retains most-recently-deleted order.
- Preferences contain no username, title, path, session material, or other
  sensitive data.
- Missing, denied, malformed, old-version, or unavailable storage falls back to
  defaults without blocking the explorer. Valid `storage` events update another
  same-origin tab; a changed sort starts a coordinated page-one request and a
  changed view relayouts current items. Invalid events are ignored.

## Dates

Use a shared formatter and semantic `<time dateTime="...">`. Display compact
relative buckets such as `5 min ago`, `Yesterday`, or a short date; a Tooltip
exposes the exact localized date and time. Do not tick seconds. Refresh relative
minute/hour buckets at most once per minute while the document is visible and
clean up the timer on unmount/visibility change. Invalid input renders a safe
fallback and never changes sorting, which remains server-authoritative.

## Loading, error, empty, and feedback states

- First load uses labelled Skeleton items matching the active view.
- Folder navigation keeps the old collection visible but inert with a subtle
  progress indicator. History/URL and the animated collection commit only after
  success; failure restores interactivity and leaves the original folder/URL.
- Sorting uses the same retain-then-commit rule and cannot append a cursor from
  the previous sort.
- Load more is explicit. It keeps existing items and focus stable, appends up to
  60 results, and exposes busy/error/retry next to the action.
- Active-job polling uses one authenticated batch-status request for the loaded
  active job IDs, chunked at 100 IDs. It never replaces loaded folder pages.
  Stable-key sorts merge statuses in place; completion under Size or Processing
  state resets to page one because the authoritative sort key changed.
- Empty explorer, empty search, and empty recycle bin use Empty with surface-
  appropriate text and actions.
- Measured progress uses Progress with numeric ARIA values. Indeterminate work
  uses Spinner/status text and never fabricates a percentage.
- Successful copy, rename, move, folder copy, restore, and similar completion
  use Sonner. Errors stay beside the failed control/dialog. Toasts are
  supplementary and are not the sole assistive-technology announcement.

## Motion contract

Use `motion` **13.1.1** through `motion/react`. shadcn/Radix/Vaul CSS owns
Dialog, Alert Dialog, Drawer, Dropdown Menu, Context Menu, Tooltip, and Toast
portal transitions. Motion does not wrap or replace those interaction
primitives; it owns explorer collection, layout, navigation, and item feedback.

- A root `MotionConfig` uses `reducedMotion="user"` and shared 160–240 ms
  transitions.
- Animate grid/list layout changes, server sort reordering, item addition/removal,
  folder-content replacement, and subtle hover/tap feedback.
- Initial folder entry may stagger only the first eight visible items by about
  15 ms each. Polling, sorting, Load more, and view switching never restagger.
- Forward folder navigation moves content slightly left; Back moves slightly
  right. The data and URL commit, focus placement, and interaction state remain
  authoritative—animation cannot delay them.
- Processing polls update state/progress without replaying entrance motion.
- Reduced-motion mode disables transform/layout movement and stagger, retaining
  only short opacity/color feedback.
- No parallax, large zoom, autoplay decoration, or indefinite animation beyond
  established progress indicators.

## Accessibility and responsive requirements

- Preserve WCAG AA contrast, visible focus, semantic headings, text status, and
  keyboard parity. Color and motion are never the only state signals.
- Breadcrumb uses `<nav aria-label="Breadcrumb">`, an ordered list, links for
  ancestors, and non-link current-page text. Long paths show root, ellipsis,
  parent, current; the ellipsis menu contains omitted ancestors in root-to-leaf
  order.
- Toggle and sort controls have persistent accessible names and selected state.
- Grid/list items expose one unambiguous primary action without nested invalid
  interactive markup. Menu triggers have item-specific labels.
- Context Menu is optional convenience; every command remains reachable through
  the visible trigger and keyboard.
- Dialog/Drawer titles and descriptions are programmatically associated. Focus
  is trapped only while open and returns to the correct invoker.
- At 320 CSS px there is no horizontal page overflow or clipped required action.
- Live regions announce navigation result, view/sort changes, failures, and
  completed mutations without announcing every animation frame or poll.
- Full component/E2E coverage runs in Chromium. Critical smoke paths for Dialog,
  Alert Dialog, Drawer, both menus, folder navigation, grid/list layout, and
  reduced motion also run in Playwright Firefox and WebKit. A touch-emulated
  Chromium project covers the visible-menu path without right-click.

## Compatibility, security, and performance

- No backend authorization or mutation semantics change. Read-only additions are
  folder sorting/keyset cursors/totals and authenticated batch job status.
- Do not expose private paths, extra cross-user metadata, source URLs, or session
  material through item props, preferences, toasts, or tooltips.
- Public clip links, previews, trash rules, folder-copy identity, processing
  cancellation, and stable browser routes retain main spec 0/1 behavior.
- Each response adds at most 60 items; explicit Load more may accumulate pages.
  Motion must remain responsive for 240 rendered items and must not cause a React
  state update per animation frame. Avoid eager video loading in item rows.
- Registry additions must justify themselves by replacing concrete custom code.
  Remove unused components/dependencies after migration.

## Overall acceptance criteria

- Every child slice passes its focused tests before the next dependent slice.
- Grid/list, all sort modes, pagination boundaries, navigation failure, and
  preferences work for normal users and administrator acting contexts.
- Dropdown and Context Menu action matrices match exactly.
- All application dialogs use the accepted shadcn foundations; the superseded
  custom `Modal` and manual account menu are removed.
- Continuous editor-title typing never loses focus and does not issue per-key
  metadata requests.
- Keyboard, touch, 320 px, reduced-motion, and automated accessibility checks
  pass.
- File drags across the explorer or an already-open idle upload Dialog show the
  viewport drop treatment and select the file in the committed current folder;
  non-file drags remain unchanged.
- Full Chromium and the defined Firefox/WebKit/touch smoke matrices pass.
- React lint/test/build, Go tests, browser tests, and Docker Compose runtime
  verification pass with no intentional contract drift.

## Slice index

The ordered child specifications in [`slices/`](slices/) are authoritative
implementation units.

### Shared shadcn and Motion foundation

1. [`2.1` registry and theme baseline](slices/2.1-shadcn-registry-and-theme-baseline.md)
2. [`2.2` Dialog primitive](slices/2.2-dialog-primitive.md)
3. [`2.3` editor Dialog focus regression](slices/2.3-editor-dialog-focus-regression.md)
4. [`2.4` application form Dialogs](slices/2.4-app-form-dialogs.md)
5. [`2.5` application content Dialogs](slices/2.5-content-dialogs.md)
6. [`2.6` Alert Dialog primitive](slices/2.6-alert-dialog-primitive.md)
7. [`2.7` explorer Alert Dialogs](slices/2.7-explorer-alert-dialogs.md)
8. [`2.8` application Alert Dialogs](slices/2.8-application-alert-dialogs.md)
9. [`2.9` responsive hook and Drawer](slices/2.9-responsive-dialog-hook-and-drawer.md)
10. [`2.10` responsive folder picker](slices/2.10-responsive-folder-picker.md)
11. [`2.11` account Dropdown Menu](slices/2.11-account-dropdown-menu.md)
12. [`2.12` status and feedback primitives](slices/2.12-status-and-feedback-primitives.md)
13. [`2.13` Motion foundation](slices/2.13-motion-foundation.md)

### Sorting and local presentation utilities

14. [`2.14` folder sort and cursor types](slices/2.14-folder-sort-and-cursor-types.md)
15. [`2.15` store-level folder sorting](slices/2.15-store-folder-sorting.md)
16. [`2.16` folder-sort HTTP contract](slices/2.16-folder-sort-http-contract.md)
17. [`2.17` preference storage](slices/2.17-explorer-preference-storage.md)
18. [`2.18` relative upload time](slices/2.18-relative-upload-time.md)

### Explorer composition and behavior

19. [`2.19` toolbar](slices/2.19-explorer-toolbar.md)
20. [`2.20` collapsed Breadcrumb](slices/2.20-collapsed-breadcrumbs.md)
21. [`2.21` typed item model](slices/2.21-explorer-item-model.md)
22. [`2.22` folder grid item](slices/2.22-folder-grid-item.md)
23. [`2.23` clip grid item](slices/2.23-clip-grid-item.md)
24. [`2.24` Dropdown and Context Menu actions](slices/2.24-item-dropdown-and-context-actions.md)
25. [`2.25` compact list](slices/2.25-compact-list-view.md)
26. [`2.26` batch job-status endpoint](slices/2.26-batch-job-status-endpoint.md)
27. [`2.27` request coordination](slices/2.27-navigation-request-coordination.md)
28. [`2.28` motion, states, and pagination](slices/2.28-collection-motion-and-pagination-states.md)

### Related surfaces and completion

29. [`2.29` upload Attachment](slices/2.29-upload-attachment-presentation.md)
30. [`2.30` Search convergence](slices/2.30-search-result-convergence.md)
31. [`2.31` Recycle-bin convergence](slices/2.31-recycle-bin-convergence.md)
32. [`2.32` cleanup and acceptance](slices/2.32-final-cleanup-and-acceptance.md)
33. [`2.33` viewport file-drop upload](slices/2.33-viewport-file-drop-upload.md)
34. [`2.34` minimum pending transitions](slices/2.34-minimum-pending-transitions.md)

## Reference inputs

- shadcn's current [component catalog](https://ui.shadcn.com/docs/components),
  [Radix Dialog](https://ui.shadcn.com/docs/components/radix/dialog), and
  [Radix Breadcrumb](https://ui.shadcn.com/docs/components/radix/breadcrumb).
- Motion's [React accessibility guidance](https://motion.dev/docs/react-accessibility)
  and [layout-animation contract](https://motion.dev/docs/react-layout-animations).
