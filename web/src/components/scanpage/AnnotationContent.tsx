import { useEffect, useRef } from 'react'
import type { Annotation } from '@/lib/types'
import { HighlightedTextSection } from './HighlightedTextSection'
import { ScrollArea } from '@/components/ui/scroll-area'

interface AnnotationContentProps {
  annotation: Annotation
  drawerState?: 'collapsed' | 'expanded' | 'closed'
}

export function AnnotationContent({ annotation, drawerState }: AnnotationContentProps) {
  const scrollAreaRef = useRef<HTMLDivElement>(null)
  const renderWordBreakdown = (breakdown: string) => {
    const items = breakdown.split('\n').filter((line) => line.trim())
    return (
      <ul className="list-inside list-disc space-y-2">
        {items.map((item, index) => (
          <li key={index} className="text-base/relaxed text-gray-900">
            {item.replace(/^•\s*/, '')}
          </li>
        ))}
      </ul>
    )
  }

  const renderSection = (title: string, content: string, isBullets = false) => (
    <div className="flex flex-col gap-4">
      <h3 className="text-base/6 font-semibold text-black">{title}</h3>
      {isBullets ? (
        renderWordBreakdown(content)
      ) : (
        <p className="text-base/relaxed font-normal text-gray-900">{content}</p>
      )}
    </div>
  )

  useEffect(() => {
    if (scrollAreaRef.current) {
      scrollAreaRef.current.scrollTop = 0
    }
  }, [annotation, drawerState])

  return (
    <ScrollArea
      ref={scrollAreaRef}
      className="min-h-0 flex-1 overscroll-contain pr-2"
      style={{ overscrollBehavior: 'contain' }}
    >
      <div className="flex flex-col gap-6">
        <HighlightedTextSection text={annotation.selectedText} />
        {renderSection('Context', annotation.context)}
        {renderSection('Meaning', annotation.meaning)}
        {renderSection('Usage Example', annotation.usageExample)}
        {renderSection('Usage Timing', annotation.whenToUse)}
        {renderSection('Word Breakdown', annotation.wordBreakdown, true)}
        {renderSection('Alternative Meaning', annotation.alternativeMeanings)}
      </div>
    </ScrollArea>
  )
}
