import type { ReactNode } from 'react'

type LegalPageProps = {
  kind: 'privacy' | 'terms'
}

function PolicySection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <h2 className="text-lg font-semibold text-slate-900">{title}</h2>
      {children}
    </div>
  )
}

export default function LegalPage({ kind }: LegalPageProps) {
  const isPrivacy = kind === 'privacy'

  return (
    <main className="min-h-screen bg-white px-6 py-10 text-slate-900">
      <div className="mx-auto max-w-md space-y-6">
        <a
          href="/login"
          className="-mx-2 inline-flex min-h-11 items-center rounded-md px-2 text-sm font-medium text-slate-500 hover:text-slate-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900"
        >
          Back to ANNOTA
        </a>
        <header className="space-y-3">
          <p className="text-sm font-semibold uppercase tracking-[0.2em] text-slate-500">ANNOTA</p>
          <h1 className="text-3xl font-semibold tracking-tight">
            {isPrivacy ? 'Privacy Policy' : 'Terms of Service'}
          </h1>
          <p className="text-sm text-slate-500">Last updated: October 20, 2026</p>
        </header>

        {isPrivacy ? (
          <section className="space-y-6 text-base leading-7 text-slate-700">
            <PolicySection title="What ANNOTA stores">
              <p>
                Your ANNOTA account number linked to your Clerk sign-in, your email address, profile
                image link, and preferred explanation language. Your uploaded PDFs and page images, the
                text extracted from them, the annotations you save (selected text, surrounding context
                and the AI explanation), your reading progress, and usage counts for AI features.
                Text-to-speech audio is generated on request and not stored.
              </p>
            </PolicySection>
            <PolicySection title="Who processes it">
              <p>
                Sign-in is provided by Clerk, which processes your email address and, if you choose
                Google sign-in, your Google account identifier and profile image.
              </p>
              <p>
                OCR sends the page image, explanations send the selected text with its surrounding
                context, and speech sends only the selected text to the AI service the operator has
                configured: UCloud AstraFlow (ModelVerse) or OpenRouter, which forwards each request to the provider of the selected model. MiniMax
                and Google Gemini are also supported configurations. Each provider handles data under
                its own terms; ANNOTA does not promise zero retention or a particular processing region.
                Do not upload material you are not allowed to process with these services.
              </p>
            </PolicySection>
            <PolicySection title="Access and logs">
              <p>
                Your files, text and annotations are available only to your signed-in account and are
                not stored in the app&apos;s offline cache. Server logs record request metadata such as
                your account number, the route, status, timing and, for AI requests, the provider,
                model and usage. They are designed not to record your uploads, extracted text or
                selected text.
              </p>
            </PolicySection>
            <PolicySection title="Deleting your data">
              <p>
                Deleting a scan removes its image and text. Deleting a document removes the PDF and the
                text extracted from its pages. Annotations you saved are separate and stay until you
                delete them or your account. Deleting your account removes your files, records and
                Clerk sign-in. An AI request already in progress may still finish at the provider, but
                its result is discarded. Backups made before your deletion may still hold your data for
                the backup retention period set by the operator.
              </p>
            </PolicySection>
            <PolicySection title="Questions and requests">
              <p>
                Use Delete account in your profile to remove your data. For export, correction or other
                questions about data handling, contact the ANNOTA operator.
              </p>
            </PolicySection>
          </section>
        ) : (
          <section className="space-y-4 text-base leading-7 text-slate-700">
            <p>
              You are responsible for the material you upload and confirm you have the rights to
              upload, process, read, and annotate PDFs or book images in ANNOTA.
            </p>
            <p>
              ANNOTA provides AI-generated learning assistance. Translations, explanations, OCR, and
              speech output may be incomplete or inaccurate and should be reviewed before professional use.
            </p>
            <p>
              Commercial availability may include usage limits for OCR, annotation, document uploads,
              and text-to-speech to protect service reliability and AI provider costs.
            </p>
            <p>
              Do not use ANNOTA to upload unlawful content, bypass copyright restrictions, attack the
              service, or interfere with other users.
            </p>
          </section>
        )}
      </div>
    </main>
  )
}
