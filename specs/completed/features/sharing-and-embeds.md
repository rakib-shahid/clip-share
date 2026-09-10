# Feature: Share pages and Discord embeds

## Status

Accepted for version one. Embed behavior remains subject to real-client
compatibility verification.

## Goal

Every shareable clip has a stable page with a pleasant video player and metadata
that Discord can unfurl into a playable or useful video preview.

## Proposed behavior

- A clip receives an opaque, immutable public identifier. Folder and title
  changes do not alter its URL.
- Public pages use `/c/<random-id>`. Media assets use separate opaque endpoints;
  physical storage names and paths are never exposed.
- Every ready clip page, processed video, and required poster asset is publicly
  reachable without a Clip Share session so Discord can fetch and embed it.
- Public access is by direct link. There is no anonymous directory, gallery,
  search, user listing, folder listing, or clip-enumeration API.
- The HTML response is server-visible to Discord's crawler and includes canonical
  Open Graph title, video URL, media MIME type, dimensions, and poster image.
- Metadata uses probed output properties rather than claiming every file is MP4.
- Editing the clip's display title changes the title returned on subsequent share
  page requests.
- Media serving supports byte-range requests so browsers and Discord can seek or
  progressively fetch compatible video.
- The UI offers an obvious copy-link action after upload and in the explorer.
- An anonymous visitor sees a minimal page with the clip title, video player, and
  a Home action leading to the login page. They see
  no edit, move, delete, upload, directory-navigation, or administration controls.
- The anonymous page has no explicit download action and discloses no uploader,
  owner, folder path, upload time, or internal media metadata. Ordinary native
  video controls remain available; public media cannot be made non-downloadable.
- The player uses the generated poster and native controls, does not autoplay or
  loop, starts unmuted, uses `preload="metadata"`, and plays inline on mobile.
- Home sends a logged-out visitor to login, a logged-in normal user to their
  library, and the super-admin to the top-level explorer.
- Public clip pages discourage search indexing with both a robots meta directive
  (`noindex, nofollow`) and an `X-Robots-Tag: noindex, nofollow` HTTP header.
- Public clip HTML, video, and poster responses send `Cache-Control: no-store` so
  browsers/shared proxies should not retain them. This cannot revoke bytes already
  downloaded or force Discord to discard its own cached preview.

## Access model

The accepted model is **link-public clips with a private library**:

- Anyone who has a clip URL can view that individual clip without logging in.
- Discord can fetch the HTML, processed video, and poster without credentials.
- Authentication is required to discover or browse users, folders, and clip
  collections, including a user's own library.
- Anonymous requests to library, user-management, upload, edit, move, and delete
  endpoints are rejected by the Go server, not merely hidden by the frontend.
- Clip IDs are generated with enough cryptographic randomness to make guessing
  another valid link impractical. Sequential database IDs are never public IDs.
- Clip titles and media are intentionally public to anyone with the URL. Owners
  should not upload media they expect the URL itself to protect.

This is sometimes called “unlisted” because clips do not appear in a public
index, but no secret capability or logged-in identity is required after someone
possesses the URL. Deleting a clip invalidates its public page and media URL.

Search-engine exclusion headers/metadata may reduce accidental indexing, but
they are not access control and cannot guarantee a clip remains undiscovered.

Missing, trashed, processing, failed, cancelled, and otherwise unavailable public
IDs all return the same generic 404 response so state/existence is not disclosed.
Restoring a trashed clip reactivates the same URL.

All titles and metadata are context-escaped. User text never becomes executable
HTML, script, headers, or an unsafe URL. Authenticated pages and API responses
prevent storage by shared caches.

Discord may cache an embed after a URL is posted. A title edit updates Clip Share
immediately, but the service cannot guarantee Discord refreshes an already-cached
message preview. Stable URLs take priority over cache-busting URL changes.

## Completed acceptance criteria

- [x] A ready clip has one stable, copyable share URL.
- [x] Moving or retitling it does not change that URL.
- [x] A share response contains accurate Open Graph video/poster metadata.
- [x] The media endpoint supports required HTTP range behavior.
- [x] A title edit appears in later share-page responses.
- [x] An anonymous visitor with the URL can view the clip and its accurate embed
      metadata without receiving directory data or management controls.
- [x] The anonymous page offers only clip playback and a Home action to login.
- [x] It has no explicit download button or owner/folder disclosure.
- [x] Anonymous directory/API requests receive an authorization failure.
- [x] Public identifiers are random and non-sequential.
- [x] Public clip page responses contain `noindex, nofollow` in both HTML robots
      metadata and the `X-Robots-Tag` response header.
- [x] Public page/media responses send `Cache-Control: no-store`.
- [x] Deleting a clip makes its old page and media unavailable.
- [x] Every unavailable public state returns the same generic 404 response.
- [x] Player defaults match the accepted no-autoplay/no-loop, unmuted,
      metadata-preload, inline-mobile behavior.
- [x] Home resolves according to anonymous, normal-user, and super-admin context.
- [x] User metadata is safely escaped and authenticated responses are not stored
      by shared caches.
- [x] Authorization behavior is tested without relying on hidden UI.

## Compatibility verification

- Test Discord desktop on Windows, Discord web, and Discord mobile on iOS and
  Android when those devices are available.
- Verify generated videos near representative configured size limits rather than
  assuming container and codec declarations guarantee Discord behavior.
