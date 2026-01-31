# CLAUDE.md

This document provides guidelines for AI agents working on the Gemini OCR+Annotation PWA project.

## Project Overview

A mobile-first PWA built with Go backend (JSON API) + React frontend that uses Gemini Flash for OCR and contextual annotations of Japanese text. The project enables users to scan book pages, convert content to digital text via OCR, and interactively highlight words or sentences to receive contextual explanations. See `docs/rfc.md` and `docs/prd.md` for detailed requirements.

## Specialized Agent Files

This project uses specialized agent files for each part of the codebase:

- **[`backend/CLAUDE.md`](backend/CLAUDE.md)** - Go backend development guidelines, build commands, code style, testing patterns, and best practices
- **[`web/CLAUDE.md`](web/CLAUDE.md)** - React/TypeScript frontend development guidelines, bun commands, shadcn MCP usage, TypeScript best practices, and frontend patterns

## Documentation

- [`docs/prd.md`](docs/prd.md) - Product requirements document
- [`docs/rfc.md`](docs/rfc.md) - Technical architecture document
- [`docs/task.md`](docs/task.md) - Implementation tasks

