import { describe, it, expect } from 'vitest'
import { cn } from '../utils'

describe('cn', () => {
  /* eslint-disable better-tailwindcss/no-unknown-classes */
  it('should merge class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar')
  })

  it('should handle conditional classes', () => {
    const condition = false
    expect(cn('foo', condition && 'bar', 'baz')).toBe('foo baz')
  })
  /* eslint-enable better-tailwindcss/no-unknown-classes */

  it('should merge tailwind classes correctly', () => {
    expect(cn('px-2 py-1', 'px-4')).toBe('py-1 px-4')
  })

  /* eslint-disable better-tailwindcss/no-unknown-classes */
  it('should handle empty strings', () => {
    expect(cn('foo', '', 'bar')).toBe('foo bar')
  })

  it('should handle undefined and null', () => {
    expect(cn('foo', undefined, null, 'bar')).toBe('foo bar')
  })
  /* eslint-enable better-tailwindcss/no-unknown-classes */
})
