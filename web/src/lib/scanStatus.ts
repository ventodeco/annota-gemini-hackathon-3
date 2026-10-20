type ScanFailureCopy = {
  title: string
  description: string
}

const GENERIC_FAILURE_COPY: ScanFailureCopy = {
  title: "We couldn't read this page",
  description: 'Please try again, or use a clearer photo.',
}

export const NO_TEXT_SCAN_COPY: ScanFailureCopy = {
  title: 'No text found in this image',
  description: 'Try a sharper, well-lit photo with the page filling the frame.',
}

const SCAN_FAILURE_COPY = new Map<string, ScanFailureCopy>([
  ['no_text_found', NO_TEXT_SCAN_COPY],
  ['ocr_timeout', {
    title: 'Reading this page took too long',
    description: 'The text service was slow to respond. Please try the scan again.',
  }],
  ['ocr_interrupted', {
    title: 'This scan was interrupted',
    description: 'Processing stopped before it finished. Please upload the page again.',
  }],
  ['ocr_failed', GENERIC_FAILURE_COPY],
])

/** User-facing copy for a scan's failureReason code. Unknown codes fall back to a generic message. */
export function getScanFailureCopy(failureReason: string | undefined): ScanFailureCopy {
  return (failureReason && SCAN_FAILURE_COPY.get(failureReason)) || GENERIC_FAILURE_COPY
}
