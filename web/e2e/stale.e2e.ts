import { expect, test, type Page } from '@playwright/test'
import { addBook } from './library'

const title = (page: Page) => page.getByLabel(/^Title/)
const question = (page: Page) =>
  page.getByRole('alertdialog', { name: 'Changed since you opened it' })

async function saveTitle(page: Page, words: string) {
  await title(page).fill(words)
  await page.getByRole('button', { name: 'Save', exact: true }).click()
}

// One book open for editing on two pages: the page that saves second is told the book changed
// since it opened, and chooses between the latest and its own.
test('a book saved on another page first asks which to keep', async ({
  context,
  page,
  request,
}) => {
  const id = await addBook(request, { title: 'Two Pages' })
  const other = await context.newPage()
  await page.goto(`/books/${id}/edit`)
  await other.goto(`/books/${id}/edit`)
  await expect(title(page)).toHaveValue('Two Pages')
  await expect(title(other)).toHaveValue('Two Pages')

  await saveTitle(page, 'Two Pages, First')
  await expect(page.getByRole('heading', { level: 1, name: 'Two Pages, First' })).toBeVisible()

  // Loading the latest drops what was typed here for what the first page saved.
  await saveTitle(other, 'Two Pages, Second')
  await expect(question(other)).toContainText(
    'This book was changed on another device or page since you opened it.',
  )
  await question(other).getByRole('button', { name: 'Load the latest' }).click()
  await expect(question(other)).toBeHidden()
  await expect(title(other)).toHaveValue('Two Pages, First')

  // Changed again on the first page, then saved over from the second.
  await page.goto(`/books/${id}/edit`)
  await saveTitle(page, 'Two Pages, Third')
  await expect(page.getByRole('heading', { level: 1, name: 'Two Pages, Third' })).toBeVisible()
  await saveTitle(other, 'Two Pages, Mine')
  await question(other).getByRole('button', { name: 'Save mine over it' }).click()
  await expect(other.getByRole('heading', { level: 1, name: 'Two Pages, Mine' })).toBeVisible()

  // The second save took the stamp it answered with, so the book is as the second page left it.
  const saved = (await (await request.get(`/api/v1/books/${String(id)}`)).json()) as {
    title: string
  }
  expect(saved.title).toBe('Two Pages, Mine')
})
