import '@testing-library/jest-dom'
import { vi } from 'vitest'

Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
  },
  writable: true,
})

Object.defineProperty(window, 'sessionStorage', {
  value: {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
  },
  writable: true,
})

window.matchMedia = vi.fn().mockImplementation((query) => {
  return {
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }
})

window.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

// DOMMatrix polyfill for pdfjs-dist
class MockDOMMatrix {
  a = 1; b = 0; c = 0; d = 1; e = 0; f = 0
  m11 = 1; m12 = 0; m13 = 0; m14 = 0
  m21 = 0; m22 = 1; m23 = 0; m24 = 0
  m31 = 0; m32 = 0; m33 = 1; m34 = 0
  m41 = 0; m42 = 0; m43 = 0; m44 = 1
  is2D = true
  isIdentity = true
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  constructor(_init?: string) {}
  invertSelf(): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  multiplySelf(_other: MockDOMMatrix): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  preMultiplySelf(_other: MockDOMMatrix): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  translateSelf(_tx: number, _ty: number, _tz?: number): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  scaleSelf(_sx: number, _sy?: number, _sz?: number): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  rotateSelf(_r: number): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  rotateAxisAngleSelf(_x: number, _y: number, _z: number, _angle: number): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  skewXSelf(_sx: number): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  skewYSelf(_sy: number): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  perspectiveSelf(_p: number): MockDOMMatrix { return this }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  transformPoint(_point?: { x: number; y: number }): { x: number; y: number } { return { x: 0, y: 0 } }
  inverse(): MockDOMMatrix { return new MockDOMMatrix() }
  toFloat32Array(): Float32Array { return new Float32Array(16) }
  toFloat64Array(): Float64Array { return new Float64Array(16) }
  toJSON(): object { return {} }
  toString(): string { return '' }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  static fromFloat32Array(_array: Float32Array): MockDOMMatrix { return new MockDOMMatrix() }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  static fromFloat64Array(_array: Float64Array): MockDOMMatrix { return new MockDOMMatrix() }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  static fromMatrix(_matrix: MockDOMMatrix): MockDOMMatrix { return new MockDOMMatrix() }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
;(window as any).DOMMatrix = MockDOMMatrix
// eslint-disable-next-line @typescript-eslint/no-explicit-any
;(globalThis as any).DOMMatrix = MockDOMMatrix
