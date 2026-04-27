import { Component, type ErrorInfo, type ReactNode } from 'react'
import { RotateCcw } from 'lucide-react'

import { Button } from '@/components/ui/button'

interface ErrorBoundaryProps {
  children: ReactNode
  resetKey?: string
  onReset?: () => void
}

interface ErrorBoundaryState {
  hasError: boolean
}

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
      return (
        <main className="min-h-screen bg-gray-50 px-4 py-6">
          <section
            role="alert"
            className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center gap-4 text-center"
          >
            <div className="space-y-2">
              <h1 className="text-xl font-semibold text-gray-900">Something went wrong</h1>
              <p className="text-sm leading-6 text-gray-600">
                This page could not finish rendering. Try again to reload this view.
              </p>
            </div>
            <Button type="button" onClick={this.handleRetry} className="min-h-11 px-5">
              <RotateCcw aria-hidden="true" />
              Try again
            </Button>
          </section>
        </main>
      )
    }

    return this.props.children
  }
}
