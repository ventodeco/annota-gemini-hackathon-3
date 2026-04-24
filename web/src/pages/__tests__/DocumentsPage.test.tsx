import React from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import DocumentsPage from '../DocumentsPage'

vi.mock('@/components/layout/Header', () => ({
  default: ({ title }: { title: string }) => <div>{title}</div>,
}))

vi.mock('@/components/layout/BottomNavigation', () => ({
  default: () => <div>Bottom Nav</div>,
}))

vi.mock('@/hooks/useDocument', () => ({
  useDocuments: () => ({
    data: {
      data: [
        {
          id: 1,
          filename: 'reader.pdf',
          pageCount: 8,
          lastPageNumber: 3,
          createdAt: '2026-02-09T00:00:00Z',
          updatedAt: '2026-02-09T00:00:00Z',
        },
      ],
      meta: { currentPage: 1, pageSize: 20 },
    },
    isLoading: false,
    error: null,
  }),
  useDeleteDocument: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

describe('DocumentsPage', () => {
  it('renders document library items with reading progress', () => {
    render(
      <MemoryRouter>
        <DocumentsPage />
      </MemoryRouter>
    )

    expect(screen.getByText('Documents')).toBeInTheDocument()
    expect(screen.getByText('reader.pdf')).toBeInTheDocument()
    expect(screen.getByText('8 pages, page 3')).toBeInTheDocument()
  })
})
