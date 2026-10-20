import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider } from '../AuthContext'
import { useAuth } from '../useAuth'
import { deleteAccount, getUserProfile } from '@/lib/api'
import { notifyUnauthorized } from '@/lib/authSession'

interface FakeClerkUser {
  primaryEmailAddress: { emailAddress: string }
  imageUrl: string
}

interface FakeClerkState {
  auth: { isLoaded: boolean; isSignedIn: boolean; userId: string | null }
  user: FakeClerkUser | null
  signOut: ReturnType<typeof vi.fn>
  /** Stable live instance, like the real useClerk(): reads current state. */
  instance: {
    signOut: (options: unknown) => unknown
    readonly user: { id: string } | null
    readonly session: { id: string } | null
  }
}

const clerk = vi.hoisted((): FakeClerkState => {
  const state: FakeClerkState = {
    auth: { isLoaded: true, isSignedIn: false, userId: null },
    user: null,
    signOut: vi.fn(),
    instance: {
      signOut: (options) => state.signOut(options),
      get user() {
        return state.auth.userId ? { id: state.auth.userId } : null
      },
      get session() {
        return state.auth.userId ? { id: `sess_${state.auth.userId}` } : null
      },
    },
  }
  return state
})

vi.mock('@clerk/react', () => ({
  useAuth: () => clerk.auth,
  useUser: () => ({ isLoaded: true, user: clerk.user }),
  useClerk: () => clerk.instance,
}))

vi.mock('@/lib/api', () => ({
  getUserProfile: vi.fn(),
  deleteAccount: vi.fn(),
}))

const cachesDelete = vi.fn()

/** Unsigned stand-in for a Clerk session JWT carrying `sub` and `sid`. */
function sessionToken(sub: string): string {
  return `header.${btoa(JSON.stringify({ sub, sid: `sess_${sub}` }))}.signature`
}

function rerenderProvider(view: ReturnType<typeof render>, queryClient: QueryClient) {
  view.rerender(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <Probe />
      </AuthProvider>
    </QueryClientProvider>,
  )
}

function setClerkSession(userId: string | null, email = 'learner@example.com') {
  clerk.auth = { isLoaded: true, isSignedIn: userId !== null, userId }
  clerk.user = userId === null
    ? null
    : { primaryEmailAddress: { emailAddress: email }, imageUrl: 'https://img.example/a.png' }
}

function Probe() {
  const { user, isLoading, isSignedIn } = useAuth()
  if (isLoading) return <p>loading</p>
  if (!isSignedIn) return <p>signed out</p>
  return <p>{user ? `${user.email} ${user.preferredLanguage}` : 'no profile'}</p>
}

function renderProvider(queryClient: QueryClient) {
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <Probe />
      </AuthProvider>
    </QueryClientProvider>,
  )
}

describe('AuthProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    clerk.signOut.mockResolvedValue(undefined)
    cachesDelete.mockResolvedValue(true)
    vi.stubGlobal('caches', { delete: cachesDelete })
    vi.mocked(getUserProfile).mockResolvedValue({ preferredLanguage: 'JP' })
  })

  it('combines Clerk identity with ANNOTA preferences', async () => {
    setClerkSession('user_a')
    renderProvider(new QueryClient())

    expect(await screen.findByText('learner@example.com JP')).toBeInTheDocument()
    expect(cachesDelete).not.toHaveBeenCalled()
  })

  it('clears cached data from the previous account before showing the next one', async () => {
    const queryClient = new QueryClient()
    setClerkSession('user_a', 'a@example.com')
    const view = renderProvider(queryClient)
    await screen.findByText('a@example.com JP')
    queryClient.setQueryData(['documents'], ['private document of A'])

    setClerkSession('user_b', 'b@example.com')
    vi.mocked(getUserProfile).mockResolvedValue({ preferredLanguage: 'EN' })
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <Probe />
        </AuthProvider>
      </QueryClientProvider>,
    )

    expect(screen.getByText('loading')).toBeInTheDocument()
    expect(await screen.findByText('b@example.com EN')).toBeInTheDocument()
    expect(queryClient.getQueryData(['documents'])).toBeUndefined()
    expect(cachesDelete).toHaveBeenCalledWith('api-cache')
  })

  it('clears caches when the session ends', async () => {
    const queryClient = new QueryClient()
    setClerkSession('user_a')
    const view = renderProvider(queryClient)
    await screen.findByText('learner@example.com JP')
    queryClient.setQueryData(['documents'], ['private'])

    setClerkSession(null)
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <Probe />
        </AuthProvider>
      </QueryClientProvider>,
    )

    expect(await screen.findByText('signed out')).toBeInTheDocument()
    expect(queryClient.getQueryData(['documents'])).toBeUndefined()
  })

  it('signs out of Clerk after an API 401', async () => {
    setClerkSession('user_a')
    renderProvider(new QueryClient())
    await screen.findByText('learner@example.com JP')

    act(() => notifyUnauthorized({ message: 'Unauthorized: invalid token', token: sessionToken('user_a') }))

    await waitFor(() =>
      expect(clerk.signOut).toHaveBeenCalledWith({ redirectUrl: '/login', sessionId: 'sess_user_a' }),
    )
    expect(deleteAccount).not.toHaveBeenCalled()
  })

  it('ignores a late 401 from a previous account after switching', async () => {
    const queryClient = new QueryClient()
    setClerkSession('user_a', 'a@example.com')
    const view = renderProvider(queryClient)
    await screen.findByText('a@example.com JP')

    setClerkSession('user_b', 'b@example.com')
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <Probe />
        </AuthProvider>
      </QueryClientProvider>,
    )
    await screen.findByText('b@example.com JP')

    act(() => notifyUnauthorized({ message: 'Unauthorized: account deleted', token: sessionToken('user_a') }))
    act(() => notifyUnauthorized({ message: 'Unauthorized: invalid token', token: sessionToken('user_a') }))
    act(() => notifyUnauthorized({ message: 'Unauthorized: missing token', token: null }))

    await new Promise((resolve) => setTimeout(resolve, 20))
    expect(deleteAccount).not.toHaveBeenCalled()
    expect(clerk.signOut).not.toHaveBeenCalled()
  })

  it('finishes a pending account deletion before signing out a deleted account', async () => {
    vi.mocked(deleteAccount).mockResolvedValue(undefined)
    setClerkSession('user_a')
    renderProvider(new QueryClient())
    await screen.findByText('learner@example.com JP')

    const token = sessionToken('user_a')
    act(() => {
      notifyUnauthorized({ message: 'Unauthorized: account deleted', token })
      notifyUnauthorized({ message: 'Unauthorized: account deleted', token })
    })

    await waitFor(() => expect(clerk.signOut).toHaveBeenCalled())
    expect(deleteAccount).toHaveBeenCalledTimes(1)
    expect(deleteAccount).toHaveBeenCalledWith({ token })
    expect(clerk.signOut).toHaveBeenCalledWith({ redirectUrl: '/login', sessionId: 'sess_user_a' })
  })

  it('does not sign out the next account when a deleted account finishes cleanup late', async () => {
    let finishCleanup: () => void = () => undefined
    vi.mocked(deleteAccount).mockReturnValue(
      new Promise<void>((resolve) => {
        finishCleanup = resolve
      }),
    )
    const queryClient = new QueryClient()
    setClerkSession('user_a', 'a@example.com')
    const view = renderProvider(queryClient)
    await screen.findByText('a@example.com JP')

    act(() => notifyUnauthorized({ message: 'Unauthorized: account deleted', token: sessionToken('user_a') }))
    expect(deleteAccount).toHaveBeenCalledWith({ token: sessionToken('user_a') })

    // Account B signs in while A's cleanup request is still pending.
    setClerkSession('user_b', 'b@example.com')
    rerenderProvider(view, queryClient)
    await screen.findByText('b@example.com JP')
    queryClient.setQueryData(['documents'], ['B document'])

    await act(async () => {
      finishCleanup()
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    expect(clerk.signOut).not.toHaveBeenCalled()
    expect(queryClient.getQueryData(['documents'])).toEqual(['B document'])
  })
})
