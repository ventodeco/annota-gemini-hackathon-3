# AGENTS.md

This document provides guidelines for AI agents working on the Gemini OCR+Annotation PWA project.

## Project Overview

A mobile-first PWA built with Go backend (JSON API) + React frontend that uses Gemini Flash for OCR and contextual annotations of Japanese text. The project enables users to scan book pages, convert content to digital text via OCR, and interactively highlight words or sentences to receive contextual explanations. See `docs/rfc.md` and `docs/prd.md` for detailed requirements.

## Specialized Agent Files

This project uses specialized agent files for each part of the codebase:

- **[`backend/AGENTS.md`](backend/AGENTS.md)** - Go backend development guidelines, build commands, code style, testing patterns, and best practices
- **[`web/AGENTS.md`](web/AGENTS.md)** - React/TypeScript frontend development guidelines, bun commands, shadcn MCP usage, TypeScript best practices, and frontend patterns

## Frontend and cross-cutting agent rules

Agents working on the web app (or on shared conventions that affect the frontend) must also follow the frontend `AGENTS.md`, including:

- **Vercel React best practices (mandatory)**: Before writing, editing, or reviewing React/TypeScript in `web/` (components, hooks, pages, data flow, effects, bundles), **read and apply the `vercel-react-best-practices` agent skill** (Vercel Engineering: performance, dependency arrays, effects, list rendering, and related patterns). This Vite + React SPA is not Next.js; use the parts of the skill that apply to client-rendered React. At the repository root, the same rule applies: any work that affects `web/` must invoke and follow that skill.
- **TypeScript: no `as` assertions**: **Do not add** TypeScript type assertions (`expr as Type` / `as const` is allowed where it is a literal assertion, not a type cast — prefer `satisfies` for literal types when it fits). Prefer type guards, discriminated unions, `satisfies`, generics, and validated parsing (e.g. schema validation) over casting. When changing existing code, **replace** unnecessary `as` with proper narrowing or types instead of adding new assertions. This applies to all TypeScript in `web/`; details and examples are in [`web/AGENTS.md`](web/AGENTS.md).

## Branching

- **`dev`**: Use for ongoing development, feature branches, and pull request bases.
- **`master`**: Frozen snapshot (Phase0-era app state); do not land new features here.

## Documentation

- [`docs/prd.md`](docs/prd.md) - Product requirements document
- [`docs/rfc.md`](docs/rfc.md) - Technical architecture document
- [`docs/task.md`](docs/task.md) - Implementation tasks
- [`docs/github-workflow.md`](docs/github-workflow.md) - GitHub issue tracking workflow with `gh` CLI

## Issue Tracking

All work must be tracked via GitHub Issues using GitHub CLI (`gh`).

See [`docs/github-workflow.md`](docs/github-workflow.md) for the complete workflow documentation.

### Quick Start

```bash
# Create issue
gh issue create --title "[TASK] Description"

# Work on main (or feature branch - see workflow doc)
git checkout main
git pull origin main

# Commit with reference
git commit -m "feat: description (#42)"

# Push
git push origin main

# Close issue
gh issue close #42
```

