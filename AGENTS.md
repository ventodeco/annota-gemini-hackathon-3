# AGENTS.md

This document provides guidelines for AI agents working on the Gemini OCR+Annotation PWA project.

## Project Overview

A mobile-first PWA built with Go backend (JSON API) + React frontend that uses Gemini Flash for OCR and contextual annotations of Japanese text. The project enables users to scan book pages, convert content to digital text via OCR, and interactively highlight words or sentences to receive contextual explanations. See `docs/rfc.md` and `docs/prd.md` for detailed requirements.

## Specialized Agent Files

This project uses specialized agent files for each part of the codebase:

- **[`backend/AGENTS.md`](backend/AGENTS.md)** - Go backend development guidelines, build commands, code style, testing patterns, and best practices
- **[`web/AGENTS.md`](web/AGENTS.md)** - React/TypeScript frontend development guidelines, bun commands, shadcn MCP usage, TypeScript best practices, and frontend patterns

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

