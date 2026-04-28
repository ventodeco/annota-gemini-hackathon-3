import { useCallback, useEffect, useRef, type ReactElement } from 'react'

type PageChange = { source: 'navigation' }

interface ExtractedTextReaderProps {
  pageText: string
  pageNumber: number
  totalPages: number
  isLoading: boolean
  onTextSelect: (selectedText: string) => void
  onPageChange: (page: number, change: PageChange) => void
}

function isSelectionInsideNode(selection: Selection, node: HTMLElement): boolean {
  if (selection.rangeCount === 0) {
    return false
  }

  const range = selection.getRangeAt(0)
  let ancestor: Node | null = range.commonAncestorContainer
  if (ancestor.nodeType === Node.TEXT_NODE) {
    ancestor = ancestor.parentNode
  }

  return ancestor ? node.contains(ancestor) : false
}

export default function ExtractedTextReader({
  pageText,
  pageNumber,
  totalPages,
  isLoading,
  onTextSelect,
  onPageChange,
}: ExtractedTextReaderProps): ReactElement {
  const contentRef = useRef<HTMLDivElement>(null)
  const trimmedPageText = pageText.trim()
  const hasText = trimmedPageText.length > 0

  const reportSelection = useCallback((clearWhenEmpty: boolean): void => {
    const selection = window.getSelection()
    const content = contentRef.current
    if (!selection || !content || selection.rangeCount === 0 || selection.toString().trim() === '') {
      if (clearWhenEmpty) {
        onTextSelect('')
      }
      return
    }

    if (isSelectionInsideNode(selection, content)) {
      onTextSelect(selection.toString())
      return
    }

    if (clearWhenEmpty) {
      onTextSelect('')
    }
  }, [onTextSelect])

  const handleSelectionEnd = useCallback((): void => {
    reportSelection(true)
  }, [reportSelection])

  useEffect(() => {
    const handleSelectionChange = (): void => {
      reportSelection(false)
    }

    document.addEventListener('selectionchange', handleSelectionChange)
    return () => document.removeEventListener('selectionchange', handleSelectionChange)
  }, [reportSelection])

  const goToPreviousPage = useCallback((): void => {
    onPageChange(pageNumber - 1, { source: 'navigation' })
  }, [onPageChange, pageNumber])

  const goToNextPage = useCallback((): void => {
    onPageChange(pageNumber + 1, { source: 'navigation' })
  }, [onPageChange, pageNumber])

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center p-6" role="status" aria-label="Loading reader text">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
      </div>
    )
  }

  return (
    <div className="flex flex-1 flex-col bg-stone-50">
      <div className="flex items-center justify-between border-b border-stone-200 bg-white px-4 py-3">
        <button
          type="button"
          className="min-h-11 rounded-lg border border-stone-200 px-4 text-sm font-medium text-stone-700 disabled:cursor-not-allowed disabled:opacity-40"
          onClick={goToPreviousPage}
          disabled={pageNumber <= 1}
        >
          Previous page
        </button>
        <p className="text-sm text-stone-500">
          Page {pageNumber} of {totalPages}
        </p>
        <button
          type="button"
          className="min-h-11 rounded-lg border border-stone-200 px-4 text-sm font-medium text-stone-700 disabled:cursor-not-allowed disabled:opacity-40"
          onClick={goToNextPage}
          disabled={pageNumber >= totalPages}
        >
          Next page
        </button>
      </div>
      <div className="flex-1 overflow-y-auto px-5 py-6">
        {hasText ? (
          <div
            ref={contentRef}
            data-testid="extracted-text-content"
            className="mx-auto max-w-2xl whitespace-pre-wrap rounded-2xl bg-white px-5 py-6 text-lg leading-9 text-stone-950 shadow-sm select-text"
            onMouseUp={handleSelectionEnd}
            onTouchEnd={handleSelectionEnd}
          >
            {pageText}
          </div>
        ) : (
          <div className="mx-auto max-w-sm rounded-2xl bg-white px-5 py-6 text-center shadow-sm">
            <p className="font-medium text-stone-900">No selectable text was found on this page.</p>
            <p className="mt-2 text-sm text-stone-500">Try PDF View or upload a text-based PDF.</p>
          </div>
        )}
      </div>
    </div>
  )
}
