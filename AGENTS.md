# Working agreement

Read `specs/README.md` and the relevant documents in `specs/` before changing
the application. This project uses spec-driven development.

For a new feature:

1. Discuss goals, non-goals, user flows, edge cases, and acceptance criteria.
2. Add or update its hard-numbered `main.md`, then decompose accepted work into
   decimal child specs as required by `specs/README.md`.
3. Obtain user agreement on material choices before scaffolding or coding them.
4. Implement the smallest accepted child slice; it may be only one function or
   one state path when that is the narrowest independently testable unit.
5. Add proportionate automated tests and verify it through Docker Compose.
6. Update the spec when implementation reveals an intentional design change.

Keep Go code idiomatic, explicit, and small. Avoid introducing frameworks,
abstractions, concurrency, or clever patterns without a concrete need. Prefer
the standard library where it keeps the implementation understandable.

Docker Compose is the supported local-development and production deployment
interface. Keep secrets out of committed Compose files, keep persistent paths
explicit, and preserve compatibility with a Komodo-managed home-server stack.
