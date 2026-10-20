import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import { ErrorBoundary } from '@/components/layout/ErrorBoundary'
import { AuthNotConfigured, ClerkRoot } from '@/components/auth/ClerkRoot'
import { ProtectedRoute, PublicRoute } from '@/components/auth/RouteGuards'
import LoginPage from '@/pages/LoginPage'
import SignUpPage from '@/pages/SignUpPage'
import WelcomePage from '@/pages/WelcomePage'
import CameraPage from '@/pages/CameraPage'
import LoadingPage from '@/pages/LoadingPage'
import HistoryPage from '@/pages/HistoryPage'
import OCRHistoryPage from '@/pages/OCRHistoryPage'
import ScanPage from '@/pages/ScanPage'
import AnnotationDetailPage from '@/pages/AnnotationDetailPage'
import DocumentPage from '@/pages/DocumentPage'
import DocumentsPage from '@/pages/DocumentsPage'
import NotFoundPage from '@/pages/NotFoundPage'
import LegalPage from '@/pages/LegalPage'
import ProfilePage from '@/pages/ProfilePage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

const CLERK_PUBLISHABLE_KEY: string = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY ?? ''

function PageErrorBoundary({ children }: { children: React.ReactNode }) {
  const location = useLocation()

  return (
    <ErrorBoundary resetKey={location.pathname}>
      {children}
    </ErrorBoundary>
  )
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        {CLERK_PUBLISHABLE_KEY ? (
          <ClerkRoot publishableKey={CLERK_PUBLISHABLE_KEY}>
            <AppRoutes />
          </ClerkRoot>
        ) : (
          <AuthNotConfigured />
        )}
        <Toaster />
      </BrowserRouter>
    </QueryClientProvider>
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
            <Navigate to="/login" replace />
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
