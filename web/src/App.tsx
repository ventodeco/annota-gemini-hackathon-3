import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import { ErrorBoundary } from '@/components/layout/ErrorBoundary'
import { AuthProvider } from '@/contexts/AuthContext'
import { useAuth } from '@/contexts/useAuth'
import LoginPage from '@/pages/LoginPage'
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
import OAuthCallbackPage from '@/pages/OAuthCallbackPage'
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

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
      </div>
    )
  }

  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />
}

function PublicRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
      </div>
    )
  }

  return isAuthenticated ? <Navigate to="/welcome" replace /> : <>{children}</>
}

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
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route
              path="/login"
              element={
                <PageErrorBoundary>
                  <PublicRoute>
                    <LoginPage />
                  </PublicRoute>
                </PageErrorBoundary>
              }
            />
            <Route
              path="/auth/callback"
              element={
                <PageErrorBoundary>
                  <OAuthCallbackPage />
                </PageErrorBoundary>
              }
            />
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
          <Toaster />
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}

export default App
