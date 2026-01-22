import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
  getGoogleAuthUrl,
  exchangeGoogleCode,
  setAuthToken,
  clearAuthToken,
} from '@/lib/api'
import { useAuth } from '@/contexts/AuthContext'

export function useLogin() {
  const navigate = useNavigate()
  const { refreshUser } = useAuth()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (code: string) => {
      const response = await exchangeGoogleCode(code)
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
        const checkClosed = setInterval(() => {
          if (popup.closed) {
            clearInterval(checkClosed)
            setIsOpening(false)
          }
        }, 500)
      } else {
        setIsOpening(false)
      }
    } catch {
      setIsOpening(false)
    }
  }, [isOpening])

  return { initiateLogin, isOpening }
}

import { useState, useCallback } from 'react'
