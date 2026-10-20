import * as pdfjsLib from 'pdfjs-dist'

let workerConfigured = false

/** Idempotent: safe to call before any PDF.js load/render. */
export function ensurePdfjsWorker(): void {
  if (workerConfigured) return
  pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
    'pdfjs-dist/build/pdf.worker.min.mjs',
    import.meta.url,
  ).toString()
  workerConfigured = true
}

export function getPdfDocumentLoadOptions(url: string) {
  return {
    url,
    cMapUrl: `https://unpkg.com/pdfjs-dist@${pdfjsLib.version}/cmaps/`,
    cMapPacked: true as const,
  }
}
