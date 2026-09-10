# Code-quality audit — September 6, 2026

## Scope and baseline

Reviewed all Go production/test packages under `cmd/` and `internal/`, all React
components and client helpers under `web/src/`, package tooling, specifications,
and production Compose/Docker configuration.

- 46 Go test functions exist across focused package tests.
- `go test ./...` passes.
- `npm.cmd --prefix web run lint` passes.
- The production frontend build passes outside the filesystem sandbox.
- `go test ./... -cover` could not produce a coverage baseline on this workstation:
  Go 1.27 reports missing `covdata`/coverage-tool packages despite ordinary tests
  passing. Treat this as a local toolchain defect; reproduce in Docker/CI before
  setting a coverage threshold.
- No React test runner, component tests, browser E2E suite, or accessibility test
  dependency currently exists.

## Backend audit

| Area | Strengths | Findings / next action |
| --- | --- | --- |
| `cmd/server` | Clear wiring and shutdown ownership. | Keep it wiring-only; no need for classes or a new framework. |
| `internal/auth` | Small, focused password/session/validation split; contract tests exist. | Add self-service password-change tests once implemented. |
| `internal/config` | Small explicit environment parsing. | No direct tests; add table-driven valid/invalid environment tests. |
| `internal/httpapi/api.go` | Standard `ServeMux`, clear security middleware, explicit errors. | At 418 lines it mixes route registration, auth, cookies, JSON, errors, logging, and security headers. Extract focused helpers and eliminate manual CSRF checks duplicated in individual handlers. |
| HTTP feature handlers | Authorization checks are generally explicit and feature-specific. | Upload intake (314 lines) combines multipart parsing, field validation, capacity reservation, filesystem writes, probing, and queueing. Split operation stages while preserving cleanup semantics. |
| `internal/store` | Explicit SQL, transaction use, typed domain errors, strong ownership checks. | Large store/test files mix unrelated behavior. Split by capability; keep media I/O outside ordinary database transactions unless an explicit short transactional protocol is required. |
| `internal/media` | FFprobe/FFmpeg seams are testable; processor has useful failure paths. | The new readable layout is not consistently used: `media/copy.go` still directly reads/writes `media/<storage-id>`. Folder copy is therefore at risk after the completed production migration. Repair before further feature work and add layout/copy tests. |
| `internal/retention`, `library`, `webui` | Each is concise and appropriately narrow. | Add a small retention behavior test and keep these packages simple. |

## Frontend audit

| Component/area | Strengths | Findings / next action |
| --- | --- | --- |
| `api.ts` | Central fetch/XHR transport and shared domain types. | Error shape and route strings remain partially ad hoc; add typed operation helpers as new mutations are added. |
| `App.tsx` | Clear bootstrap/auth shell and no external state framework. | At 236 lines it also owns recycle bin, permanent purge, and create-user dialog implementations. Extract these to components. |
| `library-explorer.tsx` | Owns the primary library flow and reuses cards/pickers. | At 253 lines it is a composition and mutation-state hotspot; split data/navigation from card/dialog rendering after tests characterize behavior. |
| `Modal` | Portal, focus trap, Escape, focus restoration, and nested z-index are centralized. | Backdrop click dismissal is absent; several dialogs bypass this component entirely, so accepted modal behavior cannot be implemented consistently yet. |
| Recycle bin, purge, create-user, upload | Feature behavior is present. | They hand-roll full-screen overlays in `App.tsx`/`upload-dialog.tsx`, duplicating dialog/focus/backdrop responsibilities and creating accessibility drift. Migrate to shared `Modal`. |
| Folder picker/search/clip preview/user manager | Good focused component boundaries and reuse. | Add cancellation/stale-response handling for async fetches and component tests for unmount, failure, and keyboard paths. |
| Styling/UI primitives | Compact Tailwind/shadcn-style primitives and consistent visual language. | No automated visual/accessibility verification. |

## Test-layout conclusion

Do **not** move all Go tests into one `tests/` directory. It is technically
possible, but tests outside a package cannot access its unexported functions and
would either reduce coverage or force production APIs to become public. Keep unit
tests beside source; introduce `tests/integration`, `tests/e2e`, and shared
fixtures/test utilities for cross-package work.

## Highest-priority remediation

1. Fix and test folder copy against readable media paths.
2. Consolidate every overlay on the shared `Modal`; then implement backdrop
   dismissal once, with nested/destructive tests.
3. Add React component/E2E/accessibility tooling before expanding UI features.
4. Implement self-service password change with HTTP and component coverage.
5. Refactor only characterized hotspots: API helpers, upload orchestration,
   `App.tsx`, library explorer, and oversized store tests.
