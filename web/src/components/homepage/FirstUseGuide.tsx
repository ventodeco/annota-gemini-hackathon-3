import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'

// Versioned so a future guide revision can be shown again to people who hid this one.
export const FIRST_USE_GUIDE_STORAGE_KEY = 'annota.firstUseGuide.v1'
const HIDDEN_VALUE = 'hidden'

const GUIDE_STEPS = [
  'Add something to read: take a photo, upload a photo or text PDF, or try the sample.',
  'Select Japanese text. On a phone, long-press a word and drag the handles.',
  'Choose “Explain this” for an English explanation in context.',
  'Press the speaker button to hear the selection read aloud.',
  'Choose “Save Annotation” to keep it. Reopen PDFs and their annotations from Documents, and photo scans from OCR History.',
]

function readHidden(): boolean {
  try {
    return localStorage.getItem(FIRST_USE_GUIDE_STORAGE_KEY) === HIDDEN_VALUE
  } catch {
    return false
  }
}

function writeHidden(hidden: boolean): void {
  try {
    if (hidden) {
      localStorage.setItem(FIRST_USE_GUIDE_STORAGE_KEY, HIDDEN_VALUE)
    } else {
      localStorage.removeItem(FIRST_USE_GUIDE_STORAGE_KEY)
    }
  } catch {
    // Storage can be unavailable (private mode); the choice then lasts for this visit only.
  }
}

export function FirstUseGuide() {
  const [hidden, setHidden] = useState<boolean>(readHidden)
  const hasToggledRef = useRef(false)
  const showButtonRef = useRef<HTMLButtonElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)

  // The toggled control unmounts, so move focus to its replacement instead of
  // dropping it to the page. Skipped on first render.
  useEffect(() => {
    if (!hasToggledRef.current) return
    if (hidden) {
      showButtonRef.current?.focus()
    } else {
      headingRef.current?.focus()
    }
  }, [hidden])

  const updateHidden = (next: boolean) => {
    writeHidden(next)
    hasToggledRef.current = true
    setHidden(next)
  }

  if (hidden) {
    return (
      <Button
        ref={showButtonRef}
        type="button"
        variant="ghost"
        className="min-h-11 text-sm text-gray-600"
        onClick={() => updateHidden(false)}
      >
        Show how ANNOTA works
      </Button>
    )
  }

  return (
    <section
      aria-labelledby="first-use-guide-title"
      className="w-full max-w-sm rounded-2xl border border-gray-200 bg-gray-50 p-4 text-left"
    >
      <h2
        id="first-use-guide-title"
        ref={headingRef}
        tabIndex={-1}
        className="text-base font-semibold text-gray-900 focus-visible:outline-2 focus-visible:outline-gray-900"
      >
        How ANNOTA works
      </h2>
      <ol className="mt-2 list-decimal space-y-1.5 pl-5 text-sm leading-6 text-gray-700">
        {GUIDE_STEPS.map((step) => (
          <li key={step}>{step}</li>
        ))}
      </ol>
      <Button type="button" variant="secondary" className="mt-3 min-h-11 rounded-full px-5" onClick={() => updateHidden(true)}>
        Hide guide
      </Button>
    </section>
  )
}
