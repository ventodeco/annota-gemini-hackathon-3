# Frontend AGENTS.md

Guidelines for AI agents working on the React/TypeScript frontend of the Gemini OCR+Annotation PWA project.

## Agent-mandated rules (read first)

### Vercel React best practices skill (mandatory)

For any work on React components, hooks, effects, and client-side data loading, **read and follow the `vercel-react-best-practices` agent skill** before and while editing (invoke the skill in your environment; Vercel Engineering: performance, dependency arrays, unnecessary effects, list rendering, and related patterns). This project uses Vite, not Next.js; apply the rules that match a client-rendered React SPA. Root-level `AGENTS.md` also requires this skill for any agent work that affects `web/`.

### TypeScript: do not use `as` type assertions

**Do not add** type assertions (`expr as SomeType`) in new or edited code. Prefer type guards, discriminated unions, `satisfies` for checked literal shapes, well-typed router APIs, and generics. **`as const`** is allowed for const assertions on literals; avoid using `as` to force a value to an arbitrary type.

When you touch code that still contains legacy `as` casts, **refactor toward** proper narrowing (for example for `location.state`, define a runtime check or a typed wrapper) instead of piling on more assertions.

**Scan and navigation code** (`LoadingPage`, `useScan`, `ScanPage`, and their tests) should be migrated to assertion-free patterns when modified; treat those files as high-impact for type safety.

## Project Overview

React frontend (mobile-first PWA) that uses Gemini Flash for OCR and contextual annotations of Japanese text. See `../docs/rfc.md` and `../docs/prd.md` for detailed requirements.

## Build Commands

**Always use bun** - never npm, yarn, or pnpm.

```bash
# Install dependencies
cd web && bun install

# Run dev server
cd web && bun run dev

# Build for production
cd web && bun run build

# Run tests (Vitest)
cd web && bun test

# Run tests with coverage
cd web && bun run test:coverage

# Run tests in watch mode
cd web && bun run test:watch
```

## Package Manager

**Always use bun** - never npm, yarn, or pnpm. Use `bun install`, `bun run`, and `bunx` for all package management and script execution.

## Environment Variables

Frontend-relevant environment variables (API endpoints, etc.) are configured via Vite environment variables. See `vite.config.ts` for proxy configuration.

## Code Style Guidelines

### TypeScript Best Practices

- **Strict mode**: Enabled (`strict: true` in tsconfig)
- **Explicit types**: Define types for props, state, and API responses
  ```typescript
  interface ComponentProps {
    title: string;
    age: number;
  }
  ```
- **State typing**: Always type useState: `useState<Type>(initialValue)`
- **Never use `any`**: Use `unknown` and type guards instead (avoid `as` casts; see [Agent-mandated rules](#agent-mandated-rules-read-first))
  ```typescript
  function isUserRecord(x: object): x is { name: unknown } {
    return 'name' in x;
  }
  function parseJSON(json: unknown): User | null {
    if (typeof json !== 'object' || json === null) return null;
    if (!isUserRecord(json)) return null;
    if (typeof json.name !== 'string') return null;
    return { name: json.name };
  }
  ```
- **Utility types**: Use `Partial<T>`, `Pick<T, K>`, `Omit<T, K>` for flexible types
- **Enums**: Use enums for component variants
  ```typescript
  enum ButtonVariants {
    Primary = 'primary',
    Secondary = 'secondary',
  }
  ```
- **Async functions**: Explicitly type return: `async function(): Promise<Type>`
- **Type inference**: Prefer inference where clear, explicit where ambiguous
- **No `as` type assertions**: Do not add `as` casts; prefer guards and `satisfies`. See [Agent-mandated rules](#agent-mandated-rules-read-first).

### React Best Practices

- **Hooks rules**: Only call hooks at top level, use proper dependency arrays
- **Functional components**: Use functional components with hooks (no class components)
- **Component composition**: Prefer composition over inheritance
- **Memoization**: Use `useMemo`/`useCallback` only when needed for performance
- **Cleanup**: Always return cleanup functions from `useEffect` when needed
  ```typescript
  useEffect(() => {
    const timer = setInterval(() => {}, 1000);
    return () => clearInterval(timer);
  }, []);
  ```

### Import Organization

Group imports in this order:
1. React imports
2. Third-party packages
3. Internal modules
4. Types (if separate)

```typescript
import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import type { Scan } from '@/lib/types';
```

## shadcn/ui Components

**Primary method**: Use MCP shadcn server when available.

**Fallback**: Use CLI command `bunx shadcn@latest add <component-name>`

**Never manually create shadcn components** - always use one of these methods. Components are configured via `components.json` (already set up).

## Tailwind CSS Guidelines

- Use utility classes for all styling; avoid custom CSS when possible
- Mobile-first: use `sm:`, `md:`, `lg:`, `xl:` breakpoints for responsive design
- Common utility patterns:
  - Spacing: `p-4`, `px-6`, `py-3`, `m-2`, `gap-4`
  - Flexbox: `flex`, `flex-col`, `justify-between`, `items-center`, `gap-4`
  - Typography: `text-lg`, `font-bold`, `text-gray-700`, `leading-relaxed`
  - Colors: `bg-white`, `text-gray-900`, `border-gray-200`, `bg-blue-500`
  - Interactive: `hover:bg-blue-600`, `focus:ring-2`, `active:bg-blue-700`
  - Touch targets: minimum `p-4` (16px) for buttons on mobile
  - Rounded: `rounded-lg`, `rounded-full`
  - Shadows: `shadow-md`, `shadow-lg`
- Group related classes conceptually for readability

## Frontend Patterns

- **Progressive Enhancement**: Ensure basic functionality works without JS
- **Loading States**: Show loading indicators during async operations
- **Error Handling**: Display user-friendly errors inline, not full page reloads
- **Mobile UX**:
  - Large touch targets (minimum 44x44px recommended)
  - Adequate spacing between interactive elements
  - Readable font sizes (`text-base` minimum, `text-lg` for body text)
- **PWA**: Include `manifest.webmanifest` and service worker for installability
- **Icons**: Use lucide-react icons (configured in `components.json`)

## Testing

- Use Vitest + React Testing Library for component and hook tests
- Write type-safe custom hooks with explicit return types
- Test user interactions, not implementation details
- Use `@testing-library/user-event` for realistic user interactions

Example:
```typescript
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Button } from './Button';

describe('Button', () => {
  it('renders correctly', () => {
    render(<Button>Click me</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();
  });
});
```

## Project Structure

```
web/
  src/
    components/      # React components
      ui/            # shadcn/ui components
      camera/        # Camera-related components
      layout/        # Layout components
      scanpage/      # Scan page components
    pages/           # Page components
    hooks/           # Custom React hooks
    lib/             # Utilities, API client, types
    test/            # Test setup and utilities
  public/            # Static assets
  dist/              # Build output (gitignored)
  components.json    # shadcn/ui configuration
  package.json       # Frontend dependencies
  vite.config.ts     # Vite configuration
```

## Issue Tracking Workflow

All work must be tracked via GitHub Issues using the GitHub CLI (`gh`).

See the complete workflow documentation: [`../docs/github-workflow.md`](../docs/github-workflow.md)

### Quick Start

```bash
# Create issue for your work
gh issue create --label frontend --title "[FEATURE] Description"

# Work on main (or create feature branch - see workflow doc)
git checkout main
git pull origin main

# Commit with issue reference
git commit -m "feat: description (#23)"

# Push
git push origin main

# Close issue when done
gh issue close #23
```

### Frontend-Specific Labels

- `frontend` - Frontend related
- `ui` - UI component changes
- `camera` - Camera-related features
- `performance` - Performance optimization
- `pwa` - PWA-related changes
- `mobile` - Mobile-specific changes
