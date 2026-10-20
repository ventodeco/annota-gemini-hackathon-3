import { Component, useEffect, useRef, type ErrorInfo, type ReactNode } from 'react'
import { Home, RotateCcw } from 'lucide-react'

import { Button, buttonVariants } from '@/components/ui/button'

interface ErrorBoundaryProps {
  children: ReactNode
  resetKey?: string
  onReset?: () => void
  // Replaces the full-page fallback for boundaries nested inside a page.
  fallback?: (reset: () => void) => ReactNode
}

interface ErrorBoundaryState {
  hasError: boolean
}

// React has no hook equivalent for catching render errors, so this stays a class.
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = {
    hasError: false,
  }

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('React render error caught by boundary', error, errorInfo)
  }

  componentDidUpdate(prevProps: ErrorBoundaryProps) {
    if (this.state.hasError && prevProps.resetKey !== this.props.resetKey) {
      this.setState({ hasError: false })
    }
  }

  private handleRetry = () => {
    this.props.onReset?.()
    this.setState({ hasError: false })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback(this.handleRetry)
      }
      return <PageErrorFallback onRetry={this.handleRetry} />
    }

    return this.props.children
  }
}

function PageErrorFallback({ onRetry }: { onRetry: () => void }) {
  const headingRef = useRef<HTMLHeadingElement>(null)

  // The crashed view took the focused element with it; move focus to the message.
  useEffect(() => {
    headingRef.current?.focus()
  }, [])

  return (
    <main className="min-h-screen bg-gray-50 px-4 py-6">
      <section
        role="alert"
        className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center gap-4 text-center"
      >
        <div className="space-y-2">
          <h1 ref={headingRef} tabIndex={-1} className="text-xl font-semibold text-gray-900 outline-none">
            Something went wrong
          </h1>
          <p className="text-sm leading-6 text-gray-600">
            This page could not finish rendering. Try again to reload this view.
          </p>
        </div>
        <div className="flex flex-col items-center gap-2">
          <Button type="button" onClick={onRetry} className="min-h-11 px-5">
            <RotateCcw aria-hidden="true" />
            Try again
          </Button>
          {/* A plain link works outside the router and discards broken in-memory state. */}
          <a href="/" className={buttonVariants({ variant: 'ghost', className: 'min-h-11 px-5' })}>
            <Home aria-hidden="true" />
            Go to home
          </a>
        </div>
      </section>
    </main>
  )
}
