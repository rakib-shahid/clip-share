# Main spec 0: Core platform

## Status

**Completed.** This main spec groups the accepted version-one product,
architecture, operations, and feature contracts that predate the numbered
main-spec convention.

## Outcome

Clip Share provides authenticated private libraries, nested folders, uploads and
durable media processing, public opaque share links, administration, and a
Docker-first deployment surface. The detailed completed contracts are preserved
as numbered child specs in [`slices/`](slices/).

## Child spec map

- `0.1`–`0.9`: product and cross-cutting platform contracts.
- `0.10`–`0.14`: version-one feature-family contracts.
- `0.15`–`0.19`: completed compatibility, UI, account, engineering, and storage
  summary increments.

These files are historical contracts and are broader than newly authored
slices. Do not split them merely to make smaller files: split only when changing
one of their behaviors, and put that successor work under the current numbered
main spec.

## Regression boundary

New work must preserve authentication, ownership, stable public IDs, storage
cleanup, ready-media compatibility, and Docker deployment unless its main spec
explicitly records and tests a compatibility change.
