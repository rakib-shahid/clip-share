# Clip Share

A self-hosted clip upload and sharing service.

Version-one product and architecture specifications are accepted. The first
vertical slices are implemented: database migrations, initial-admin setup,
authentication, admin-created users, an ownership-safe folder explorer, streamed
uploads, durable FFmpeg processing, and Docker-first development/production builds.

## Run locally

Docker Compose is the supported development environment. Copy `.env.example` to
`.env`, replace both secret placeholders, then start the API and Vite dev server:

```powershell
Copy-Item .env.example .env
docker compose up --build
```

Open <http://localhost:5173>. On the first visit, create the one super-admin with
the setup token from `.env`. Later starts go directly to login.

See [development and deployment](docs/development.md) for the complete workflow,
verification commands, current scope, and a guided tour of the Go code.

## Start here

- [Project brief](specs/README.md)
- [Core platform main spec](specs/completed/0-core-platform/main.md)
- [Architecture](specs/completed/0-core-platform/slices/0.2-architecture.md)
- [Frontend framework decision](specs/completed/0-core-platform/slices/0.5-frontend-framework.md)
- [Docker development and deployment](specs/completed/0-core-platform/slices/0.6-docker.md)
- [Editor GUI main spec](specs/completed/1-editor-gui/main.md)
- [File explorer UI](specs/completed/2-file-explorer-ui/main.md)
- [Development and deployment guide](docs/development.md)

## Current direction

- A simple Go backend owns the API, authentication, metadata, media processing,
  and storage behavior.
- A modern, responsive frontend consumes the Go API.
- Docker Compose is the supported interface for both local development and
  deployment through a Komodo-managed stack.
- Important behavior is specified and accepted before it is implemented.

The accepted frontend is React, Vite, TypeScript, Tailwind CSS, and shadcn/ui.
Every overarching feature has a permanent numbered main spec. Implementation
proceeds through its decimal child specs, each scoped to the smallest independently
testable unit. See the [specification map and authoring rule](specs/README.md).
