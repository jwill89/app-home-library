import { describe, expect, it } from 'vitest'
import type { Role } from '@/api/client'
import { afterAnswer, type Asker } from './session'

function asker(role: Role): Asker & { forgotten: boolean } {
  return {
    role,
    isVisitor: role === 'visitor',
    forgotten: false,
    forget() {
      this.forgotten = true
    },
  }
}

describe('afterAnswer', () => {
  it('leaves the session as it was after a sign-in that failed', () => {
    for (const path of ['/auth/passkey/finish', '/auth/setup', '/auth/passkey/begin']) {
      const visitor = asker('visitor')
      expect(afterAnswer(visitor, 401, path)).toBeUndefined()
      expect(visitor.forgotten).toBe(false)
    }
  })

  it('sends a visitor whose link ended to the page that says so', () => {
    const visitor = asker('visitor')
    expect(afterAnswer(visitor, 401, '/books')).toBe('/link-ended')
    expect(visitor.forgotten).toBe(true)
  })

  it('sends the owner whose sign-in went stale to sign in', () => {
    const owner = asker('owner')
    expect(afterAnswer(owner, 401, '/passkeys/begin')).toBe('/sign-in')
    expect(owner.forgotten).toBe(true)
  })

  it('changes nothing for any other answer, or for no one signed in', () => {
    const owner = asker('owner')
    expect(afterAnswer(owner, 403, '/books')).toBeUndefined()
    expect(afterAnswer(owner, 200, '/me')).toBeUndefined()
    const none = asker('none')
    expect(afterAnswer(none, 401, '/books')).toBeUndefined()
    expect(owner.forgotten || none.forgotten).toBe(false)
  })
})
