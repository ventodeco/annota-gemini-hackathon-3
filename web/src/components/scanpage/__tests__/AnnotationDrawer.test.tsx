import { afterEach, describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AnnotationDrawer } from '../AnnotationDrawer'
import type { Annotation } from '@/lib/types'

vi.mock('@/hooks/useDrawerHeight', () => ({
  useDrawerHeight: () => ({
    drawerState: 'collapsed',
    expandDrawer: vi.fn(),
    collapseDrawer: vi.fn(),
  }),
  useDrawerGestures: () => ({
    handleDragEnd: vi.fn(),
  }),
}))

vi.mock('@/components/ui/sheet', () => ({
  Sheet: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SheetContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('framer-motion', () => ({
  motion: {
    div: (props: React.ComponentProps<'div'> & {
      drag?: unknown
      dragConstraints?: unknown
      dragElastic?: unknown
    }) => {
      const { children, ...rest } = props
      const sanitizedProps = { ...rest }
      delete sanitizedProps.drag
      delete sanitizedProps.dragConstraints
      delete sanitizedProps.dragElastic
      return <div {...sanitizedProps}>{children}</div>
    },
  },
}))

const contentRender = vi.hoisted(() => ({ shouldThrow: false }))

vi.mock('../AnnotationContent', () => ({
  AnnotationContent: () => {
    if (contentRender.shouldThrow) {
      throw new Error('malformed annotation')
    }
    return <div>annotation-content</div>
  },
}))

const annotation: Annotation = {
  id: 1,
  user_id: 1,
  scan_id: 1,
  highlighted_text: '今月はCVRが前月比+1.2pt',
  context_text: 'report context',
  nuance_data: {
    meaning: 'meaning',
    usageExample: 'example',
    usageTiming: 'timing',
    wordBreakdown: 'word',
    alternativeMeaning: 'alt',
  },
  is_bookmarked: true,
  created_at: new Date().toISOString(),
}

describe('AnnotationDrawer', () => {
  it('renders both action buttons in drawer footer', () => {
    render(
      <AnnotationDrawer
        isOpen
        onClose={vi.fn()}
        annotation={annotation}
        onRegenerate={vi.fn()}
        onSave={vi.fn()}
        version={1}
        maxVersions={2}
      />,
    )

    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save Annotation' })).toBeInTheDocument()
    expect(screen.getByText('Version 1/2')).toBeInTheDocument()
  })

  it('disables regenerate when version reaches cap', () => {
    render(
      <AnnotationDrawer
        isOpen
        onClose={vi.fn()}
        annotation={annotation}
        onRegenerate={vi.fn()}
        onSave={vi.fn()}
        version={2}
        maxVersions={2}
      />,
    )

    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeDisabled()
  })

  it('calls regenerate callback when clicked', async () => {
    const user = userEvent.setup()
    const onRegenerate = vi.fn()

    render(
      <AnnotationDrawer
        isOpen
        onClose={vi.fn()}
        annotation={annotation}
        onRegenerate={onRegenerate}
        onSave={vi.fn()}
        version={1}
        maxVersions={2}
      />,
    )

    await user.click(screen.getByRole('button', { name: 'Regenerate' }))

    expect(onRegenerate).toHaveBeenCalledTimes(1)
  })

  describe('when the annotation content fails to render', () => {
    afterEach(() => {
      contentRender.shouldThrow = false
      vi.restoreAllMocks()
    })

    it('shows a contained, closable fallback instead of crashing the page', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      contentRender.shouldThrow = true
      const user = userEvent.setup()
      const onClose = vi.fn()

      const { rerender } = render(
        <AnnotationDrawer isOpen onClose={onClose} annotation={annotation} />,
      )

      expect(screen.getByRole('alert')).toHaveTextContent('Explanation unavailable')
      expect(screen.getByRole('heading', { name: 'Explanation unavailable' })).toHaveFocus()
      expect(screen.queryByRole('button', { name: 'Save Annotation' })).not.toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: 'Close' }))
      expect(onClose).toHaveBeenCalledTimes(1)

      contentRender.shouldThrow = false
      rerender(<AnnotationDrawer isOpen={false} onClose={onClose} annotation={null} />)
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()

      rerender(<AnnotationDrawer isOpen onClose={onClose} annotation={annotation} />)
      expect(screen.getByText('annotation-content')).toBeInTheDocument()
    })
  })
})
