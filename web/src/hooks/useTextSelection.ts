import { useState, useCallback } from 'react'

export function useTextSelection() {
  const [selectedText, setSelectedText] = useState<string>('')

  const handleSelection = useCallback((text: string) => {
    const trimmed = text.trim()
    if (trimmed.length === 0) {
      return { valid: false, error: 'Please select some text' }
    }
    if (trimmed.length > 1000) {
      return { valid: false, error: 'Selected text is too long (max 1000 characters)' }
    }
    setSelectedText(trimmed)
    return { valid: true }
  }, [])

  const clearSelection = useCallback(() => {
    setSelectedText('')
  }, [])

  return {
    selectedText,
    handleSelection,
    clearSelection,
  }
}
