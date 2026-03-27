import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useDocument, useDocumentPage } from '../useDocument'
import * as api from '@/lib/api'

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }
}

describe('useDocument', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches document metadata', async () => {
    const mockDoc = {
      id: 1,
      filename: 'test.pdf',
      pageCount: 10,
      createdAt: '2026-03-21T00:00:00Z',
    }
    vi.spyOn(api, 'getDocument').mockResolvedValueOnce(mockDoc)

    const { result } = renderHook(() => useDocument(1), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(mockDoc)
  })

  it('does not fetch when id is undefined', () => {
    const spy = vi.spyOn(api, 'getDocument')
    renderHook(() => useDocument(undefined), { wrapper: createWrapper() })
    expect(spy).not.toHaveBeenCalled()
  })

  it('handles error state', async () => {
    vi.spyOn(api, 'getDocument').mockRejectedValueOnce(new Error('Not found'))
    const { result } = renderHook(() => useDocument(999), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isError).toBe(true))
  })
})

describe('useDocumentPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches page text', async () => {
    const mockPage = { pageNumber: 3, text: 'Page content', totalPages: 10 }
    vi.spyOn(api, 'getDocumentPage').mockResolvedValueOnce(mockPage)

    const { result } = renderHook(() => useDocumentPage(1, 3), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(mockPage)
  })

  it('does not fetch when documentId is undefined', () => {
    const spy = vi.spyOn(api, 'getDocumentPage')
    renderHook(() => useDocumentPage(undefined, 1), { wrapper: createWrapper() })
    expect(spy).not.toHaveBeenCalled()
  })
})
