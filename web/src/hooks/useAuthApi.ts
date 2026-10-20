import { useMutation } from '@tanstack/react-query'
import { useAuth } from '@/contexts/useAuth'

/** Signs out of Clerk after clearing user-scoped caches; Clerk redirects to /login. */
export function useLogout() {
  const { logout } = useAuth()

  return useMutation({
    mutationFn: logout,
  })
}
