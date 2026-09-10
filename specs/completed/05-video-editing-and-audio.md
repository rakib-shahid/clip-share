# Feature: pre-finalization video editing and audio mixing

## Status

**Completed September 6, 2026.** This is the implemented upload-time editor, not
a change to an already-ready clip. The completed behavior in the revision and
implementation record below is authoritative. The decision register is retained
only as historical context and does not describe unfinished work.

## Confirmed editor UX revision — September 6, 2026

This revision is authoritative where older preview/timeline wording below
conflicts with it:

- Editing is optional. **Upload clip** follows the original validate-and-queue
  workflow without opening an editor. **Trim clip** opens the editing workflow.
- The Trim action opens an editor immediately and retains the selected browser
  `File` for private local playback while upload and analysis continue. When the
  original is browser-playable, playback and seeking use that local object URL.
  Otherwise the server automatically creates a private browser-compatible proxy.
- Local playback uses the browser-selected/default source audio and is intended
  for instant visual trimming. **Render preview** remains available on demand
  and renders both video and the exact configured multi-track audio mix. From
  the moment rendering is requested until polling reports completion or failure,
  an indeterminate progress bar, spinner, and **Rendering preview…** status are
  displayed beside the disabled action. A percentage must not be fabricated
  unless the backend later exposes measured encoder progress.
- **Finalize upload** proceeds directly only when a ready preview's recipe
  revision equals the current saved edit revision and no unsaved draft exists.
  Otherwise it opens a nested **Finalize without a current preview?** warning
  explaining that rendering is recommended to verify the audio and video. Its
  actions are **Back**, **Render preview** (close the warning and start a render),
  and an amber warning-icon **Finalize upload** (close the warning and explicitly
  finalize without a current preview).
- The timeline uses an evenly spaced private filmstrip and automatically zooms
  as the selected range becomes shorter so its handles remain easy to position.
  The wheel zooms in or out around the selection, while visible padding and
  pointer capture continue to allow either handle to be dragged outward. While
  an edge is being dragged, its starting zoom/scale is frozen, the opposite edge
  stays visually fixed, and nearing either side pans the viewport instead of
  changing zoom. Recenter and adaptive rezoom happen only after release. At more
  than 1x zoom, a horizontal timeline scrollbar is shown; Shift+wheel and a
  trackpad's horizontal gesture also pan without changing zoom. Its thumb is
  proportional to the visible portion of the clip and is styled as a scrollbar,
  not a playback/progress control. The scrollbar row always reserves the same
  height, including at 1x and during handle dragging, so controls below it never
  shift vertically. One selection rectangle replaces separate playhead/In/Out
  sliders: either slim edge handle resizes it, its middle moves the range without
  changing duration, and clicking
  the filmstrip seeks the playhead. Pointer Events provide click-and-hold
  dragging for mouse, touch, and pen. The last complete, overscanned frame strip
  remains visible throughout interaction. After a brief settled-state debounce,
  the browser captures a replacement set of evenly distributed frames for the
  visible window and swaps it atomically; it must never clear the old strip or
  enlarge a low-detail overview while new frames are decoding.
- Seeking and playback are constrained to the selected range. Reaching the Out
  point while playing immediately loops to In and continues until explicitly
  paused, including when Out is the physical end of the media. Pressing Space
  anywhere in the main editor toggles playback and takes precedence over a
  focused button; text-entry fields and open nested dialogs retain normal Space
  behavior. Changing the selection clamps the playhead to In when it would
  otherwise fall outside. This applies to both local-source and rendered-preview
  playback. Keyframe seeking affects preview responsiveness only; finalized
  trims remain accurately re-encoded.
- Precise fields display and accept `HH:MM:SS.mmm`, including shorter forms such
  as `1:23.500`. Timeline and arrow-key changes use 100 ms steps; Shift+Arrow uses
  one second. The existing 250 ms minimum duration remains.
- Browser-generated filmstrip frames are used for playable local sources; proxy
  frames are used for fallback sources. All proxy/editor artifacts remain private
  and follow the existing session cleanup rules.
- Closing while transfer or analysis is active always asks for confirmation.
  After analysis, the existing changed-recipe confirmation rule applies.
- Editor and upload actions use icons plus semantic button colors: green for
  upload/save/finalize, red for cancel/discard, amber for warning overrides,
  blue for trim/render, and neutral outlined styling for Back and secondary
  actions. **Set In**, **Set Out**, and **Reset trim** are visibly button-shaped.

## Hosted preview and lost-session revision — September 6, 2026

- Production Content Security Policy explicitly permits private browser `blob:`
  media. This is required for the selected local `File` fast-preview and its
  browser-generated timeline; it does not publish or upload that object URL.
- If the local source container/codec is genuinely unsupported, the editor
  automatically renders one full-source browser-compatible private MP4. That
  proxy becomes the player and filmstrip source for the remainder of the session.
- Saving trim or audio changes marks the last rendered preview stale by revision,
  but never hides or deletes it. It remains usable for playback, seeking, and
  filmstrip generation until the user explicitly requests another render.
- A replacement render writes a separate pending file. The previous ready preview
  stays visible throughout rendering and is atomically replaced only after the
  new file succeeds. A failed replacement leaves the previous preview intact.
- Users may have multiple independent editor sessions. Re-selecting **Trim clip**
  with the same normalized title, destination, and source byte size within five
  minutes of the last heartbeat resumes that session and its saved edits instead
  of transferring or reserving a duplicate. A different identity starts another
  editor.
- Open editors heartbeat once per minute and therefore remain alive indefinitely
  while open. A background pass runs once per minute and permanently removes each
  editing session, placeholder, reservation, source, and preview after five full
  minutes without a heartbeat. A session cannot be resumed after that cleanup.
  Container restart recovery preserves recent sessions and their last complete
  preview, clears interrupted preview-render locks/partial files, and then applies
  the same five-minute cutoff.

## Problem and outcome

People often need to remove the beginning/end of a recording, choose between
game/chat/microphone tracks, or lower one track before sharing. The finished
clip must still be one browser- and Discord-compatible MP4 with predictable
storage, limits, privacy, and public-link behavior.

The recommended first release is intentionally narrow:

- One uploaded source video becomes one ready clip.
- The editor supports one continuous trim range: remove before an in-point and
  after an out-point. It does not splice, join, crop, add text, or replace media
  after publication.
- It reads every supported source audio stream, lets the uploader include/mute
  each stream, and sets each included stream's level in the final mix.
- The published MP4 contains one normalized AAC stereo **mixdown**. This is the
  reliable choice for HTML video, Discord, byte-range playback, metadata, and
  size accounting. Source tracks are not exposed to anonymous viewers.

## Goals

- Let an authorized uploader inspect the source and make a trim/mix decision
  before final processing is finalized.
- Accept normal multi-audio-stream containers without silently discarding useful
  tracks before the uploader can choose.
- Preserve the existing one-click sharing contract: a ready clip has one video
  stream and one universally playable AAC stereo audio stream.
- Make preview behavior, final render behavior, and output-size limits match.
- Keep source files private, temporary, and cleanup-safe at every cancellation,
  expiration, restart, and failure boundary.

## Non-goals for this release

- Editing an already-ready clip, changing its public URL, or retaining edit
  history/undo after finalization.
- Multi-range cutting, clip joining, transitions, captions, cropping, rotation,
  speed changes, user-adjustable fades, EQ, compression/normalization, surround
  output, or adding a separate audio file. The documented final peak limiter is
  the sole automatic audio-protection step.
- Publishing alternate audio tracks, runtime audio-track switching on public
  pages, or a downloadable original.
- Frame-perfect professional NLE behavior. The final encoder is authoritative.

## Decision register

The Question/Proposed-default entries below preserve review history only. Every
one was resolved before implementation. The completed contract and later UX
clarifications are authoritative.

The following choices change user expectation, resource use, or the persisted
contract. The **recommended** column is the coherent small first release; it is
not an implementation authorization.

| Decision | Options | Recommended decision and reason |
| --- | --- | --- |
| Where editing occurs | Before processing; edit ready clips | Before processing only. It avoids changing a shared public asset and keeps one immutable ready output. |
| Edit shape | One continuous trim; multiple cuts | One continuous trim. It solves the stated need without a timeline/NLE data model. |
| Trim precision | Keyframe-fast; accurate re-encode | Accurate re-encode at the requested timestamps. Faster keyframe cuts visibly surprise users. |
| Editor preview | Browser source preview; server proxy/proxy renditions | Private server-rendered preview for every source. Browser decoding of original containers/track mixes is not authoritative. |
| Audio result | One selectable source track; retained alternate tracks; one mixed track | One AAC stereo mixdown. It is the most compatible output and enables per-track levels. |
| Default audio mix | First/default only; all tracks at unity | Default-marked audio track only (otherwise first usable track) at 0 dB; other tracks start muted. This prevents doubled game/chat audio. |
| Level control | Percentage; dB; normalize/duck | Per-track gain in dB only, −60 dB to +12 dB, plus Mute. Do not add automatic normalization or ducking in the first release. |
| Channel mapping | Preserve layout; automatic stereo downmix; user routing matrix | FFmpeg's deterministic automatic downmix to stereo. Show source channel layout, but no routing matrix. |
| Silent output | Reject; allow | Allow. A user may mute every track; the output has no audio stream. |
| More than one audio stream | Cap/reject; support all | Analyze all supported audio streams, but expose at most 8 editable tracks; reject a source with more than 8 usable audio streams with a clear message. |
| Unsupported/undecodable audio | Reject source; ignore track | Ignore individual unusable tracks when a valid video remains; tell the user which tracks cannot be used. Reject only if the source has no valid video. |
| Draft lifetime | Existing one-hour total; pause deadline; longer editor deadline | No retained draft. An active editor session has a five-minute lost-session cleanup deadline; transfer keeps its existing one-hour deadline and final processing gets its own one-hour deadline. |
| Persistence across refresh | Browser-only; server draft | Session-only server state supports autosave/preview and can be resumed for five minutes by choosing the same title and destination. It is permanently removed after lost-session cleanup. |
| Reopening the editor | Free before queue; lock after queue | Editable until Finalize is accepted; lock once queued. Discarding/cancelling cleans all temporary data and never reopens a draft. |
| Source retention | Keep original after ready; delete it | Delete all source, editor, preview, processing, and edit-recipe data after ready/failure/cancel/lost-session cleanup. Retain only the finalized clip record and its ready assets. |
| Output-size estimate | None; advisory; block before render | Advisory only. The server remains authoritative after final encode and uses the existing per-file limit policy. |
| Super-admin limits | Existing bypass; apply destination limit | Retain the existing super-admin bypass for final stored size; all global source/editor limits still apply. |

Every row in this register is confirmed. The review history below records the
reasoning and exact accepted outcome; it no longer represents open choices.

## Review findings and confirmed decisions

These decisions were confirmed on September 5, 2026. The alternatives remain as
decision history only. The confirmed outcome is authoritative wherever it differs
from an earlier recommendation.

### A. Published-audio contract

**Question A1 — what does “multiple audio tracks” mean for the ready clip?**

- **Proposed default:** accept and expose up to eight input tracks in the editor,
  then publish exactly one AAC stereo mixdown. This is the only option that makes
  level adjustments reliable on Discord and ordinary browser players.
- Alternative: retain multiple selectable AAC tracks in the MP4. This preserves
  alternatives but public browser/Discord track selection is inconsistent, and
  per-track levels cannot be applied interactively after publication.
- Alternative: publish both a mixed track and source alternatives. This enlarges
  files and substantially complicates size policy and public-player behavior.

**Question A2 — should mixing protect against clipping?**

- **Proposed default:** apply a transparent final peak limiter at −1 dBFS after
  the user-selected gains. The UI says that it protects against distortion but
  can change peaks; per-track gain remains the user's primary control.
- Alternative: no limiter. This is a mathematically pure mix but two loud tracks
  can produce audible digital clipping.
- Alternative: automatically normalize the whole mix. This is not recommended:
  it makes the selected track levels less predictable and changes loudness.

**Question A3 — what output audio format is wanted?**

- **Proposed default:** one AAC-LC stereo stream at 128 kbps when any track is
  included; no audio stream when all are muted. Surround sources downmix using
  FFmpeg's documented coefficients.
- Alternative: adaptive mono/stereo bitrate. This saves little space but adds
  rules and makes output less uniform.

### B. Preview contract

HTML video does not provide a portable way to independently level and mix all
audio streams from an arbitrary uploaded container. A local browser preview may
show only one track or a codec the browser cannot decode, so it cannot promise
that the reviewed mix equals the final render.

**Question B1 — how exact must preview be?**

- **Proposed default:** provide **Render preview**, a private, temporary 720p
  H.264/AAC MP4 produced with the same trim/mix filter graph. It is exact for
  timing and audio behavior, though lower resolution/bitrate than the final.
  Each request supersedes the prior preview for that draft revision.
- Alternative: local source preview only, with a warning that track mixing and
  unsupported codecs cannot be previewed. This is cheaper but violates
  what-you-hear-is-what-you-publish for the central new feature.
- Alternative: create a full-quality final render for every change. This is too
  slow and wasteful for an editor.

**Question B2 — what preview resource limits apply?**

- **Proposed default:** one preview render at a time per draft, preview output
  capped at 720p/2 Mbps and 60 seconds. For a longer selected range, **Render
  preview** renders the 60-second window centered on the playhead and clamped to
  the selection, so users can inspect either trim boundary or any middle section.
  Private previews expire on replacement, cancellation, draft expiry, or
  finalization. The UI names the exact range being previewed.
- Alternative: permit full-selection previews. This is simpler to explain but
  can consume nearly as much time/storage as the final render for 30-minute clips.

**Question B3 — how do preview renders interact with final jobs?**

- **Confirmed:** a preview is a bounded child job of an `editing_session`,
  never a public clip/job state. It uses the existing worker infrastructure and
  remains cancellable/replacement-safe. A Finalize action waits for the current
  preview to finish or cancels it before enqueuing the final job; final renders
  take precedence over preview work when both are waiting.
- Alternative: make previews ordinary FIFO processing jobs. This needs less new
  scheduling logic but a nonessential preview can delay a user's final upload.

**Question B4 — how should unplayable source video be handled before preview?**

- **Proposed default:** the editor always uses the private render-preview route;
  it does not depend on the browser being able to decode the original container.
- Alternative: block editing of non-browser-playable sources. This weakens the
  accepted input compatibility and is not recommended.

### C. Editing and render semantics

**Question C1 — when does a change require re-encoding?**

- **Proposed default:** no edit keeps the current remux-or-normalize behavior.
  Any trim re-encodes video for accurate boundaries. Audio-only edits remux a
  compatible video stream and re-encode only the generated AAC mix. The UI says
  that trimming may take longer and may use the selected compression settings.
- Alternative: keyframe-only trim with video stream copy. It is quicker but the
  saved boundaries can be visibly earlier/later than the handles.

**Question C2 — are tiny edge fades desired?**

- **Proposed default:** apply no hidden fade. The cut occurs exactly at the
  selected boundary; a later release may offer an explicit 5–100 ms audio fade.
- Alternative: always apply a 10 ms audio fade to prevent clicks. This slightly
  changes the literal requested content and must be disclosed.

**Question C3 — how do streams of unequal length behave?**

- **Proposed default:** the selected video trim is authoritative. Every chosen
  audio stream is trimmed to that interval, begins at output time zero, and the
  mixed audio ends with the video; a shorter source track contributes silence
  after it ends. Initial source offsets/delays are preserved relative to video.
- Alternative: end the clip at the shortest chosen stream. This unexpectedly
  shortens video and is not recommended.

**Question C4 — what happens to nonessential source media data?**

- **Proposed default:** do not preserve subtitles, chapters, attachments,
  timecode, alternate video streams, or arbitrary container metadata in the
  ready output. Preserve only the explicit display title/public metadata and
  required orientation after pixels are normalized.
- Alternative: preserve selected metadata/subtitles. Each adds UI, security,
  compatibility, and render rules outside this feature's scope.

### D. Draft lifecycle, limits, and retries

**Decision D1 — no retained editor draft**

- **Confirmed:** there is no 24-hour retained draft. An active editor session
  sends a heartbeat; after five minutes without it, the server removes all
  temporary data. Transfer retains its existing one-hour deadline and final
  processing receives a new one-hour deadline after Finalize.
- Alternative: retain the current one-hour absolute deadline over upload, editing,
  queueing, and render. This reuses existing behavior but gives little practical
  editing time.
- Alternative: no/longer expiry. This risks abandoned raw media consuming the
  small self-hosted server indefinitely.

**Decision D2 — all failures clean up**

- **Confirmed:** every intake or final-render failure removes all source, preview,
  recipe, reservation, job, and placeholder data. Retrying always requires a new
  upload.
- Alternative: every failure deletes the source and requires a new upload. This
  is simpler but makes a correctable trim/mix mistake unnecessarily expensive.

**Question D3 — what can the user do after pressing Finalize?**

- **Proposed default:** Finalize locks a recipe snapshot. While queued/rendering,
  the user may only cancel; cancellation cleans the draft and does not restore it.
  To revise, they cancel and upload again. This prevents races between a render
  and mutable source settings.
- Alternative: cancel returns to the editable draft. This is friendlier but
  requires precise cancellation checkpoints and careful partial-output cleanup.

**Question D4 — how many retained drafts/previews are allowed?**

- **Superseded default:** the original plan allowed one active draft per user.
  The completed hosted-session revision permits multiple independently leased
  sessions, while title uniqueness, storage reservations, source limits, and
  five-minute inactivity cleanup bound their storage use.
- Alternative: several drafts. This needs an explicit count, reservation policy,
  and a draft-management screen.

### E. Limits, UI behavior, and persistence

**Question E1 — how does trimming interact with compression and stored-file
limits?**

- **Proposed default:** the existing source-size rule remains at intake. The
  editor displays an estimate based on trimmed duration but never promises it.
  If an edit needs encoding, existing mandatory/optional compression rules and
  the 110% acceptance threshold apply to the final file. A no-edit compatible
  source retains current remux behavior.
- Alternative: force compression whenever the editor opens. This is predictable
  technically but needlessly degrades unedited compatible uploads.

**Question E2 — should users see visual timelines beyond trim handles?**

- **Proposed default:** time ruler, current frame/time readout, and selected
  range only. No thumbnails or waveform in the first release.
- Alternative: filmstrip thumbnails and/or waveform. These require separate
  background extraction, storage/reservation, accessibility alternatives, and
  cache cleanup; they should be a distinct feature.

**Question E3 — how are changes saved and conflicts resolved?**

- **Proposed default:** autosave a debounced whole recipe after every settled
  trim/gain/toggle change, visibly show Saved/Saving/Retry, and use revision
  compare-and-swap. On a conflict, retain the local recipe and offer Reload or
  Overwrite after presenting the remote change summary.
- Alternative: explicit Save button. This is simpler but creates more loss risk
  and can leave Finalize disabled unexpectedly.

**Decision E4 — remove editing data after success**

- **Confirmed:** after success delete all editor/session/recipe/provenance,
  source, preview, and temporary editor/preview-job records and assets. Retain
  the ordinary ready final-render job plus the ready clip record and its finalized
  MP4/poster assets.
- Alternative: delete all edit data after ready. This minimizes metadata but
  makes render/debug investigations harder.

**Question E5 — may title and destination change while editing?**

- **Proposed default:** yes, until Finalize. The workspace uses the existing
  title/name rules and folder picker; a server transaction releases/replaces the
  draft title reservation only after validating the newly selected destination.
  Finalization snapshots title and destination with the edit recipe.
- Alternative: lock both values after transfer. This is simpler but forces a
  discard/re-upload for a common correction discovered while editing.

### F. Scope boundaries

**Question F1 — is editing a ready/public clip explicitly out of scope?**

- **Proposed default:** yes. The new workflow is only between intake and ready;
  ready clips remain immutable except for title/folder management.
- Alternative: replace/rerender an existing clip. This needs versioning, public
  cache/embed semantics, recovery, and a new permission/trash design.

**Question F2 — should more than eight usable audio streams be rejected?**

- **Proposed default:** yes; report `too_many_audio_tracks` and require a source
  with eight or fewer usable audio streams. This bounds UI, filter complexity,
  preview work, and test coverage.
- Alternative: expose more tracks. State the exact cap and whether the UI pages
  them; “unlimited” is not suitable for this installation's operating model.

### Resolved wording conflict

The earlier draft used “Cancel returns to the private draft” in one place and
“Cancel removes the draft” in another. The accepted rule is now unambiguous:
there is no retained private draft. **Discard upload** deletes the active editor
session and all temporary data; after Finalize, **Cancel render** does the same
and never reopens editing.

## Completed implementation contract

The following records the completed implementation contract. The later UX
revision at the top of this file overrides older timeline/preview wording here.

- One source video produces one immutable ready clip. Ready clips are not edited.
  The editor supports one accurate, continuous trim range, without hidden fades.
- Probe and expose at most eight usable input audio streams. The ready MP4 has
  exactly one AAC-LC stereo mixdown at 128 kbps when audio is included, or no
  audio stream when every track is muted. It never publishes alternate tracks.
- The default mix includes only the default source stream (or first usable stream)
  at 0 dB; all other tracks begin muted. Included tracks have gains from −60 to
  +12 dB in 0.5-dB steps and are mixed with a transparent −1 dBFS final peak
  limiter. Do not normalize, duck, add EQ, or apply a hidden edge fade.
- Video duration is authoritative. Audio preserves its relative source timing;
  a selected track that ends early contributes silence until video ends. Do not
  append black video frames, and reject trim bounds outside the selected video.
- Any trim accurately re-encodes video. Audio-only edits remux compatible video
  and encode only the AAC mix. No edit retains the existing remux-or-normalize
  processing behavior and every final output obeys the existing size policy.
- Preview is always a private server-rendered H.264/AAC 720p, 2 Mbps asset using
  the final trim/mix graph. It covers the full selection up to 60 seconds; for a
  longer selection it covers a 60-second window centered on the playhead. One
  preview child job may be active per editor session, and a final render takes
  priority over preview work. The application runs one FFmpeg render at a time;
  a queued final render is claimed before queued previews, but a running preview
  is never preempted.
- Admission control reserves an additional fixed 100 MB for the one active
  private preview, on top of the source and final-output reservation.
- The editor is a short-lived resumable session, not a retained draft. An open
  editor heartbeats at least once per minute. After a tab is lost, selecting the
  same title and destination reattaches to its saved recipe for up to five minutes.
  Different clips may be edited concurrently; title uniqueness prevents two
  sessions from reserving the same title in one folder. Five minutes without a
  heartbeat triggers idempotent cleanup. Explicit discard still removes the
  session immediately after the changed-recipe confirmation when applicable.
- Finalize snapshots and locks that session's title, destination, and recipe;
  only confirmed cancellation remains afterward. Cancellation, every failure,
  restart recovery, and lost-session cleanup delete its temporary source,
  preview, recipe, reservation, temporary job, and placeholder data.
- Title and destination remain editable before Finalize, with existing ownership,
  folder-picker, uniqueness, and reservation rules. Autosave is session-only,
  debounced, revision-protected, and visibly reports Saving/Saved/Retry.
- The final commit deletes all editing provenance, stream descriptors, preview
  assets, source media, and temporary editor/preview-job records. It retains the
  ordinary final-render job under the existing ready-job lifecycle, plus ready
  clip metadata, the final MP4, and poster. Strip subtitles, chapters,
  attachments, timecode, alternate video streams, and arbitrary source metadata;
  preserve correct normalized orientation.

## User flow and states

The active editor is an authenticated modal opened from the upload flow and
backed by its opaque editor session ID. It is not a public or bookmarkable route.
The session ID is authorized only for its owner/acting admin and is never a public
media identifier.

```text
select source → uploading → analyzing → editing session → finalizing → queued
      → validating/rendering → ready
                         ↘ failed / cancelled / expired
```

1. The existing upload dialog collects source, title, destination, and the
   existing compression choices. It transfers exactly one source and reserves
   temporary capacity as it does today.
2. Server-side FFprobe validates the source and records duration, selected video
   stream, and every usable audio stream. The browser cannot establish these
   facts by MIME type or `HTMLMediaElement` metadata alone.
3. After analysis the dialog becomes the full-screen-or-large-modal **Edit clip**
   workspace. It is a five-minute resumable active session; explicit discard removes
   the temporary upload rather than leaving an upload card to resume later.
4. The user chooses the in/out range and audio mix, previews it, then presses
   **Finalize upload**. This presents the final recipe and changes the durable
   state to queued atomically. The app then returns to the selected destination
   folder, where the existing queued/processing clip state and cancellation
   controls represent the normal final-render job.
5. Server-side render applies the stored recipe, enforces existing validation,
   resolution/frame-rate/compression/size rules, generates the poster, and
   publishes only when the ready assets commit together.
6. **Discard upload** during editing removes temporary source, recipe,
   reservation, and placeholder clip. If edits differ from the initial recipe,
   the editor asks for confirmation first. Closing/reloading a browser with edits
   uses the browser's leave-page confirmation where available; the five-minute
   lost-session cleanup remains the safety net. Failure and queued-render
   cancellation always clean up all temporary data.

### State invariants

- An editor session is authenticated-only and is never returned by public routes, search,
  folder counts intended for ready clips, or share metadata.
- `editing_session` reserves its title and temporary capacity but has no public
  URL that resolves publicly. Its hidden placeholder owns a stable public ID,
  but public routing rejects it until the clip becomes ready. It receives a client
  heartbeat at least once per minute and expires five minutes after its most
  recent heartbeat.
- A finalizing request compares an editor-session revision number. A stale tab receives
  `409 edit_conflict` and reloads the authoritative recipe instead of silently
  overwriting another tab.
- A queued/rendering job uses an immutable recipe snapshot. Mutating the session
  after finalization is not permitted.
- A preview is a separately tracked private child job while its parent remains
  `editing_session`; it never changes clip visibility, title reservation, or public
  readiness state.
- Cancellation, expiry, job failure, and restart recovery are idempotent. No
  source, preview proxy, partial MP4, or reservation may be orphaned.

## UI specification

### Layout

Use the existing dark modal system. On desktop, present a two-column workspace:
preview/player above the timeline on the left, and a scrollable Inspector on the
right. On a narrow screen, stack Player, timeline, then Inspector; the sticky
footer remains reachable without covering controls.

```text
Edit clip                                      Close
Title [________________]  Destination [path]

┌──────────── player / current frame ─────────────┐ ┌─ Inspector ─────────────┐
│                                                   │ │ Duration / source info  │
└───────────────────────────────────────────────────┘ │ Audio tracks            │
0:00  [trim handle ===== selected range ===== handle] │ [x] Game      −6.0 dB  │
      In  [00:00.000]  Out [00:42.500]                │ Mute Voice   0.0 dB   │
                                                        └────────────────────────┘
Close editor                  Render preview      Finalize upload
```

### Video and trim controls

- Display clip title, duration, dimensions, frame rate, source size, and clear
  `Source remains private until finalization` helper text.
- The player is muted by default only if browser autoplay policy requires it;
  otherwise play respects the current audio-mix preview. It never autoplay-starts
  with sound.
- Timeline has a single selected region, slim draggable in/out handles, a
  playhead, generated filmstrip, adaptive zoom, fixed-height horizontal
  scrollbar row, and visible current-time readout. Clicking the filmstrip seeks.
- In and Out fields accept `HH:MM:SS.mmm` (with shorter forms accepted) and have
  Set In / Set Out buttons at the playhead. Keyboard arrow adjustments must be
  available; recommended increments are 100 ms and 1 s with a modifier key.
- Enforce `0 ≤ in < out ≤ duration` and a minimum final duration of 0.25 seconds
  unless the selected output/container policy requires a different technical
  floor. Errors are inline and Finalize stays disabled.
- The visible trim range is always the recipe that will render. Accurate final
  encoding remains authoritative even though live browser seeking is keyframe
  optimized.
- Reset Trim restores `in=0` and `out=source duration`, asks for no confirmation,
  and does not reset audio settings. Closing asks for confirmation whenever the
  recipe differs from its initial state; confirmed cancellation discards the
  session rather than retaining a server draft.

### Audio-track inspector

For each usable source audio stream, show its ordinal, language tag when present,
codec, channel layout, sample rate, and disposition labels such as Default or
Commentary. Never infer a human-friendly track name that FFprobe did not provide.

- Each row has an Include/Mute toggle, draggable level slider, and live numeric
  dB readout. Muted rows keep their chosen gain but contribute no audio until
  included again.
- The initial default mix includes only the default stream or first usable stream
  at 0 dB. Other streams start muted at 0 dB. The user may include any number.
- The slider range is −60 to +12 dB, step 0.5 dB. −60 dB is functionally silent;
  the distinct Mute state has accessible text and is unambiguous to keyboard and
  screen-reader users.
- A master meter/level is intentionally absent. The rendered mix applies the
  accepted peak limiter without hidden normalization or fades.
- If every track is muted, show `This clip will be silent` and permit finalizing.
- Unusable tracks appear as disabled informational rows with the safe rejection
  reason. They have no Include control and do not make a valid video unuploadable.
- A track cannot be soloed in the persisted recipe. Temporary Preview solo is a
  possible later enhancement; adding it now would need a separate accessibility
  and state decision.

### Preview, review, feedback, and accessibility

- Live source playback is constrained to the trim selection and loops from Out
  to In until paused. **Render preview** creates a private preview using the same
  selection/mute/gain recipe as final output. The footer shows an indeterminate
  progress indicator while it renders. A full selected range is previewable when
  it fits the limit; otherwise the preview is a playhead-centered sample. Only
  final FFmpeg output is authoritative.
- The workspace shows a clear browser-preview limitation when the source codec
  cannot play locally. The selected fallback decision determines whether users
  may still finalize by numeric controls or wait for a server-created private
  proxy; it must never silently preview a different range/mix.
- The workspace displays trim duration, tracks and gains, compression choice,
  source/output facts, and destination. **Finalize upload** is disabled while
  analysis/save is pending or any field is invalid. If no current rendered
  preview matches the edit revision, it opens the confirmed three-action warning:
  **Back**, **Render preview**, or warning-styled **Finalize upload**.
- Transfer, analysis, draft saving, render progress, failure, and expiry use
  text status plus `role=status`/`role=alert`, not color alone. Level inputs have
  labels, all icon buttons have names, and the modal preserves the existing focus
  trap/Escape/return-focus conventions.
- **Discard upload** asks for confirmation only when edits differ from the initial
  recipe. It must distinguish `Keep editing` and `Discard upload`; a queued
  render instead exposes the existing confirmed cancellation action and cannot
  return to editing.

## Media and render contract

### Source analysis

- Continue enforcing the global source ceilings and video validation in
  `uploads-and-processing.md`: 500 MB, 30 minutes, 8K, 240 FPS, unencrypted,
  decodable video.
- Probe all audio streams, including stream index, codec, channels/layout,
  sample rate, language/disposition metadata, and whether FFmpeg can decode it.
  Do not treat attachment/cover-art streams as audio.
- Supported input audio codecs are whatever the installed FFmpeg can safely
  decode. The actual supported set is deployment-tested; unsupported individual
  streams are excluded with a reason rather than guessed conversion.
- A source whose selected video stream is undecodable fails analysis and is
  cleaned up. If a requested preview/final render later encounters a decode
  failure, cancel and clean up the session with a stable safe render error code.
- A transient preview-render failure removes only its preview job and asset,
  leaves the editor open, and presents a safe retry action. It does not become a
  failed library clip or require re-uploading the source.

### Recipe validation

The server validates every submitted recipe regardless of the UI:

- `trim_start_ms` and `trim_end_ms` are integer milliseconds within the probed
  duration and meet the accepted minimum duration.
- Each `audio_stream_index` belongs to this source, appears at most once, is
  usable, and is within the editable-track cap.
- `included` is boolean; `gain_db` is finite and within −60.0 through +12.0 in
  0.5-dB increments. Track order is canonicalized by source stream index.
- An omitted audio configuration means the safe default mix, not “all tracks.”
- All source identity, title/destination authorization, capacity, timeout, and
  compression rules are rechecked at finalization.

### FFmpeg behavior

- Trim before encode using accurate timestamp seeking and filter trim, then reset
  timestamps so output begins at zero. The job must not use `-c copy` for a trim
  that promises accurate boundaries.
- Map exactly the chosen video stream. When selected audio tracks exist, map
  those streams to an audio-filter graph, apply `volume=<gain>dB` per included
  stream, then `amix=normalize=0` them into a single stream followed by a
  transparent `-1 dBFS` peak limiter. The filter graph must use explicit source
  stream indexes, not default stream selection, and must not apply normalization,
  ducking, EQ, or hidden fades.
- Normalize the mixed result to AAC-LC stereo at the existing accepted bitrate;
  use FFmpeg's standard mono/stereo/multichannel-to-stereo downmix policy, and
  preserve silence by omitting audio entirely when no tracks are included.
- Final video duration is authoritative. Pad shorter selected audio with silence
  and cut longer selected audio to that duration; do not use `-shortest`.
- Final MP4 remains H.264 High Profile, 8-bit `yuv420p`, fast-start, at most
  1080p60, with one video stream and zero or one AAC stereo audio stream. Poster
  selection uses the final trimmed duration (10%, capped at five seconds).
- Size targeting uses the **trimmed duration** and only the output audio allowance
  (zero for silent output). It does not reserve one audio bitrate per input track.
- Record exact FFmpeg/FFprobe versions and a safe render failure category in
  structured logs; never log source paths, signed URLs, session material, or raw
  user media metadata beyond existing safe identifiers.
- The FFmpeg capability shipped in the production Docker image is the supported
  baseline. Commit compact media fixtures for that image's supported containers,
  codecs, orientations, stream counts, silence, and multichannel cases.

## Backend/API contract

All routes are authenticated, CSRF-protected state changes unless noted. Existing
upload endpoints may be evolved or versioned, but no public route serves an active
editor session or preview.

| Method and route | Purpose | Success response / important failures |
| --- | --- | --- |
| `POST /api/uploads` | Transfer source and create analyzing editor session, replacing the private immediate-queue upload flow | `201` session ID/state; existing size/capacity/title errors |
| `GET /api/uploads/{id}` | Poll active editor-session analysis/edit/preview state | session metadata, recipe revision, preview state/revision, stream descriptors; `404` outside authorization or after cleanup |
| `PUT /api/uploads/{id}/edit` | Save a whole trim/mix recipe with `If-Match` revision | updated recipe/revision; `409 edit_conflict`, `422 invalid_edit` |
| `POST /api/uploads/{id}/heartbeat` | Keep the active editor session alive | `204`; `404` after session cleanup |
| `POST /api/uploads/{id}/preview` | Create/replaces the private preview child job for a bounded window | `202` preview state; `409` while finalizing |
| `GET /api/uploads/{id}/preview` | Read the authorized ready private preview | range bytes with `Cache-Control: no-store`; generic `404` otherwise |
| `POST /api/uploads/{id}/finalize` | Lock current recipe and enqueue render | `202` queued job; `409` if stale/not editable; `422` invalid recipe |
| `DELETE /api/uploads/{id}` | Confirmed editor-session/render cancellation | `204`; repeat-safe |

`GET /api/uploads/{id}` exposes only source facts necessary for editing, never
absolute paths. Example edit representation:

```json
{
  "state": "editing_session",
  "edit_revision": 7,
  "duration_ms": 185432,
  "video_stream_index": 0,
  "audio_streams": [{"index": 1, "codec": "aac", "channels": 2,
    "language": "eng", "default": true, "usable": true}],
  "edit": {"trim_start_ms": 1250, "trim_end_ms": 42500,
    "audio": [{"stream_index": 1, "included": true, "gain_db": -4.0}]}
}
```

Use stable API error codes: `source_preview_unsupported`, `too_many_audio_tracks`,
`audio_track_unusable`, `invalid_trim`, `invalid_audio_gain`, `edit_conflict`,
`session_expired`, and `session_not_editable`. Human messages may change; codes do
not. The browser must not receive FFmpeg command lines or internal paths.

Finalize is idempotent for the same locked recipe revision and returns the
already-created final job; a stale/different revision returns `409 edit_conflict`.
Cancellation is repeat-safe and returns `204` even if cleanup already completed.
Preview status exposes its recipe revision and state separately; the preview media
endpoint serves media bytes only.

## Data, migrations, and cleanup

Use new numbered migrations; never alter an applied migration.

Source transfer has its existing one-hour deadline. Finalize records a separate
final-render deadline one hour from finalization; it never derives that deadline
from the source upload/session creation time. Once Finalize atomically locks and
queues the normal final-render job, the five-minute editor heartbeat lease no
longer applies; the job deadline exclusively governs final processing.

Implementation requirements for the accepted session lifecycle:

- Persist an `editing_session` only long enough to authorize requests, protect
  title/capacity reservations, coordinate preview/final jobs, and recover safely.
  It includes `last_heartbeat_at`, a revision, and an immutable job snapshot at
  Finalize. It is not a resumable user draft.
- Upload intake creates a hidden, non-public placeholder clip row that owns the
  stable storage/public IDs and requested title/destination. It is omitted from
  library and public lookups until Finalize promotes it into the ordinary queued
  final-render flow; cleanup deletes it completely.
- A startup sweep and a recurring cleanup sweep remove any non-final session whose
  heartbeat is older than five minutes. The same cleanup removes its source,
  preview, recipe/audio rows, reservation, temporary job row, and placeholder
  clip row.
- The ready-commit transaction deletes all editing-session, stream-descriptor,
  recipe, preview, and temporary editor/preview-job rows after atomically
  publishing the final MP4/poster and ready clip record. It retains the ordinary
  final-render job according to the established ready-job lifecycle and never
  retains edit provenance.
- Preview assets are included in the capacity reservation and are removed before
  final processing begins. A preview request while another preview is rendering
  returns `409 preview_in_progress` and does not preempt it. After it completes,
  fails, or is queued for replacement, the next preview replaces the prior asset
  before publishing the new private preview.
- Finalize during an active preview locks the recipe immediately, queues the final
  render behind that preview, blocks further edit/preview requests, and removes
  the preview asset as soon as the preview completes before starting final render.

- Extend clip/job state with `analyzing`, `editing_session`, and `finalizing` only
  if distinct rows are required. Keep ready/public state semantics unchanged.
- Add an `upload_edits` row keyed by clip/upload ID: source fingerprint or storage
  identity, `revision`, trim milliseconds, canonical JSON/audio rows, editor
  timestamps, and deadline. Store individual editable audio choices in normalized
  `upload_edit_audio` rows; Finalize copies the complete canonical recipe as an
  immutable JSON snapshot onto the final-render job.
- Add probed stream metadata for the active session only and delete it after the
  final commit or every cleanup path. Do not retain original source stream labels
  or edit provenance with the ready clip.
- Use a schema check/transaction to prevent more than one current editor session
  per acting user and to atomically advance revision/finalize. The job snapshot
  cannot depend on mutable editor-session rows.
- The reservation must cover source plus final output and private preview assets.
- Startup recovery removes all non-final editor sessions rather than resuming
  them. It removes source/preview/partial assets, releases capacity, and leaves
  no failed notice or resumable placeholder.

## Security, privacy, and operations

- Ownership checks apply to every draft, recipe, stream descriptor, preview,
  finalization, and cancellation request. Super-admin authority follows the
  existing destination-library rules.
- Source and any proxy remain under server-owned opaque paths outside public media
  routing. Reject path-like, non-finite, huge, and duplicate recipe values.
- Do not trust browser duration, selected stream IDs, or level values. Re-probe
  after restart or when source integrity identity changes.
- Editing is a heavier CPU path because accurate trimming requires encoding even
  when source codecs would otherwise remux. Expose queue position/state and safe
  FFmpeg errors. The application runs one FFmpeg process at a time; a queued final
  render is chosen before queued previews, without preempting a running preview.
- Backups may contain active editor sessions if `/data` is copied. Restore/startup
  deterministically cleans them; it never resumes or publishes an incomplete asset.
- The Compose image must include a tested FFmpeg build with the decoders and AAC
  encoder relied upon by the documented compatibility matrix.

## Regression and acceptance expectations

- Unit-test trim bounds, duration floor, gain parsing/quantization, duplicate or
  foreign stream IDs, default-mix construction, canonical recipe snapshots, and
  revision conflicts.
- FFprobe fixture tests cover a silent source, one track, multiple default/non-
  default tracks, language/disposition metadata, surround input, unusable audio,
  and nine usable tracks.
- Processor integration tests assert accurate output bounds, zero-based output
  timestamps, one H.264 video stream, zero/one AAC stereo stream, expected mix
  audibility/relative gain, trimmed poster position, fast-start, size accounting,
  and no ready output on render failure.
- HTTP tests cover authorization, CSRF, generic missing-draft behavior, stale-tab
  conflict, finalization lock, cancellation/expiry/restart cleanup, and proof that
  no draft/preview is public.
- Browser coverage includes keyboard trim entry, handle interaction, screen-reader
  labels/status, narrow layout, disabled finalization, track mute/gain/reset,
  all-muted warning, discard/leave-page confirmation, lost-session cleanup, and
  selection-loop playback and preview-window boundaries.
- Manual compatibility testing verifies representative AAC, Opus, multiple-track,
  and silent sources in the supported browsers and Discord clients, including
  playback, seeking, poster/embed behavior, and that the final mix matches the
  reviewed recipe.

## Completed implementation slices

1. **Completed — schema/state/API:** short-lived resumable editor-session lifecycle, heartbeat cleanup,
   analysis descriptors, secure recipe persistence, cancellation/recovery tests.
2. **Completed — editor UI:** source analysis state, accessible trim controls, autosave/discard,
   preview, and review/finalize flow; no render behavior change until recipe
   validation exists.
3. **Completed — renderer:** accurate trim and single-track/default audio selection, then complete
   multi-track mixer/gain graph with media fixtures.
4. **Completed — operational/release pass:** private preview scheduling, migration/backup
   cleanup, Compose FFmpeg matrix, browser/Discord compatibility verification.

No editor session exposes a public ready clip until final rendering and atomic
publication succeed. Successful completion retains only the finished clip assets
and established ready-job metadata; temporary editor data is removed.
