import { describe, expect, it, vi } from 'vitest'
import { cameraProblem, scanNote } from '@/app/barcodes'

describe('cameraProblem', () => {
  const said = (name: string) => cameraProblem(new DOMException('', name))

  it('says to allow the camera when the browser or the site refused it', () => {
    for (const name of ['NotAllowedError', 'SecurityError']) {
      expect(said(name)).toBe(
        "The camera was not allowed. Allow it for this site in the browser's settings, or type the ISBN.",
      )
    }
  })

  it('says there is no camera when none answers what was asked', () => {
    for (const name of ['NotFoundError', 'OverconstrainedError']) {
      expect(said(name)).toBe('No camera was found. Type the ISBN instead.')
    }
  })

  it('says another app has the camera', () => {
    expect(said('NotReadableError')).toBe(
      'Another app is using the camera. Close it and try again.',
    )
  })

  it('says the camera could not start for anything else', () => {
    expect(said('AbortError')).toBe('The camera could not start. Type the ISBN instead.')
    expect(cameraProblem(new Error('NotAllowedError'))).toBe(
      'The camera could not start. Type the ISBN instead.',
    )
  })
})

describe('scanNote', () => {
  it('marks when it was said, so the same words again show as new', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000)
    expect(scanNote('Added to the library.', 'good')).toEqual({
      text: 'Added to the library.',
      tone: 'good',
      at: 1_000,
    })
  })
})
