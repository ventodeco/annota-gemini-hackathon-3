import { useEffect, useCallback } from 'react'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { motion } from 'framer-motion'
import { useDrawerHeight, useDrawerGestures } from '@/hooks/useDrawerHeight'
import { DrawerHeader } from './DrawerHeader'
import { AnnotationContent } from './AnnotationContent'
import { Button } from '@/components/ui/button'
import { Bookmark, RotateCcw } from 'lucide-react'
import type { Annotation } from '@/lib/types'
import type { DrawerState } from '@/hooks/useDrawerHeight'

interface AnnotationDrawerProps {
  isOpen: boolean
  onClose: () => void
  annotation: Annotation | null
  onRegenerate?: () => void
  onSave?: () => void
  isRegenerating?: boolean
  isSaving?: boolean
  version?: number
  maxVersions?: number
}

function getDrawerHeight(drawerState: DrawerState): string {
  switch (drawerState) {
    case 'expanded':
      return '75vh'
    case 'closed':
    case 'collapsed':
      return '35vh'
  }
}

function getDrawerAriaValue(drawerState: DrawerState): number {
  switch (drawerState) {
    case 'expanded':
      return 2
    case 'collapsed':
      return 1
    case 'closed':
      return 0
  }
}

export function AnnotationDrawer({
  isOpen,
  onClose,
  annotation,
  onRegenerate,
  onSave,
  isRegenerating = false,
  isSaving = false,
  version = 1,
  maxVersions = 2,
}: AnnotationDrawerProps) {
  const { drawerState, expandDrawer, collapseDrawer } = useDrawerHeight()
  const { handleDragEnd } = useDrawerGestures(expandDrawer, collapseDrawer)

  useEffect(() => {
    if (isOpen && annotation) {
      collapseDrawer()
    }
  }, [isOpen, annotation, collapseDrawer])

  const handleSave = useCallback(() => {
    if (onSave) {
      onSave()
    }
  }, [onSave])

  const handleRegenerate = useCallback(() => {
    if (onRegenerate) {
      onRegenerate()
    }
  }, [onRegenerate])

  const handleHeaderCollapse = useCallback(() => {
    if (drawerState === 'expanded') {
      collapseDrawer()
    }
  }, [drawerState, collapseDrawer])

  const isRegenerateDisabled =
    !annotation || isRegenerating || version >= maxVersions

  return (
    <Sheet open={isOpen} onOpenChange={(open: boolean) => !open && onClose()}>
      <SheetContent
        side="bottom"
        className="bg-white border-t border-gray-200 rounded-t-2xl p-6 overflow-hidden"
        showCloseButton={false}
        style={{
          height: getDrawerHeight(drawerState),
          transition: 'height 300ms cubic-bezier(0.4, 0, 0.2, 1)',
        }}
        aria-label="Annotation drawer"
        role="dialog"
        aria-modal="true"
      >
        <div className="flex flex-col h-full">
          <motion.div 
            className="flex justify-center -mt-4 pb-2 cursor-grab active:cursor-grabbing"
            role="slider"
            aria-label="Drawer resize handle"
            aria-valuemin={0}
            aria-valuemax={2}
            aria-valuenow={getDrawerAriaValue(drawerState)}
            tabIndex={0}
            drag="y"
            dragConstraints={{ top: 0, bottom: 0 }}
            dragElastic={0.2}
            onDragEnd={(_, info) => handleDragEnd(_, info)}
          >
            <div className="w-12 h-1.5 bg-gray-300 rounded-full" />
          </motion.div>

          <DrawerHeader
            onClose={onClose}
            onCollapse={handleHeaderCollapse}
            version={version}
            maxVersions={maxVersions}
          />

          <div className="h-6" />

          {annotation && (
            <div className="flex-1 overflow-hidden flex flex-col min-h-0">
              <AnnotationContent
                key={drawerState}
                annotation={annotation}
                drawerState={drawerState}
              />
              
              <div className="h-4 shrink-0" />
              
              <div className="grid grid-cols-2 gap-4 shrink-0">
                <Button
                  onClick={handleRegenerate}
                  className="h-12 font-medium text-base bg-[#0F172A] text-white hover:bg-[#1E293B]"
                  size="lg"
                  disabled={isRegenerateDisabled}
                >
                  {isRegenerating ? (
                    <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin mr-2" />
                  ) : (
                    <RotateCcw className="w-5 h-5 mr-2" />
                  )}
                  {isRegenerating ? 'Regenerating...' : 'Regenerate'}
                </Button>

                <Button
                  onClick={handleSave}
                  className="h-12 font-medium text-base"
                  size="lg"
                  variant="secondary"
                  disabled={isSaving}
                >
                  {isSaving ? (
                    <div className="w-5 h-5 border-2 border-gray-500 border-t-transparent rounded-full animate-spin mr-2" />
                  ) : (
                    <Bookmark className="w-5 h-5 mr-2" />
                  )}
                  {isSaving ? 'Saving...' : 'Save Annotation'}
                </Button>
              </div>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
