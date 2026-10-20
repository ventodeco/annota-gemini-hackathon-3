import { useState, type ReactElement } from 'react'

interface ErrorReferenceProps {
  /** Request ID or other non-private identifier support can look up. */
  reference: string
  className?: string
}

/** A copyable support reference shown next to a failed operation. */
export default function ErrorReference({ reference, className = '' }: ErrorReferenceProps): ReactElement {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    // The reference stays selectable when the clipboard is unavailable.
    navigator.clipboard
      ?.writeText(reference)
      .then(() => setCopied(true))
      .catch(() => undefined)
  }

  return (
    <div className={`flex flex-wrap items-center justify-center gap-x-2 text-xs text-gray-500 ${className}`}>
      <span>
        Reference: <code className="select-all break-all font-mono">{reference}</code>
      </span>
      <button
        type="button"
        onClick={handleCopy}
        aria-label={`Copy reference ${reference}`}
        className="min-h-11 min-w-11 rounded-full px-3 font-medium text-gray-700 underline underline-offset-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-900"
      >
        {copied ? 'Copied' : 'Copy'}
      </button>
      <span aria-live="polite" className="sr-only">
        {copied ? 'Reference copied' : ''}
      </span>
    </div>
  )
}
