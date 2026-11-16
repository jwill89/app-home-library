import { describe, expect, it } from 'vitest'
import type { ErrorModel } from '@/types'
import { messageOf, ProblemError } from '@/app/problem'

const problem = (fields: Partial<ErrorModel>): ErrorModel => ({ type: 'about:blank', ...fields })

describe('messageOf', () => {
  it("tells the server's own detail first", () => {
    expect(
      messageOf(problem({ title: 'Conflict', detail: 'That ISBN is already a book here.' })),
    ).toBe('That ISBN is already a book here.')
  })

  it('tells the first field that says what is wrong when the detail only says validation failed', () => {
    const answer = problem({
      title: 'Unprocessable Entity',
      detail: 'validation failed',
      errors: [{ location: 'body.title' }, { location: 'body.isbn', message: 'Not an ISBN.' }],
    })
    expect(messageOf(answer)).toBe('Not an ISBN.')
  })

  it('tells the title when there is nothing more', () => {
    expect(messageOf(problem({ title: 'Not Found', detail: 'validation failed' }))).toBe(
      'Not Found',
    )
    expect(messageOf(problem({ title: 'Not Found', errors: null }))).toBe('Not Found')
  })

  it("tells a thrown error's message", () => {
    expect(messageOf(new TypeError('Failed to fetch'))).toBe('Failed to fetch')
  })

  it('falls back to words of its own for anything else', () => {
    expect(messageOf(undefined)).toBe('Something went wrong. Try again.')
    expect(messageOf('a string')).toBe('Something went wrong. Try again.')
    expect(messageOf(problem({}))).toBe('Something went wrong. Try again.')
  })
})

describe('ProblemError', () => {
  it("keeps the answer's status and says what messageOf says", () => {
    const error = new ProblemError(problem({ status: 401, title: 'Unauthorized' }))
    expect(error).toBeInstanceOf(Error)
    expect(error.name).toBe('ProblemError')
    expect(error.message).toBe('Unauthorized')
    expect(error.status).toBe(401)
  })

  it('has no status when the answer gave none', () => {
    expect(new ProblemError(problem({ detail: 'No connection.' })).status).toBeUndefined()
  })
})
