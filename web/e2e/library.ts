import { expect, type APIRequestContext } from '@playwright/test'
import type { BookInput } from '../src/api/client'
import { emptyBook, emptyCopy } from '../src/app/book'

/** Adds a book through the API, as the owner, for a test to work on; it returns the book. */
export async function addBook(
  request: APIRequestContext,
  book: Partial<BookInput>,
  wanted = false,
): Promise<number> {
  const res = await request.post('/api/v1/books', {
    data: {
      book: { ...emptyBook(), contributors: [], ...book },
      copy: emptyCopy(),
      coverUrl: '',
      wanted,
    },
  })
  expect(res.status(), await res.text()).toBe(201)
  return ((await res.json()) as { id: number }).id
}
