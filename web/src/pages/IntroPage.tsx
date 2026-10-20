import type { ReactNode } from 'react'
import { Link, Navigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '@/contexts/useAuth'
import { safeRedirectPath } from '@/lib/authSession'
import { SAMPLE_PDF_URL } from '@/lib/sample'

const linkClass =
  'inline-flex min-h-11 items-center justify-center rounded-full px-6 text-sm font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900'
const primaryLinkClass = `${linkClass} bg-gray-900 text-white hover:bg-gray-800`
const secondaryLinkClass = `${linkClass} border border-gray-200 bg-white text-gray-900 hover:bg-gray-50`
const textLinkClass =
  'font-medium text-gray-900 underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900'
const navLinkClass = `${textLinkClass} inline-flex min-h-11 items-center`

const WORKFLOW_STEPS = [
  'Add a Japanese text PDF, or take or upload a photo of a book page.',
  'Select a word or sentence. On a phone, long-press and drag the handles.',
  'Choose “Explain this” for an English explanation of its meaning, usage and nuance in context.',
  'Play the selected text to hear it read aloud.',
  'Save the annotation. Reopen PDFs from Documents and photo scans from OCR History.',
]

const LIMITATIONS = [
  'Text-based PDFs up to 10 MB. Their text is read straight from the file, without OCR.',
  'Page photos in JPEG, PNG or WebP up to 10 MB, read with AI OCR.',
  'Scanned, image-only PDFs are not supported yet. Take a photo of each page instead.',
  'Explanations are written in English.',
  'Photo scans, explanations and speech have daily limits per account. Your remaining allowance is shown after you sign in.',
  'AI output can be incomplete or wrong, so check anything important.',
]

function IntroSection({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="space-y-3">
      <h2 id={id} className="text-xl font-semibold text-gray-900">
        {title}
      </h2>
      {children}
    </section>
  )
}

export default function IntroPage() {
  const { isSignedIn } = useAuth()
  const [searchParams] = useSearchParams()
  // A visitor who reached the intro from a sign-in redirect keeps their destination.
  const redirectPath = safeRedirectPath(searchParams.get('redirect_url'))
  const redirectQuery = redirectPath ? `?redirect_url=${encodeURIComponent(redirectPath)}` : ''

  // Static content renders while Clerk loads so slow or offline visitors still see it.
  if (isSignedIn) return <Navigate to={redirectPath ?? '/welcome'} replace />

  return (
    <div className="min-h-screen bg-white text-gray-700">
      <header className="mx-auto flex max-w-2xl items-center justify-between gap-4 px-6 pt-6">
        <img src="/logo-anota.png" alt="ANNOTA" className="h-10 w-auto" />
        <Link to={`/login${redirectQuery}`} className={navLinkClass}>
          Sign in
        </Link>
      </header>

      <main className="mx-auto max-w-2xl space-y-10 px-6 py-10 text-base leading-7">
        <div className="space-y-4">
          <p className="inline-flex rounded-full bg-orange-50 px-3 py-1 text-sm font-medium text-orange-800">
            Public beta
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-gray-900">
            Read real Japanese, with an explanation one tap away
          </h1>
          <p>
            ANNOTA is a reading companion for Japanese learners. Open a PDF or a photo of a page, select the words
            you don&apos;t know, and get an English explanation in the context of what you&apos;re reading. You can
            listen to it and save it for later.
          </p>
          <div className="flex flex-wrap gap-3">
            <Link to={`/sign-up${redirectQuery}`} className={primaryLinkClass}>
              Create an account
            </Link>
            <Link to={`/login${redirectQuery}`} className={secondaryLinkClass}>
              Sign in
            </Link>
          </div>
        </div>

        <IntroSection id="intro-how" title="How it works">
          <ol className="list-decimal space-y-2 pl-6">
            {WORKFLOW_STEPS.map((step) => (
              <li key={step}>{step}</li>
            ))}
          </ol>
        </IntroSection>

        <IntroSection id="intro-sample" title="Try the sample">
          <p>
            The sample is a short, original two-page Japanese reading: one page of horizontal text and one of
            vertical text. It is free to share (CC0).
          </p>
          <p>
            <a href={SAMPLE_PDF_URL} target="_blank" rel="noopener" className={textLinkClass}>
              View the sample PDF
            </a>{' '}
            (read-only). After you sign in, choose &ldquo;Try the sample&rdquo; on the start screen. ANNOTA adds a
            private copy to your Documents for you to annotate. You can delete it at any time.
          </p>
        </IntroSection>

        <IntroSection id="intro-limits" title="What works in the beta">
          <ul className="list-disc space-y-2 pl-6">
            {LIMITATIONS.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
          <p>ANNOTA runs in current mobile and desktop browsers. Taking photos needs camera permission.</p>
        </IntroSection>

        <IntroSection id="intro-privacy" title="Accounts and privacy">
          <p>
            Sign-in is handled by Clerk, with Google or email. To read photos, explain text and speak it, ANNOTA
            sends the page image or the selected text with its surrounding context to the AI service its operator
            has configured. Your files and annotations are only available to your account.
          </p>
          <p>
            Read the{' '}
            <Link to="/privacy" className={textLinkClass}>
              Privacy Policy
            </Link>{' '}
            and{' '}
            <Link to="/terms" className={textLinkClass}>
              Terms of Service
            </Link>
            .
          </p>
        </IntroSection>
      </main>

      <footer className="mx-auto flex max-w-2xl flex-wrap gap-x-6 gap-y-2 border-t border-gray-100 px-6 py-6 text-sm">
        <span className="inline-flex min-h-11 items-center">ANNOTA (beta)</span>
        <Link to="/privacy" className={navLinkClass}>
          Privacy
        </Link>
        <Link to="/terms" className={navLinkClass}>
          Terms
        </Link>
      </footer>
    </div>
  )
}
