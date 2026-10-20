import { describe, expect, it } from 'vitest'
import { version } from 'pdfjs-dist'
import { getPdfDocumentLoadOptions } from '../initPdfWorker'

describe('PDF document load options', () => {
  it('loads character maps from the installed PDF.js release', () => {
    const options = getPdfDocumentLoadOptions('/document.pdf')
    const characterMapURL = new URL(options.cMapUrl)

    expect(characterMapURL.pathname.split('/')[1]).toBe(`pdfjs-dist@${version}`)
    expect(options.url).toBe('/document.pdf')
    expect(options.cMapPacked).toBe(true)
  })
})
