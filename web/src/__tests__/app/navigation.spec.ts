import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Router } from 'vue-router'
import { goBack, returnTo } from '@/app/navigation'

// Vue Router keeps the page before in history.state.back; these set it as it would.
function cameFrom(back: unknown) {
  window.history.replaceState({ back }, '')
}

function fakeRouter() {
  const back = vi.fn()
  const replace = vi.fn(() => Promise.resolve())
  return { router: { back, replace } as unknown as Router, back, replace }
}

beforeEach(() => {
  window.history.replaceState(null, '')
})

describe('returnTo', () => {
  it('goes back when the page before is the record, its query and hash aside', () => {
    cameFrom('/books/12?tab=copies#loans')
    const { router, back, replace } = fakeRouter()
    returnTo(router, '/books/12')
    expect(back).toHaveBeenCalledOnce()
    expect(replace).not.toHaveBeenCalled()
  })

  it('puts the record in place of the edit when the page before is another', () => {
    cameFrom('/scan')
    const { router, back, replace } = fakeRouter()
    void returnTo(router, '/books/12')
    expect(replace).toHaveBeenCalledWith('/books/12')
    expect(back).not.toHaveBeenCalled()
  })

  it('puts the record in place of the edit when it is the first page the app showed', () => {
    cameFrom(null)
    const { router, replace } = fakeRouter()
    void returnTo(router, '/books/12')
    expect(replace).toHaveBeenCalledWith('/books/12')
  })

  it('takes no history state at all as the first page', () => {
    const { router, replace } = fakeRouter()
    void returnTo(router, '/papers/3')
    expect(replace).toHaveBeenCalledWith('/papers/3')
  })
})

describe('goBack', () => {
  it('goes back to the page before', () => {
    cameFrom('/loans')
    const { router, back, replace } = fakeRouter()
    goBack(router, '/')
    expect(back).toHaveBeenCalledOnce()
    expect(replace).not.toHaveBeenCalled()
  })

  it('goes to the fallback when the page before is the record being left, or one of its pages', () => {
    for (const before of ['/books/12', '/books/12/edit?from=scan']) {
      cameFrom(before)
      const { router, back, replace } = fakeRouter()
      void goBack(router, '/', '/books/12')
      expect(replace).toHaveBeenCalledWith('/')
      expect(back).not.toHaveBeenCalled()
    }
  })

  it('goes back past a record that only shares the start of its address', () => {
    cameFrom('/books/123')
    const { router, back } = fakeRouter()
    goBack(router, '/', '/books/12')
    expect(back).toHaveBeenCalledOnce()
  })

  it('goes to the fallback when this is the first page the app showed', () => {
    cameFrom(undefined)
    const { router, replace } = fakeRouter()
    void goBack(router, '/papers')
    expect(replace).toHaveBeenCalledWith('/papers')
  })
})
