import { TextLayer } from 'pdfjs-dist'
import type { PDFDocumentProxy } from 'pdfjs-dist'

export interface TextLayerInstanceRef {
  current: TextLayer | null
}

export interface RenderPdfPageMetrics {
  width: number
  height: number
  scale: number
}

export async function renderPdfPageToCanvas(params: {
  pdfDoc: PDFDocumentProxy
  pageNumber: number
  containerWidth: number
  canvas: HTMLCanvasElement
  textLayerDiv: HTMLDivElement
  textLayerInstanceRef: TextLayerInstanceRef
}): Promise<RenderPdfPageMetrics> {
  const { pdfDoc, pageNumber, containerWidth, canvas, textLayerDiv, textLayerInstanceRef } = params

  const page = await pdfDoc.getPage(pageNumber)
  const viewport = page.getViewport({ scale: 1 })
  const scale = containerWidth / viewport.width
  const scaledViewport = page.getViewport({ scale })

  canvas.height = scaledViewport.height
  canvas.width = scaledViewport.width

  await page.render({
    canvas,
    viewport: scaledViewport,
  }).promise

  if (textLayerInstanceRef.current) {
    textLayerInstanceRef.current.cancel()
    textLayerInstanceRef.current = null
  }

  const textContent = await page.getTextContent()
  textLayerDiv.innerHTML = ''
  textLayerDiv.style.setProperty('--scale-factor', `${scale}`)
  textLayerDiv.style.setProperty('--total-scale-factor', `${scale}`)
  textLayerDiv.style.setProperty('--user-unit', '1')

  const textLayer = new TextLayer({
    textContentSource: textContent,
    container: textLayerDiv,
    viewport: scaledViewport,
  })
  textLayerInstanceRef.current = textLayer
  await textLayer.render()

  return {
    width: scaledViewport.width,
    height: scaledViewport.height,
    scale,
  }
}
