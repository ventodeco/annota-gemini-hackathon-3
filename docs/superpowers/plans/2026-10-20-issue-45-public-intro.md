# Issue #45: public beta introduction, lawful sample and first-use guidance

## Finish condition

A signed-out visitor at `/` can understand what ANNOTA does, which files work (and which don't), see the beta limits, reach the existing Clerk sign-in with the intended destination preserved, and open a lawful sample read-only. A signed-in user can add the same sample to their own library with one tap (per-user copy, no shared learning records) and sees short, skippable, accessible first-use guidance. Public title/metadata/PWA identity consistently say ANNOTA.

## User story

As a new Japanese learner, I want to understand the product and try a sample so that I can reach my first useful explanation without preparing a document.

## Scope (files)

- `web/src/pages/IntroPage.tsx` (new): public landing at `/`.
  - Real workflow: add a text PDF or page photo → select Japanese text → English explanation → listen → save → find it in History/Documents.
  - Supported material, matching the backend: text-based PDFs and JPEG/PNG/WebP photos, up to 10 MB each (`MAX_UPLOAD_SIZE` default and `isValidImageType`). Image-only (scanned) PDFs are unsupported; the workaround is to photograph the pages. Explanations are English only.
  - Beta status and limits: per-account daily AI allowances, shown after sign-in. Numbers are not hardcoded because they are configurable.
  - Auth and AI processors: Clerk (Google or email), plus the operator-configured AI provider. Links to `/privacy` and `/terms`.
  - "Sign in", "Create account", and "View sample PDF" (static, read-only file).
  - Signed-in visitors are redirected to `/welcome`. Static content renders while Clerk is loading, so slow or offline connections never get a blank spinner.
- `web/src/App.tsx`: `/` renders IntroPage instead of redirecting to `/login`. All other routes are unchanged.
- `web/src/pages/LoginPage.tsx` (`AuthPageLayout`): replace the generic slogan with a one-line product description and an "About ANNOTA" link to `/`. `redirect_url` handling is untouched; ProtectedRoute already preserves direct reader links.
- `web/src/pages/LegalPage.tsx`: "Back to ANNOTA" goes to `/` (signed-in users are forwarded to `/welcome`).
- `web/src/pages/WelcomePage.tsx`:
  - "Try the sample" fetches `/samples/annota-sample.pdf` and uploads it through the existing `pdfUploadMutation`. Each user gets their own document; reset means deleting that document and adding the sample again.
  - The guidance block is `FirstUseGuide`, a non-modal section with an ordered list: upload/sample, select (long-press on phones), Explain, Listen, Save, return through Documents/History. "Hide guide" dismisses it and the choice is persisted in versioned localStorage. A "Show guide" button brings it back.
  - #44 owns the upload mutation handlers and the upload error block. This issue does not edit them.
- `web/public/samples/annota-sample.pdf` (new): 2 pages of original Japanese text written for ANNOTA, dedicated CC0 1.0. Page 1 is horizontal and page 2 is vertical (tategaki). It is typeset in Sawarabi Mincho (SIL OFL 1.1, embedded subset), about 66 KB. Both pages extract cleanly with the backend `ledongthuc/pdf` extractor. Fonts that Chrome emits as Type3 (variable/CFF Noto) did not extract.
- `web/public/samples/README.md`: rights and font notice for the sample.
- `web/index.html` and `web/vite.config.ts`: title `ANNOTA`, description, Open Graph/Twitter text metadata. Manifest name/short_name `ANNOTA` (was "Gemini OCR+Annotation PWA" / "ANOTA"). Add 192/512 PNG icons and a 180 PNG apple-touch-icon rendered from the existing `pwa-icon.svg`.

## Out of scope

- Support/Contact page: deferred to #80, so the support-link acceptance stays pending there. No invented address and no link to a page that doesn't exist.
- Reader/progress files (#23 PR77).
- Upload mutation/error code (#44).
- New auth or AI integration, quota changes, backend changes.
- A shared read-only sample backend: the per-user upload copy gives isolation without new storage contracts.
- `og:image`: it needs an absolute production URL. That belongs to release setup.

## Edge cases

- Signed-out visitor sees the intro. A returning signed-in visitor is redirected to `/welcome`.
- A direct reader link goes to `/login?redirect_url=…` (existing behaviour). The login page links to the intro.
- No personal file: the sample is available. Sample reset or deletion goes through the existing document delete.
- Slow or offline connection: the intro is static. If the sample fetch fails, the existing inline upload error is shown.
- Unsupported PDF: the intro and the existing PDF note explain the photo workaround.
- Small viewport: single-column layout, 44px targets.
- Skipped guidance stays hidden and can be shown again.
- Browser back: no history is pushed for redirects (`replace`).

## Google / Clerk requirements (verified 2026-10-20, primary source)

Google OAuth production policy requires the following. The home page must be publicly accessible and describe the app's functionality. It must link to a privacy policy (terms optional) and be hosted on a verified domain the owner controls. The app name and logo must accurately represent the app.

This PR delivers the public description page with privacy and terms links. Domain verification, consent-screen branding, and production Clerk configuration are owner release setup and remain pending.

## Verification

- Focused Vitest:
  - IntroPage content and links, with a signed-in redirect.
  - WelcomePage sample upload calls `uploadDocument` with a PDF named `annota-sample.pdf`, and guide dismissal persists.
- `bun run lint`, `bun run build`, and the CI required checks.
- One desktop and one mobile screenshot of `/` and the welcome guide, recorded in the PR.
- Live Clerk sign-in and AI explanation/speech/save on the sample wait for owner keys.
