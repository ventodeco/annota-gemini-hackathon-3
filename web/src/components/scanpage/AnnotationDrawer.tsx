import { useEffect, useCallback, useState } from 'react'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { motion } from 'framer-motion'
import { useDrawerHeight, useDrawerGestures } from '@/hooks/useDrawerHeight'
import { DrawerHeader } from './DrawerHeader'
import { AnnotationContent } from './AnnotationContent'
import { Button } from '@/components/ui/button'
import { Bookmark, BookmarkCheck } from 'lucide-react'
import { toast } from 'sonner'
import { saveAnnotation, isAnnotationSaved, removeAnnotation } from '@/lib/storage'
import type { Annotation } from '@/lib/types'

interface AnnotationDrawerProps {
  isOpen: boolean
  onClose: () => void
  annotation: Annotation | null
}

export function AnnotationDrawer({ isOpen, onClose, annotation }: AnnotationDrawerProps) {
  const { drawerState, expandDrawer, collapseDrawer } = useDrawerHeight()
  const { handleDragEnd } = useDrawerGestures(expandDrawer, collapseDrawer)
  const [isSaved, setIsSaved] = useState(false)

  useEffect(() => {
    if (isOpen && annotation) {
      collapseDrawer()
    }
  }, [isOpen, annotation, collapseDrawer])

  useEffect(() => {
    const checkSavedStatus = () => {
      if (annotation?.id) {
        setIsSaved(isAnnotationSaved(annotation.id))
      } else {
        setIsSaved(false)
      }
    }

    checkSavedStatus()
  }, [annotation?.id])

  const handleSave = useCallback(() => {
    if (!annotation) {
      return
    }

    try {
      if (isSaved) {
        removeAnnotation(annotation.id)
        setIsSaved(false)
        toast('Annotation Removed', {
          description: 'The annotation has been removed from your saved items',
          duration: 3000,
        })
      } else {
        saveAnnotation(annotation)
        setIsSaved(true)
        toast.success('Annotation Saved', {
          description: 'Your annotation has been saved successfully',
          duration: 3000,
        })
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to save annotation'
      toast.error('Operation Failed', {
        description: errorMessage,
      })
    }
  }, [annotation, isSaved])

  const handleHeaderCollapse = useCallback(() => {
    if (drawerState === 'expanded') {
      collapseDrawer()
    }
  }, [drawerState, collapseDrawer])

  return (
    <Sheet open={isOpen} onOpenChange={(open: boolean) => !open && onClose()}>
      <SheetContent
        side="bottom"
        className="
          overflow-hidden rounded-t-2xl border-t border-gray-200 bg-white p-6
        "
        showCloseButton={false}
        style={{
          height: drawerState === 'closed' ? '0%' : drawerState === 'collapsed' ? '35vh' : '75vh',
          transition: 'height 300ms cubic-bezier(0.4, 0, 0.2, 1)',
        }}
        aria-label="Annotation drawer"
        role="dialog"
        aria-modal="true"
      >
        <div className="flex h-full flex-col">
          <motion.div 
            className="
              -mt-4 flex cursor-grab justify-center pb-2
              active:cursor-grabbing
            "
            role="slider"
            aria-label="Drawer resize handle"
            aria-valuemin={0}
            aria-valuemax={2}
            aria-valuenow={drawerState === 'collapsed' ? 1 : drawerState === 'expanded' ? 2 : 0}
            tabIndex={0}
            drag="y"
            dragConstraints={{ top: 0, bottom: 0 }}
            dragElastic={0.2}
            onDragEnd={(_, info) => handleDragEnd(_, info)}
          >
            <div className="h-1.5 w-12 rounded-full bg-gray-300" />
          </motion.div>

          <DrawerHeader onClose={onClose} onCollapse={handleHeaderCollapse} />

          <div className="h-6" />

          {annotation && (
            <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
              <AnnotationContent
                key={drawerState}
                annotation={annotation}
                drawerState={drawerState}
              />
              
              <div className="h-4 shrink-0" />
              
              <Button
                onClick={handleSave}
                className={`
                  h-12 w-full shrink-0 text-base font-medium
                  ${
                  isSaved
                    ? `
                      border border-green-200 bg-green-50 text-green-900
                      hover:bg-green-100
                    `
                    : 'bg-[#0F172A] text-white'
                }
                `}
                size="lg"
                disabled={!annotation}
              >
                {isSaved ? (
                  <>
                    <BookmarkCheck className="mr-2 size-5" />
                    Saved
                  </>
                ) : (
                  <>
                    <Bookmark className="mr-2 size-5" />
                    Save Annotation
                  </>
                )}
              </Button>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
