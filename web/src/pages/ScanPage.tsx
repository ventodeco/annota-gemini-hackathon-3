import { useParams } from 'react-router-dom'
import { useState } from 'react'
import Header from '@/components/layout/Header'
import BottomActionBar from '@/components/layout/BottomActionBar'
import { ScrollArea } from '@/components/ui/scroll-area'
import { getMockScan, getMockImageUrl } from '@/lib/mockData'
import type { GetScanResponse, Annotation } from '@/lib/types'
import { useTextSelection } from '@/hooks/useTextSelection'
import { AnnotationDrawer } from '@/components/scanpage/AnnotationDrawer'
import { getMockAnnotation } from '@/lib/mockAnnotations'

export default function ScanPage() {
  const { id } = useParams<{ id: string }>()
  const [scanData] = useState<GetScanResponse | null>(() => {
    if (id) {
      return getMockScan(id) || null
    }
    return null
  })
  const [imageUrl] = useState<string | null>(() => {
    if (id) {
      return getMockImageUrl(id)
    }
    return null
  })
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState(false)
  const { selectedText, handleSelection, clearSelection } = useTextSelection()

  const handleTextSelect = () => {
    const selection = window.getSelection()
    if (!selection || selection.toString().trim() === '') {
      clearSelection()
      return
    }
    handleSelection(selection.toString())
  }

  const handleExplain = async () => {
    if (!selectedText) return
    
    setIsLoadingAnnotation(true)
    
    setTimeout(() => {
      const mockAnnotation = getMockAnnotation(selectedText)
      setCurrentAnnotation(mockAnnotation)
      setIsLoadingAnnotation(false)
      setIsDrawerOpen(true)
    }, 500)
  }

  const handleDrawerClose = () => {
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    clearSelection()
  }

  if (!scanData || !scanData.ocrResult) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Scan Result" />
        <div className="flex-1 flex items-center justify-center p-6">
          <p className="text-gray-600">Scan not found</p>
        </div>
      </div>
    )
  }

  const { ocrResult } = scanData

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <Header title="Scan Result" />
      <ScrollArea className="flex-1">
        <div className="p-6">
          {imageUrl && (
            <img
              src={imageUrl}
              alt="Scanned document"
              className="w-full mb-6 rounded-lg"
            />
          )}
          <p
            className="text-base leading-relaxed text-gray-900 whitespace-pre-wrap"
            onMouseUp={handleTextSelect}
            onTouchEnd={handleTextSelect}
          >
            {ocrResult.rawText}
          </p>
        </div>
      </ScrollArea>
      <BottomActionBar 
        disabled={!selectedText || isLoadingAnnotation}
        isLoading={isLoadingAnnotation}
        onExplain={handleExplain} 
      />
      <AnnotationDrawer
        isOpen={isDrawerOpen}
        onClose={handleDrawerClose}
        annotation={currentAnnotation}
      />
    </div>
  )
}
