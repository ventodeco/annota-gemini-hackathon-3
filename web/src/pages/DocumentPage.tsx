import { useParams } from 'react-router-dom'
import { useState } from 'react'
import Header from '@/components/layout/Header'
import BottomActionBar from '@/components/layout/BottomActionBar'
import PageList from '@/components/documentpage/PageList'
import PageReader from '@/components/documentpage/PageReader'
import { AnnotationDrawer } from '@/components/scanpage/AnnotationDrawer'
import { useDocument, useDocumentPage } from '@/hooks/useDocument'
import { useTextSelection } from '@/hooks/useTextSelection'
import { useAnalyzeText, useCreateAnnotation } from '@/hooks/useAnnotations'
import { createScanFromPage } from '@/lib/api'
import type { Annotation } from '@/lib/types'

const MAX_ANNOTATION_VERSIONS = 2

type ViewMode = 'list' | 'reader'

export default function DocumentPage() {
  const { id } = useParams<{ id: string }>()
  const documentId = id ? parseInt(id, 10) : undefined

  const { data: document, isLoading, error } = useDocument(documentId)
  const [viewMode, setViewMode] = useState<ViewMode>('list')
  const [currentPage, setCurrentPage] = useState(1)
  const { data: pageData, isLoading: isPageLoading } = useDocumentPage(
    viewMode === 'reader' ? documentId : undefined,
    currentPage,
  )

  const { selectedText, handleSelection, clearSelection } = useTextSelection()
  const analyzeText = useAnalyzeText()
  const createAnnotation = useCreateAnnotation()

  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [annotationVersion, setAnnotationVersion] = useState(1)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState(false)
  const [contextText, setContextText] = useState('')
  const [bridgeScanId, setBridgeScanId] = useState<number | null>(null)

  const handleSelectPage = (pageNumber: number) => {
    setCurrentPage(pageNumber)
    setViewMode('reader')
    clearSelection()
  }

  const handleBackToList = () => {
    setViewMode('list')
    clearSelection()
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    setBridgeScanId(null)
  }

  const handlePrev = () => {
    if (currentPage > 1) {
      setCurrentPage((p) => p - 1)
      clearSelection()
    }
  }

  const handleNext = () => {
    const total = document?.pageCount ?? 0
    if (currentPage < total) {
      setCurrentPage((p) => p + 1)
      clearSelection()
    }
  }

  const handleTextSelect = () => {
    const selection = window.getSelection()
    if (!selection || selection.toString().trim() === '') {
      clearSelection()
      setContextText('')
      return
    }
    handleSelection(selection.toString())

    const range = selection.getRangeAt(0)
    const context = range.endContainer.textContent || ''
    setContextText(context)
  }

  const handleExplain = async () => {
    if (!selectedText || !documentId) return

    setIsLoadingAnnotation(true)

    try {
      let scanId = bridgeScanId
      if (!scanId) {
        const bridgeResult = await createScanFromPage(documentId, currentPage)
        scanId = bridgeResult.scanId
        setBridgeScanId(scanId)
      }

      const result = await analyzeText.mutateAsync({
        textToAnalyze: selectedText,
        context: contextText,
      })

      const annotation: Annotation = {
        id: Date.now(),
        user_id: 0,
        scan_id: scanId,
        highlighted_text: selectedText,
        context_text: contextText,
        nuance_data: result,
        is_bookmarked: true,
        created_at: new Date().toISOString(),
      }

      setCurrentAnnotation(annotation)
      setAnnotationVersion(1)
      setIsDrawerOpen(true)
    } catch (err) {
      console.error('Failed to analyze text:', err)
      alert('Failed to analyze text. Please try again.')
    } finally {
      setIsLoadingAnnotation(false)
    }
  }

  const handleSaveAnnotation = async () => {
    if (!currentAnnotation || !currentAnnotation.scan_id) return

    try {
      await createAnnotation.mutateAsync({
        scanId: currentAnnotation.scan_id,
        highlightedText: currentAnnotation.highlighted_text,
        contextText: currentAnnotation.context_text,
        nuanceData: currentAnnotation.nuance_data,
      })
      setIsDrawerOpen(false)
      setCurrentAnnotation(null)
      setAnnotationVersion(1)
      clearSelection()
    } catch (err) {
      console.error('Failed to save annotation:', err)
      alert('Failed to save annotation. Please try again.')
    }
  }

  const handleRegenerateAnnotation = async () => {
    if (!currentAnnotation || analyzeText.isPending || annotationVersion >= MAX_ANNOTATION_VERSIONS) {
      return
    }

    try {
      const result = await analyzeText.mutateAsync({
        textToAnalyze: currentAnnotation.highlighted_text,
        context: currentAnnotation.context_text ?? '',
      })

      setCurrentAnnotation({
        ...currentAnnotation,
        nuance_data: result,
        created_at: new Date().toISOString(),
      })
      setAnnotationVersion((prev) => Math.min(prev + 1, MAX_ANNOTATION_VERSIONS))
    } catch (err) {
      console.error('Failed to regenerate annotation:', err)
      alert('Failed to regenerate annotation. Please try again.')
    }
  }

  const handleDrawerClose = () => {
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    setAnnotationVersion(1)
    clearSelection()
  }

  if (isLoading) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Document" />
        <div className="flex-1 flex items-center justify-center p-6">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
        </div>
      </div>
    )
  }

  if (error || !document) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Document" />
        <div className="flex-1 flex items-center justify-center p-6">
          <p className="text-gray-600">Document not found</p>
        </div>
      </div>
    )
  }

  if (viewMode === 'list') {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title={document.filename} />
        <div className="flex-1 p-4">
          <p className="text-sm text-gray-500 mb-4">{document.pageCount} pages</p>
          <PageList pageCount={document.pageCount} onSelectPage={handleSelectPage} />
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <Header title={document.filename} onBack={handleBackToList} />
      <PageReader
        text={pageData?.text ?? ''}
        currentPage={currentPage}
        totalPages={document.pageCount}
        onPrev={handlePrev}
        onNext={handleNext}
        onTextSelect={handleTextSelect}
        isLoading={isPageLoading}
      />
      <BottomActionBar
        disabled={!selectedText || isLoadingAnnotation}
        isLoading={isLoadingAnnotation || analyzeText.isPending}
        onExplain={handleExplain}
      />
      <AnnotationDrawer
        isOpen={isDrawerOpen}
        onClose={handleDrawerClose}
        annotation={currentAnnotation}
        onRegenerate={handleRegenerateAnnotation}
        onSave={handleSaveAnnotation}
        isRegenerating={analyzeText.isPending}
        isSaving={createAnnotation.isPending}
        version={annotationVersion}
        maxVersions={MAX_ANNOTATION_VERSIONS}
      />
    </div>
  )
}
