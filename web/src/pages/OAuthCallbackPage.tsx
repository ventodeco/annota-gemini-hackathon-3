import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useLogin } from '@/hooks/useAuthApi'

export default function OAuthCallbackPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [status, setStatus] = useState<'processing' | 'success' | 'error'>('processing')
  const loginMutation = useLogin()

  useEffect(() => {
    const code = searchParams.get('code')
    const state = searchParams.get('state')
    const error = searchParams.get('error')

    if (error) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setStatus('error')
      return
    }

    if (!code || !state) {
      return
    }

    loginMutation.mutate({ code, state }, {
      onSuccess: () => {
        setStatus('success')
        // If opened as popup, notify opener and close
        if (window.opener) {
          window.opener.postMessage({ type: 'oauth-success' }, window.location.origin)
          setTimeout(() => window.close(), 500)
        } else {
          // If not a popup, redirect normally
          setTimeout(() => {
            navigate('/welcome')
          }, 1500)
        }
      },
      onError: () => {
        setStatus('error')
        // If opened as popup, notify opener of error
        if (window.opener) {
          window.opener.postMessage({ type: 'oauth-error' }, window.location.origin)
        }
      },
    })
  }, [searchParams, loginMutation, navigate])

  return (
    <div className="min-h-screen bg-white flex flex-col items-center justify-center p-6">
      <div className="flex flex-col items-center gap-8 max-w-md w-full">
        {status === 'processing' && (
          <>
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900" />
            <p className="text-center text-gray-700 text-base">Processing your login...</p>
          </>
        )}

        {status === 'success' && (
          <>
            <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center">
              <svg className="w-8 h-8 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            </div>
            <p className="text-center text-gray-700 text-base">Login successful! Redirecting...</p>
          </>
        )}

        {status === 'error' && (
          <>
            <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center">
              <svg className="w-8 h-8 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </div>
            <p className="text-center text-gray-700 text-base">Login failed. Please try again.</p>
            <button
              onClick={() => navigate('/login')}
              className="px-6 py-2 bg-gray-900 text-white rounded-full text-sm font-medium"
            >
              Back to Login
            </button>
          </>
        )}
      </div>
    </div>
  )
}
