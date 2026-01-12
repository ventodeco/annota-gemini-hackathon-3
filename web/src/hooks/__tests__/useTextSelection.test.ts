import { describe, it, expect } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useTextSelection } from '../useTextSelection'

describe('useTextSelection', () => {
  it('should initialize with empty selected text', () => {
    const { result } = renderHook(() => useTextSelection())

    expect(result.current.selectedText).toBe('')
  })

  it('should handle valid selection', () => {
    const { result } = renderHook(() => useTextSelection())

    act(() => {
      const res = result.current.handleSelection('test text')
      expect(res.valid).toBe(true)
    })

    expect(result.current.selectedText).toBe('test text')
  })

  it('should reject empty selection', () => {
    const { result } = renderHook(() => useTextSelection())

    act(() => {
      const res = result.current.handleSelection('   ')
      expect(res.valid).toBe(false)
      expect(res.error).toBe('Please select some text')
    })

    expect(result.current.selectedText).toBe('')
  })

  it('should reject text longer than 1000 characters', () => {
    const { result } = renderHook(() => useTextSelection())
    const longText = 'a'.repeat(1001)

    act(() => {
      const res = result.current.handleSelection(longText)
      expect(res.valid).toBe(false)
      expect(res.error).toContain('too long')
    })

    expect(result.current.selectedText).toBe('')
  })

  it('should clear selection', () => {
    const { result } = renderHook(() => useTextSelection())

    act(() => {
      result.current.handleSelection('test text')
    })

    expect(result.current.selectedText).toBe('test text')

    act(() => {
      result.current.clearSelection()
    })

    expect(result.current.selectedText).toBe('')
  })
})
