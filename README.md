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
- [Architecture](specs/completed/general/architecture.md)
- [Frontend framework decision](specs/completed/general/frontend-framework.md)
- [Docker development and deployment](specs/completed/general/docker.md)
- [Accepted version-one feature scope](specs/completed/general/features.md)
- [Spec-driven workflow](specs/completed/general/workflow.md)
- [Development and deployment guide](docs/development.md)

## Current direction

- A simple Go backend owns the API, authentication, metadata, media processing,
  and storage behavior.
- A modern, responsive frontend consumes the Go API.
- Docker Compose is the supported interface for both local development and
  deployment through a Komodo-managed stack.
- Important behavior is specified and accepted before it is implemented.

The accepted frontend is React, Vite, TypeScript, Tailwind CSS, and shadcn/ui.
Implementation proceeds from the accepted specs in small vertical slices.
