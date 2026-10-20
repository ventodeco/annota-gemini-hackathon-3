import { Suspense, lazy } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import { ErrorBoundary } from '@/components/layout/ErrorBoundary'
import { PageErrorBoundary } from '@/components/layout/PageErrorBoundary'
import { AuthNotConfigured, ClerkRoot } from '@/components/auth/ClerkRoot'
import { ProtectedRoute, PublicRoute } from '@/components/auth/RouteGuards'
import LoginPage from '@/pages/LoginPage'
import SignUpPage from '@/pages/SignUpPage'
import IntroPage from '@/pages/IntroPage'
import NotFoundPage from '@/pages/NotFoundPage'
import LegalPage from '@/pages/LegalPage'

// Signed-in pages load on demand so public and sign-in routes don't download the reader,
// PDF.js or camera code. A failed chunk load is caught by the route's PageErrorBoundary.
const WelcomePage = lazy(() => import('@/pages/WelcomePage'))
const CameraPage = lazy(() => import('@/pages/CameraPage'))
const LoadingPage = lazy(() => import('@/pages/LoadingPage'))
const HistoryPage = lazy(() => import('@/pages/HistoryPage'))
const OCRHistoryPage = lazy(() => import('@/pages/OCRHistoryPage'))
const ScanPage = lazy(() => import('@/pages/ScanPage'))
const AnnotationDetailPage = lazy(() => import('@/pages/AnnotationDetailPage'))
const DocumentPage = lazy(() => import('@/pages/DocumentPage'))
const DocumentsPage = lazy(() => import('@/pages/DocumentsPage'))
const ProfilePage = lazy(() => import('@/pages/ProfilePage'))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

const CLERK_PUBLISHABLE_KEY: string = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY ?? ''

function RouteFallback() {
  return (
    <div className="min-h-screen flex items-center justify-center" role="status" aria-busy="true">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
      <span className="sr-only">Loading page…</span>
    </div>
  )
}

function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          {CLERK_PUBLISHABLE_KEY ? (
            <ClerkRoot publishableKey={CLERK_PUBLISHABLE_KEY}>
              <Suspense fallback={<RouteFallback />}>
                <AppRoutes />
              </Suspense>
            </ClerkRoot>
          ) : (
            <AuthNotConfigured />
          )}
          <Toaster />
        </BrowserRouter>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}

function AppRoutes() {
  return (
    <Routes>
      <Route
        path="/login/*"
        element={
          <PageErrorBoundary>
            <PublicRoute>
              <LoginPage />
            </PublicRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/sign-up/*"
        element={
          <PageErrorBoundary>
            <PublicRoute>
              <SignUpPage />
            </PublicRoute>
          </PageErrorBoundary>
        }
      />
      {/* Legacy Google OAuth callback links from pre-Clerk builds. */}
      <Route path="/auth/callback" element={<Navigate to="/login" replace />} />
      <Route
        path="/privacy"
        element={
          <PageErrorBoundary>
            <LegalPage kind="privacy" />
          </PageErrorBoundary>
        }
      />
      <Route
        path="/terms"
        element={
          <PageErrorBoundary>
            <LegalPage kind="terms" />
          </PageErrorBoundary>
        }
      />
      <Route
        path="/welcome"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <WelcomePage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/camera"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <CameraPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/loading/:id"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <LoadingPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/history"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <HistoryPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/scans-history"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <OCRHistoryPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/scans/:id"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <ScanPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/documents"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <DocumentsPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/documents/:id"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <DocumentPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/profile"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <ProfilePage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/annotations/:id"
        element={
          <PageErrorBoundary>
            <ProtectedRoute>
              <AnnotationDetailPage />
            </ProtectedRoute>
          </PageErrorBoundary>
        }
      />
      <Route
        path="/"
        element={
          <PageErrorBoundary>
            <IntroPage />
          </PageErrorBoundary>
        }
      />
      <Route
        path="*"
        element={
          <PageErrorBoundary>
            <NotFoundPage />
          </PageErrorBoundary>
        }
      />
    </Routes>
  )
}

export default App
