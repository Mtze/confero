# Confero

> **Confero** — Latin *cōnferō*, "to bring together, to discuss, to contribute"
> — the etymological root of *conference*.

A small internal tool for the TUM CS-Education chair to keep an
organized, shared overview of relevant academic conferences, with
starring, deadline reminders, and a clean API-first architecture.

This file is a placeholder for v0.1; a full quickstart and deploy
guide land with milestone **M13**. Until then, the design docs in
[`docs/`](./docs/) are the source of truth.

## Where to look

| If you want to ...                          | Read                                                |
| ------------------------------------------- | --------------------------------------------------- |
| Understand what the system does             | [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md)    |
| Understand how it is built                  | [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md)    |
| Understand the database / domain shape      | [`docs/DATA_MODEL.md`](./docs/DATA_MODEL.md)        |
| Follow the milestone-by-milestone build     | [`docs/IMPLEMENTATION_PLAN.md`](./docs/IMPLEMENTATION_PLAN.md) |
| See locked architectural decisions          | [`docs/adr/`](./docs/adr/)                          |
| Contribute as a human or AI agent           | [`AGENTS.md`](./AGENTS.md), [`CLAUDE.md`](./CLAUDE.md) |

## Repo layout

The canonical layout lives in
[`docs/ARCHITECTURE.md §2`](./docs/ARCHITECTURE.md#2-repository-layout).
At a glance:

```
api/        # OpenAPI 3.1 spec — single source of truth
server/     # Go API + in-process scheduler (one binary, distroless)
web/        # Vite + React + TypeScript SPA (served via nginx)
bruno/      # Bruno API collection
deploy/     # docker-compose for local dev + Helm chart for k8s
docs/       # design docs and ADRs
```

## Quickstart (placeholder)

A real `make dev` workflow ships with **M3** (auth) and is finalized
with the Helm chart in **M12**. Until then the only useful targets
are `make help`, `make lint`, `make test`, and `make build`.

## Status

This repository is under active initial development. The milestone
plan is [`docs/IMPLEMENTATION_PLAN.md`](./docs/IMPLEMENTATION_PLAN.md);
the current milestone label appears on the latest commit.

## License

TBD (will be set in milestone M13).
