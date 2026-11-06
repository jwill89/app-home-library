import { useToast } from 'primevue/usetoast'
import type { ErrorModel } from '@/api/client'
import type { ScanNote } from './barcodes'

/** What to tell the owner about a failed request: the server's own words where it has them. */
export function messageOf(error: unknown): string {
  if (error && typeof error === 'object') {
    const p = error as ErrorModel
    const field = p.errors?.find((e) => e.message)?.message
    if (p.detail && p.detail !== 'validation failed') return p.detail
    if (field) return field
    if (p.title) return p.title
  }
  if (error instanceof Error) return error.message
  return 'Something went wrong. Try again.'
}

const severity = { good: 'success', info: 'info', bad: 'error' } as const

/** A toast for a failure, one for a success, and one for each scan. */
export function useNotice() {
  const toast = useToast()
  return {
    /**
     * What a scan came to, near the top of the screen where the camera is. Only the latest
     * shows: scanning a shelf quickly must not stack them up.
     */
    scanned(note: ScanNote) {
      toast.removeGroup('scan')
      // A problem stays long enough to read what to do about it.
      const life = note.tone === 'bad' ? 6000 : 2500
      toast.add({ group: 'scan', severity: severity[note.tone], summary: note.text, life })
    },
    failed(error: unknown) {
      // Signed out or a link ended: the router has moved to the page that says so.
      if (error && typeof error === 'object' && (error as ErrorModel).status === 401) return
      toast.add({ severity: 'error', summary: messageOf(error), life: 6000 })
    },
    done(summary: string) {
      toast.add({ severity: 'success', summary, life: 3000 })
    },
  }
}
