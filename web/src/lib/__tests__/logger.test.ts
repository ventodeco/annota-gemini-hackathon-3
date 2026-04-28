import { describe, expect, it } from 'vitest'
import { resolveLogLevel } from '../logger'

describe('resolveLogLevel', () => {
  it('keeps supported log levels', () => {
    expect(resolveLogLevel('debug')).toBe('debug')
    expect(resolveLogLevel('info')).toBe('info')
    expect(resolveLogLevel('warn')).toBe('warn')
    expect(resolveLogLevel('error')).toBe('error')
  })

  it('falls back to info for unsupported values', () => {
    expect(resolveLogLevel('verbose')).toBe('info')
    expect(resolveLogLevel(undefined)).toBe('info')
  })
})
