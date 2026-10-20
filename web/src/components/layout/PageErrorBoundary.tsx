import type { ReactNode } from 'react'
import { useLocation } from 'react-router-dom'

import { ErrorBoundary } from '@/components/layout/ErrorBoundary'

export function PageErrorBoundary({ children }: { children: ReactNode }) {
  const location = useLocation()

  return (
    <ErrorBoundary resetKey={location.pathname}>
      {children}
    </ErrorBoundary>
  )
}
