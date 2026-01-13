interface HighlightedTextSectionProps {
  text: string
}

export function HighlightedTextSection({ text }: HighlightedTextSectionProps) {
  return (
    <div className="flex flex-col gap-4 rounded-lg bg-[#EFF6FF] p-3">
      <span className="text-sm text-gray-600 italic">Highlighted:</span>
      <span className="text-sm/relaxed font-normal text-gray-900">{text}</span>
    </div>
  )
}
