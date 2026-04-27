type LegalPageProps = {
  kind: 'privacy' | 'terms'
}

export default function LegalPage({ kind }: LegalPageProps) {
  const isPrivacy = kind === 'privacy'

  return (
    <main className="min-h-screen bg-white px-6 py-10 text-slate-900">
      <div className="mx-auto max-w-md space-y-6">
        <a href="/login" className="text-sm font-medium text-slate-500 hover:text-slate-900">
          Back to ANNOTA
        </a>
        <header className="space-y-3">
          <p className="text-sm font-semibold uppercase tracking-[0.2em] text-slate-500">ANNOTA</p>
          <h1 className="text-3xl font-semibold tracking-tight">
            {isPrivacy ? 'Privacy Policy' : 'Terms of Service'}
          </h1>
          <p className="text-sm text-slate-500">Last updated: April 27, 2026</p>
        </header>

        {isPrivacy ? (
          <section className="space-y-4 text-base leading-7 text-slate-700">
            <p>
              ANNOTA stores your account profile, uploaded PDFs, scanned images, extracted text,
              annotations, pronunciation requests, and reading progress so you can continue learning.
            </p>
            <p>
              OCR, annotation, and speech features send selected content and surrounding learning
              context to Google Gemini for AI processing. Do not upload confidential material unless
              you are allowed to process it with this service.
            </p>
            <p>
              Your uploaded PDFs and images are private to your authenticated account. You can delete
              scans, documents, annotations, or delete your account to remove stored learning data.
            </p>
            <p>
              Contact the ANNOTA operator to request export, correction, deletion, or questions about
              subprocessors and data handling.
            </p>
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
