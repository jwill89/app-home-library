import { expect, test } from '@playwright/test'
import type { BookInput } from '../src/types'
import { addBook } from './library'

// The Stacks: every book a spine on bookcases drawn on the page. Run after the shelves' tests,
// which leave a bookcase with an area of nonfiction and a layout in use.

// A title of its own each run, so a retry's second copy of the book is not the first's.
const pullTest = {
  title: `Stacks Pull ${String(Date.now())}`,
  contributors: [{ name: 'Stacks Author', role: 'author' as const }],
  series: 'Stacks Series',
  seriesNumber: '2',
  category: 'fiction',
  ddc: '823.7',
  heightMm: 198,
  thicknessMm: 31,
} satisfies Partial<BookInput>

test('a book is found on the shelves, pulled off and put back by the keyboard alone', async ({
  page,
  request,
}) => {
  const id = await addBook(request, pullTest)
  await page.goto('/stacks')
  await expect(page.getByRole('heading', { name: 'Fiction', exact: true })).toBeVisible()

  // Finding it marks it and goes to it; Enter pulls it off the shelf.
  await page.getByRole('searchbox', { name: 'Find on the shelves' }).fill(pullTest.title)
  await page.keyboard.press('Enter')
  await expect(page.getByRole('status').filter({ hasText: '1 book found' })).toBeVisible()
  const spine = page.getByRole('button', {
    name: `${pullTest.title}, Stacks Series #2, by Stacks Author`,
  })
  await expect(spine).toBeFocused()
  await page.keyboard.press('Enter')
  const pulled = page.getByRole('dialog', { name: pullTest.title })
  await expect(pulled.getByRole('img', { name: `No cover for ${pullTest.title}` })).toBeVisible()
  await expect(pulled.getByText('823.7 AUT v.2')).toBeVisible()
  await expect(pulled.getByText('Stacks Series #2')).toBeVisible()

  // Put back, the focus is on its spine again, where the reader was.
  await page.keyboard.press('Escape')
  await expect(pulled).toBeHidden()
  await expect(spine).toBeFocused()

  // Pulled off again, the dialog takes the focus, and Tab goes round it to the way to its page.
  await page.keyboard.press('Enter')
  await expect(pulled).toBeVisible()
  await expect.poll(() => pulled.evaluate((d) => d.contains(document.activeElement))).toBe(true)
  const open = pulled.getByRole('button', { name: 'Open its page' })
  await expect(async () => {
    await page.keyboard.press('Tab')
    await expect(open).toBeFocused({ timeout: 250 })
  }).toPass()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(new RegExp(`/books/${String(id)}$`))
})

test('a spine moves at once where the device asks for less motion', async ({ page, request }) => {
  await addBook(request, { ...pullTest, title: 'Stacks Still Test' })
  await page.goto('/stacks')
  const spine = page.getByRole('button', { name: /^Stacks Still Test,/ }).first()
  const moving = () => spine.evaluate((el) => getComputedStyle(el).transitionDuration)
  expect(await moving()).toBe('0.2s')
  await page.emulateMedia({ reducedMotion: 'reduce' })
  expect(await moving()).toBe('0s')
})

test('every control on the shelves is at least 44 pixels each way', async ({ page }) => {
  for (const viewport of [
    { width: 390, height: 844 },
    { width: 1280, height: 900 },
  ]) {
    await page.setViewportSize(viewport)
    await page.goto('/stacks')
    await expect(page.getByRole('heading', { name: 'Fiction', exact: true })).toBeVisible()
    const small = await page
      .getByRole('main')
      .getByRole('button')
      .evaluateAll((buttons) =>
        buttons
          .map((b) => ({
            name: b.getAttribute('aria-label') ?? b.textContent,
            box: b.getBoundingClientRect(),
          }))
          .filter(({ box }) => box.width < 44 || box.height < 44)
          .map(({ name }) => name),
      )
    expect(small).toEqual([])
  }
})

test('the stacks stand by call number and by area, each shelf named for what it holds', async ({
  page,
}) => {
  await page.goto('/stacks')
  await page.getByRole('combobox', { name: 'Arrange by' }).click()
  await page.getByRole('option', { name: 'Call number' }).click()
  const every = page.getByRole('region', { name: 'Every book' })
  await expect(every.getByRole('list', { name: /^Shelf 1: / })).toBeVisible()

  // The shelves' tests made an area of nonfiction, which takes their book.
  await page.getByRole('combobox', { name: 'Arrange by' }).click()
  await page.getByRole('option', { name: 'Area' }).click()
  const area = page.getByRole('region', { name: 'Nonfiction' })
  await expect(area.getByRole('button', { name: /^Shelf Area Test/ })).toBeVisible()
  await expect(page.getByRole('region', { name: 'In no area' })).toBeVisible()

  // The device keeps the choice.
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Arrange by' })).toHaveText('Area')
})

test('my bookcases are drawn as the layout in use fills them, or as a fresh suggestion', async ({
  page,
}) => {
  await page.goto('/stacks')
  await page.getByRole('combobox', { name: 'Arrange by' }).click()
  await page.getByRole('option', { name: 'My bookcases' }).click()
  await expect(page.getByText(/^The layout put to use/)).toBeVisible()
  const left = page.getByRole('region', { name: 'Living room, left' })
  // Its books stand on one shelf of the area or the other, as each shelf's start has them.
  await expect(
    left.getByRole('img', { name: /^Shelf [12], Nonfiction, \d+ books?, \d+% full/ }).first(),
  ).toBeVisible()

  await page.getByRole('button', { name: 'A fresh suggestion' }).click()
  await expect(page.getByRole('button', { name: 'A fresh suggestion' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(page.getByText(/Nothing is put to use from here/)).toBeVisible()
  await expect(left.getByRole('img', { name: /^Shelf 1, Nonfiction/ })).toBeVisible()
})

test('a visitor browses the stacks, without the areas or the owner’s bookcases', async ({
  browser,
  request,
}) => {
  await addBook(request, { title: 'Stacks Visitor Test', category: 'fiction' })
  const made = await request.post('/api/v1/shares', {
    data: {
      name: 'Stacks',
      expiresAt: '',
      scope: {
        books: 'all',
        categories: [],
        bookIds: [],
        wishlist: false,
        papers: 'none',
        courseIds: [],
        paperIds: [],
      },
    },
  })
  const { token } = (await made.json()) as { token: string }
  const visitor = await browser.newContext({ storageState: { cookies: [], origins: [] } })
  const page = await visitor.newPage()
  await page.goto(`/s/${token}`)
  await page
    .getByRole('navigation', { name: 'Library' })
    .getByRole('link', { name: 'Stacks' })
    .click()
  await expect(page.getByRole('button', { name: /^Stacks Visitor Test/ }).first()).toBeVisible()
  await page.getByRole('combobox', { name: 'Arrange by' }).click()
  await expect(page.getByRole('option')).toHaveText(['Category', 'Call number', 'Title', 'Author'])
  await visitor.close()
})
