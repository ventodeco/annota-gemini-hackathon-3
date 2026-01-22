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
      <ul className="list-disc list-inside space-y-2">
        {items.map((item, index) => (
          <li key={index} className="text-base text-gray-900 leading-relaxed">
            {item.replace(/^•\s*/, '')}
          </li>
        ))}
      </ul>
    )
  }

  const renderSection = (title: string, content: string, isBullets = false) => (
    <div className="flex flex-col gap-4">
      <h3 className="font-semibold text-base leading-6 text-black">{title}</h3>
      {isBullets ? (
        renderWordBreakdown(content)
      ) : (
        <p className="text-base font-normal text-gray-900 leading-relaxed">{content}</p>
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
      className="flex-1 pr-2 overscroll-contain min-h-0"
      style={{ overscrollBehavior: 'contain' }}
    >
      <div className="flex flex-col gap-6">
        <HighlightedTextSection text={annotation.highlighted_text} />
        {renderSection('Context', annotation.context_text || '')}
        {renderSection('Meaning', annotation.nuance_data.meaning)}
        {renderSection('Usage Example', annotation.nuance_data.usageExample)}
        {renderSection('Usage Timing', annotation.nuance_data.usageTiming)}
        {renderSection('Word Breakdown', annotation.nuance_data.wordBreakdown, true)}
        {renderSection('Alternative Meaning', annotation.nuance_data.alternativeMeaning)}
      </div>
    </ScrollArea>
  )
}
