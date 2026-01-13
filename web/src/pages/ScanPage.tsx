import { useParams } from 'react-router-dom'
import { useEffect, useState } from 'react'
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
  const [scanData, setScanData] = useState<GetScanResponse | null>(null)
  const [imageUrl, setImageUrl] = useState<string | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState(false)
  const { selectedText, handleSelection, clearSelection } = useTextSelection()

  useEffect(() => {
    if (!id) return

    const initializeScan = async () => {
      const mockScan = getMockScan(id)
      if (mockScan) {
        setScanData(mockScan)
        const url = getMockImageUrl(id)
        if (url) {
          setImageUrl(url)
        }
      }
    }

    initializeScan()
  }, [id])

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
      <div className="flex min-h-screen flex-col bg-white">
        <Header title="Scan Result" />
        <div className="flex flex-1 items-center justify-center p-6">
          <p className="text-gray-600">Scan not found</p>
        </div>
      </div>
    )
  }

  const { ocrResult } = scanData

  return (
    <div className="flex min-h-screen flex-col bg-white pb-20">
      <Header title="Scan Result" />
      <ScrollArea className="flex-1">
        <div className="p-6">
          {imageUrl && (
            <img
              src={imageUrl}
              alt="Scanned document"
              className="mb-6 w-full rounded-lg"
            />
          )}
          <p
            className="text-base/relaxed whitespace-pre-wrap text-gray-900"
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
