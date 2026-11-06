import type { BarcodeFormat, DetectedBarcode } from 'barcode-detector/ponyfill'

// The barcodes a book carries: its ISBN as EAN-13, a mass-market paperback's UPC on the back,
// and the library's own Code 128 labels.
const FORMATS: BarcodeFormat[] = ['ean_13', 'ean_8', 'upc_a', 'upc_e', 'code_128']

/** What a scan came to, as the camera's banner and the scan toast say it. */
export interface ScanNote {
  text: string
  tone: 'good' | 'info' | 'bad'
  /** When it was said, so the same words said again still show as new. */
  at: number
}

export function scanNote(text: string, tone: ScanNote['tone']): ScanNote {
  return { text, tone, at: Date.now() }
}

export interface Detector {
  detect(source: HTMLVideoElement | HTMLCanvasElement | ImageBitmap): Promise<DetectedBarcode[]>
}

interface DetectorClass {
  new (options: { formats: BarcodeFormat[] }): Detector
  getSupportedFormats(): Promise<BarcodeFormat[]>
}

/**
 * The browser's own barcode reader where it reads EAN-13 (Chrome and its kin on Android), and
 * zxing-wasm otherwise, loaded only then and from this site.
 */
export async function createDetector(): Promise<Detector> {
  const native = (globalThis as { BarcodeDetector?: DetectorClass }).BarcodeDetector
  if (native) {
    try {
      const supported = await native.getSupportedFormats()
      if (supported.includes('ean_13')) {
        return new native({ formats: FORMATS.filter((f) => supported.includes(f)) })
      }
    } catch {
      // A reader that cannot say what it reads is passed over for zxing.
    }
  }
  const [{ BarcodeDetector, prepareZXingModule }, { default: wasm }] = await Promise.all([
    import('barcode-detector/ponyfill'),
    import('zxing-wasm/reader/zxing_reader.wasm?url'),
  ])
  prepareZXingModule({
    overrides: {
      locateFile: (path: string, prefix: string) => (path.endsWith('.wasm') ? wasm : prefix + path),
    },
  })
  return new BarcodeDetector({ formats: FORMATS })
}

/** Why the camera could not start, in words the owner can act on. */
export function cameraProblem(error: unknown): string {
  const name = error instanceof DOMException ? error.name : ''
  switch (name) {
    case 'NotAllowedError':
    case 'SecurityError':
      return "The camera was not allowed. Allow it for this site in the browser's settings, or type the ISBN."
    case 'NotFoundError':
    case 'OverconstrainedError':
      return 'No camera was found. Type the ISBN instead.'
    case 'NotReadableError':
      return 'Another app is using the camera. Close it and try again.'
  }
  return 'The camera could not start. Type the ISBN instead.'
}
