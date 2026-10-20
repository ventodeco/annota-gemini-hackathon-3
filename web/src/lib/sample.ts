/** Original CC0 sample shipped with the app; see public/samples/README.md. */
export const SAMPLE_PDF_URL = '/samples/annota-sample.pdf'
export const SAMPLE_PDF_FILENAME = 'annota-sample.pdf'

/** Loads the bundled sample as a File so it can go through the normal PDF upload. */
export async function fetchSamplePdf(): Promise<File> {
  const response = await fetch(SAMPLE_PDF_URL)
  if (!response.ok) {
    throw new Error(`Sample PDF request failed with ${response.status}`)
  }
  const blob = await response.blob()
  return new File([blob], SAMPLE_PDF_FILENAME, { type: 'application/pdf' })
}
