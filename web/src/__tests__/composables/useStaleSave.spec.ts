import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ConfirmationOptions } from 'primevue/confirmationoptions'
import { isStale, useStaleSave } from '@/composables/useStaleSave'
import type { ErrorModel } from '@/types'

// The confirmation and the toasts are PrimeVue's services, which a page provides; here each is
// a stand-in that keeps what it was asked.
const asked = vi.hoisted(() => ({ confirmations: [] as unknown[], toasts: [] as unknown[] }))
vi.mock('primevue/useconfirm', () => ({
  useConfirm: () => ({ require: (o: unknown) => asked.confirmations.push(o) }),
}))
vi.mock('primevue/usetoast', () => ({
  useToast: () => ({ add: (t: unknown) => asked.toasts.push(t), removeGroup: () => undefined }),
}))

const words =
  'This book was changed on another device or page since you opened it. Load the latest to see the change, or save yours over it.'
const refusal: ErrorModel = {
  type: 'about:blank',
  status: 409,
  title: 'Conflict',
  detail: words,
  errors: [{ location: 'body.updatedAt', message: words }],
}
const duplicate: ErrorModel = {
  type: 'about:blank',
  status: 409,
  title: 'Conflict',
  detail: 'There is already a bookcase named Hall.',
  errors: [{ location: 'body.name', message: 'There is already a bookcase named Hall.' }],
}

interface Kept {
  id: number
  title: string
  updatedAt?: string
}

/** The choices a form gives, each recording that it ran. */
function form(latest: { data?: Kept; error?: unknown } = {}) {
  return {
    latest: vi.fn(() => Promise.resolve(latest)),
    load: vi.fn<(r: Kept) => unknown>(),
    overwrite: vi.fn<(updatedAt: string | undefined) => unknown>(),
  }
}

const question = () => asked.confirmations.at(-1) as ConfirmationOptions
const settled = () => new Promise((resolve) => setTimeout(resolve, 0))

beforeEach(() => {
  asked.confirmations = []
  asked.toasts = []
})

describe('isStale', () => {
  it('knows a save refused because the record moved', () => {
    expect(isStale(refusal)).toBe(true)
  })

  it('takes nothing else for one', () => {
    expect(isStale(duplicate)).toBe(false)
    expect(isStale({ ...refusal, status: 422 })).toBe(false)
    expect(isStale({ type: 'about:blank', status: 409, errors: null })).toBe(false)
    expect(isStale(new Error('Failed to fetch'))).toBe(false)
    expect(isStale(undefined)).toBe(false)
  })
})

describe('useStaleSave', () => {
  it("asks what to do in the server's words, with the two choices", () => {
    const stale = useStaleSave()
    expect(stale(refusal, form())).toBe(true)
    expect(asked.confirmations).toHaveLength(1)
    expect(question()).toMatchObject({
      header: 'Changed since you opened it',
      message: words,
      rejectProps: { label: 'Load the latest' },
      acceptProps: { label: 'Save mine over it' },
      defaultFocus: 'reject',
    })
  })

  it('loads the latest into the form when the owner loads it', async () => {
    const latest = { id: 4, title: 'Middlemarch', updatedAt: '2026-11-16T10:00:05Z' }
    const choices = form({ data: latest })
    useStaleSave()(refusal, choices)
    question().reject?.()
    await settled()
    expect(choices.latest).toHaveBeenCalledOnce()
    expect(choices.load).toHaveBeenCalledWith(latest)
    expect(choices.overwrite).not.toHaveBeenCalled()
  })

  it("saves the form over the change with the latest record's stamp", async () => {
    const choices = form({
      data: { id: 4, title: 'Middlemarch', updatedAt: '2026-11-16T10:00:05Z' },
    })
    useStaleSave()(refusal, choices)
    question().accept?.()
    await settled()
    expect(choices.overwrite).toHaveBeenCalledWith('2026-11-16T10:00:05Z')
    expect(choices.load).not.toHaveBeenCalled()
  })

  it('says why when the latest cannot be fetched, and changes nothing', async () => {
    const choices = form({ error: { type: 'about:blank', status: 503, title: 'No connection' } })
    useStaleSave()(refusal, choices)
    question().accept?.()
    await settled()
    expect(choices.overwrite).not.toHaveBeenCalled()
    expect(asked.toasts).toEqual([
      expect.objectContaining({ severity: 'error', summary: 'No connection' }),
    ])
  })

  it('says so when the record was removed meanwhile', async () => {
    const choices = form({})
    useStaleSave()(refusal, choices)
    question().reject?.()
    await settled()
    expect(choices.load).not.toHaveBeenCalled()
    expect(asked.toasts).toEqual([
      expect.objectContaining({
        summary: 'It is no longer here: it was removed on another device or page.',
      }),
    ])
  })

  it('leaves any other refusal to the form, and asks nothing', () => {
    const choices = form()
    expect(useStaleSave()(duplicate, choices)).toBe(false)
    expect(useStaleSave()(new Error('Failed to fetch'), choices)).toBe(false)
    expect(asked.confirmations).toHaveLength(0)
    expect(choices.latest).not.toHaveBeenCalled()
  })
})
