# Main spec 90: Optional GitHub Actions CI/CD quality gate

## Status

**Optional; do not implement now.** Local testing and manual Docker Hub image
publishing remain the required release workflow.

## Purpose

Add a future GitHub Actions workflow that runs the existing local quality suite
automatically for pull requests and/or pushes, and optionally builds/publishes
Docker images only after those checks pass.

## Scope when selected

- Run Go formatting, vet, unit/integration tests, and race tests where supported.
- Run web lint, production build, Vitest component tests, and Playwright E2E with
  the accessibility scan in a dedicated test environment.
- Validate Compose configuration and build the production image.
- Keep Docker Hub credentials in GitHub secrets; never put them in repository
  files or workflow logs.
- Make publishing opt-in and tag-based; do not replace local/manual publishing
  until deliberately accepted.

## Prerequisites

- The local quality commands from
  `../../completed/0-core-platform/slices/0.18-engineering-quality-and-test-strategy.md` are stable
  and documented.
- A repository host and image-tag/release policy are chosen.
- The project owner explicitly authorizes GitHub repository secrets and Docker
  Hub credential integration.
