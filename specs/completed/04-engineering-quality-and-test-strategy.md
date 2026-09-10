# Engineering quality, simplification, and test strategy

## Status

**Completed 2026-09-06 (test foundation).** This behavior-preserving engineering
initiative. It must not change permissions, public URLs, upload outcomes,
retention, or user-visible workflows except where an accepted feature explicitly
does so.

**Confirmed approach:** use idiomatic Go (small structs/functions and narrow
interfaces at genuine I/O seams), retain colocated Go unit tests with top-level
integration/E2E/fixture tests, and refactor incrementally behind regression tests.
Do not add inheritance-style classes, a DI framework, ORM, Redux, or a generic
repository layer.

**Confirmed test environment:** Vitest and React Testing Library run in the
normal web Node environment. Playwright and its accessibility scan run in a
separate local Compose test image; no test runner is included in the production
image. No hosted CI workflow is in scope: the documented suite is run locally
before building and publishing a Docker Hub image.

## Goals

- Reduce duplication and make each feature easier to locate, understand, test,
  and safely change.
- Keep Go idiomatic: small packages, structs, functions, explicit errors, and
  narrow interfaces at real I/O boundaries.
- Keep React components focused on one screen or reusable interaction, with one
  source of truth for repeated modal/form/API behavior.
- Establish a layered automated suite that catches regressions before Docker Hub
  releases.
- Repair existing storage-layout compatibility gaps before further refactoring.

## Non-goals

- Rewriting working code merely to make it look object-oriented.
- Adding a dependency-injection framework, ORM, Redux, state-machine library,
  generic repository layer, or class hierarchy.
- Moving every Go test to a top-level directory at the cost of testing unexported
  code and package-local invariants.
- Combining feature work with broad unrelated formatting or behavior changes.

## Accepted language and organization principles

Go has structs and interfaces, not classes/inheritance. Its equivalent of useful
polymorphism is an interface that names a small capability needed at a boundary,
then multiple concrete implementations used in production/tests. Keep interfaces
consumer-owned and introduce them only where substitution is proven useful (for
example FFprobe, disk-space inspection, clock, or a media publisher).

- `cmd/server` owns process wiring only.
- `internal/httpapi` owns HTTP routes, authentication middleware, request decode,
  response mapping, and no business/media orchestration beyond invoking a use case.
- `internal/store` owns SQLite queries, transactions, and persistence-only errors.
- `internal/media` owns filesystem/FFmpeg/ffprobe operations and opaque asset
  resolution; every caller uses the same layout-aware asset resolver.
- Introduce a small `internal/app` package only when an operation coordinates two
  or more boundaries and cannot remain readable in a handler (upload intake,
  folder copy, asset-layout reconciliation, or password change are likely first
  candidates). Do not create it preemptively for simple CRUD.
- `internal/library` remains pure validation/naming logic with no HTTP, SQL, or
  filesystem dependency.
- Export only cross-package contracts. Prefer a concrete type until a second
  implementation or test seam is needed.

## Required simplification work

### Backend

1. Split `httpapi/api.go` into route registration, authentication/session helpers,
   JSON/error response helpers, and security/logging middleware. Retain one
   `authorizeMutation` path; remove handlers that manually reimplement cookie and
   CSRF validation.
2. Split upload intake into parsing/validation, reservation creation, streamed
   storage, source probing, and queue commit. Keep HTTP response mapping in the
   handler and make cleanup ownership explicit in one operation function.
3. Replace repeated anonymous `map[string]any` API envelopes with named response
   structs where they cross a public API boundary. Keep exceptional variable-shape
   fields explicitly typed too.
4. Centralize media asset lookup/copy/delete/publish behind layout-aware helpers.
   No media function may construct `media/<storage-id>` directly. Folder copy,
   public serving, trash preview/purge, processor publication, and reconciliation
   must use the same asset reference.
5. Keep transactions short. Do not perform long FFmpeg/copy/file operations while
   holding SQLite's single connection unless the operation specifically requires a
   documented transaction/rollback protocol.
6. Split oversized store tests by behavior (folders, clips, trash/restore,
   users, search) while retaining shared fixture helpers.

### Frontend

1. Make `components/ui/modal.tsx` the sole dialog/backdrop/focus implementation.
   Migrate the handcrafted recycle-bin, purge, create-user, and upload overlays
   out of `App.tsx`/feature components before implementing backdrop dismissal.
2. Split `App.tsx` into app bootstrap/session state, authenticated shell/header,
   library chooser, recycle-bin dialog, create-user dialog, and account menu.
3. Split `library-explorer.tsx` by data loading/navigation, folder cards,
   clip cards, and mutation dialogs. Keep the explorer as composition, not a
   second API/state-management layer.
4. Move repeated labelled-field, inline-error, mutation-busy, confirmation, and
   API-request patterns into small shared primitives. Do not make a generic form
   framework.
5. Centralize route construction and request/response types in `api.ts`; avoid
   untyped endpoint strings scattered through components where a typed helper can
   express the operation.
6. Every async effect/request must define stale-response/unmount handling when a
   user can close the component or change its target before completion.

## Test organization

Keep Go unit tests colocated with the package they test:

```text
internal/media/layout.go
internal/media/layout_test.go
internal/store/trash.go
internal/store/trash_test.go
```

This is Go convention and permits tests of package-private helpers without making
production APIs public. A top-level `tests/` directory cannot access unexported
identifiers from `internal/media`, `internal/store`, and similar packages; moving
everything there would reduce coverage or force poor exports.

Use a hybrid layout instead:

```text
internal/<package>/*_test.go       package-level unit/contract tests
internal/testutil/                 reusable test-only fixtures and HTTP helpers
tests/integration/                 black-box HTTP/database/media workflows
tests/e2e/                         Playwright browser journeys
tests/fixtures/media/              tiny committed/generated media fixtures
```

`internal/testutil` must not be imported by production code. Integration and E2E
tests verify observable behavior; colocated tests verify local edge cases.

## Required test layers and quality gates

| Layer | Required coverage |
| --- | --- |
| Go unit | Validation, name/path safety, store error mapping, media layout/copy/delete, processor argument construction, session/password logic. |
| Go HTTP integration | Route authorization, CSRF, error contracts, upload/cancel/failure cleanup, public links, trash, moves, copies, password change. |
| Media integration | FFprobe/FFmpeg fixtures and the readable-layout lifecycle, including folder copy after migration. |
| React unit/component | Modal backdrop/focus behavior, forms, error/loading states, API failure handling, navigation state. |
| Browser E2E | Login/setup, upload, folder/clip management, recycle bin, admin context, keyboard navigation, backdrop dismissal, password change. |
| Release/Compose | Production image build, Compose config, health/readiness, readable-layout compatibility, representative public embed playback. |

Required pull/release checks:

```text
gofmt -w (changed Go files only)
go vet ./...
go test ./...
go test -race ./...                  # where the local host supports it
npm.cmd --prefix web run lint
npm.cmd --prefix web run build
component tests
Playwright E2E + accessibility scan
docker compose ... config
production-image build
```

Add Vitest + React Testing Library for component tests and Playwright with an
automated accessibility scan for browser journeys. No coverage percentage gate is
used initially; every changed branch must have a direct test, and baseline package
coverage is recorded once the host Go coverage tool is repaired. A future minimum
may be set only after that baseline is trustworthy. Run this suite locally before
each Docker Hub image publish; hosted CI is intentionally out of scope.

**Confirmed coverage policy:** do not set an initial coverage-percentage target;
require a direct regression test for every changed behavior or branch instead.

## Initial implementation order

1. Add media-layout tests and repair folder-copy use of flat paths.
2. Add frontend test tooling and modal component tests; migrate every handcrafted
   overlay to `Modal`, then implement accepted backdrop dismissal.
3. Implement accepted self-service password change with HTTP/component/E2E tests.
4. Extract shared HTTP middleware/typed response helpers and split upload flow.
5. Split large React composition files and store test files only after their
   existing behavior is characterized by tests.
6. Add local release quality gates and record the test baseline.

## Acceptance criteria

- [ ] Every finalized-media operation uses the layout-aware resolver; folder copy
      works against the readable layout and has direct regression tests.
- [ ] No feature component implements its own dialog backdrop/focus behavior.
- [ ] Route handlers consistently use shared authentication, CSRF, decoding, and
      error helpers without changing API contracts.
- [ ] Backend use-case orchestration is readable without a framework and store
      methods contain no HTTP concerns.
- [ ] Go unit tests remain colocated; only cross-package integration/E2E fixtures
      live beneath `tests/`.
- [ ] React component tests and browser E2E tests cover every primary mutation and
      the accessibility/modal contracts.
- [ ] All documented quality gates pass in the supported local Docker environment
      before image publishing.
