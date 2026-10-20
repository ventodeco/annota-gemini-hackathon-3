import type { ReactElement } from 'react'

interface ScanFailureStateProps {
  title: string
  description: string
  actionLabel: string
  onAction: () => void
  secondaryActionLabel?: string
  onSecondaryAction?: () => void
}

const PRIMARY_BUTTON_CLASS =
  'min-h-11 rounded-full bg-gray-900 px-5 text-sm font-medium text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-offset-2'
const SECONDARY_BUTTON_CLASS =
  'min-h-11 rounded-full border border-gray-300 px-5 text-sm font-medium text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-offset-2'

/** A recoverable failure: what happened, what to do, and the actions to do it. */
export default function ScanFailureState({
  title,
  description,
  actionLabel,
  onAction,
  secondaryActionLabel,
  onSecondaryAction,
}: ScanFailureStateProps): ReactElement {
  return (
    <div className="flex flex-1 flex-col items-center justify-center p-6 text-center" role="alert">
      <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
      <p className="mt-2 max-w-xs text-sm text-gray-600">{description}</p>
      <div className="mt-6 flex flex-wrap justify-center gap-3">
        <button type="button" className={PRIMARY_BUTTON_CLASS} onClick={onAction}>
          {actionLabel}
        </button>
        {secondaryActionLabel && onSecondaryAction && (
          <button type="button" className={SECONDARY_BUTTON_CLASS} onClick={onSecondaryAction}>
            {secondaryActionLabel}
          </button>
        )}
      </div>
    </div>
  )
}
