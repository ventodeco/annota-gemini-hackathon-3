import { useState, useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
  getGoogleAuthUrl,
  exchangeGoogleCode,
  setAuthToken,
  clearAuthToken,
} from '@/lib/api'
import { useAuth } from '@/contexts/useAuth'

export function useLogin() {
  const navigate = useNavigate()
  const { refreshUser } = useAuth()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ code, state }: { code: string; state: string }) => {
      const response = await exchangeGoogleCode(code, state)
      setAuthToken(response.token)
      return response
    },
    onSuccess: async () => {
      await refreshUser()
      queryClient.invalidateQueries({ queryKey: ['user'] })
      navigate('/welcome')
    },
    onError: () => {
      clearAuthToken()
    },
  })
}

export function useLogout() {
  const navigate = useNavigate()
  const { logout } = useAuth()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => {
      clearAuthToken()
    },
    onSuccess: () => {
      logout()
      queryClient.clear()
      navigate('/login')
    },
  })
}

export function useInitiateLogin() {
  const [isOpening, setIsOpening] = useState(false)
  const { refreshUser } = useAuth()
  const navigate = useNavigate()

  const initiateLogin = useCallback(async () => {
    if (isOpening) return

    setIsOpening(true)
    try {
      const { ssoRedirection } = await getGoogleAuthUrl()

      const width = 500
      const height = 700
      const left = (window.innerWidth - width) / 2
      const top = (window.innerHeight - height) / 2

      const popup = window.open(
        ssoRedirection,
        'google-login',
        `width=${width},height=${height},left=${left},top=${top}`
      )

      if (popup) {
        // Listen for message from popup
        const handleMessage = async (event: MessageEvent) => {
          if (event.origin !== window.location.origin) return

          if (event.data?.type === 'oauth-success') {
            window.removeEventListener('message', handleMessage)
            setIsOpening(false)
            await refreshUser()
            navigate('/welcome')
          } else if (event.data?.type === 'oauth-error') {
            window.removeEventListener('message', handleMessage)
            setIsOpening(false)
          }
        }
        window.addEventListener('message', handleMessage)

        const checkClosed = setInterval(() => {
          if (popup.closed) {
            clearInterval(checkClosed)
            window.removeEventListener('message', handleMessage)
            setIsOpening(false)
          }
        }, 500)
      } else {
        setIsOpening(false)
      }
    } catch {
      setIsOpening(false)
    }
  }, [isOpening, refreshUser, navigate])

  return { initiateLogin, isOpening }
}
