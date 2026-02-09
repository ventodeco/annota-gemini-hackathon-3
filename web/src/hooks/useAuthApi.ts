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

  const initiateLogin = useCallback(async () => {
    if (isOpening) return

    setIsOpening(true)
    try {
      const { ssoRedirection } = await getGoogleAuthUrl()
      window.location.assign(ssoRedirection)
    } catch {
      setIsOpening(false)
    }
  }, [isOpening])

  return { initiateLogin, isOpening }
}
