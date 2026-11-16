import type { ErrorModel } from '@/types'

/** What to tell the owner about a failed request: the server's own words where it has them. */
export function messageOf(error: unknown): string {
  if (error && typeof error === 'object') {
    const p = error as ErrorModel
    const field = p.errors?.find((e) => e.message)?.message
    if (p.detail && p.detail !== 'validation failed') {
      return p.detail
    }
    if (field) {
      return field
    }
    if (p.title) {
      return p.title
    }
  }
  if (error instanceof Error) {
    return error.message
  }
  return 'Something went wrong. Try again.'
}

/**
 * A failed answer as an error, for a step that throws rather than returns: its message is what
 * to tell the owner, and it keeps the answer's status, so a 401 is still told apart.
 */
export class ProblemError extends Error {
  readonly status: number | undefined

  constructor(problem: ErrorModel) {
    super(messageOf(problem))
    this.name = 'ProblemError'
    this.status = problem.status
  }
}
