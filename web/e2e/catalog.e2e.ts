import { expect, test } from '@playwright/test'
import { addBook } from './library'

test('a book added by hand is in the library, on its own page', async ({ page }) => {
  await page.goto('/add')
  await page.getByRole('button', { name: 'By hand' }).click()
  await page.getByRole('button', { name: 'Start the form' }).click()
  await page.getByLabel(/^Title/).fill('The Name of the Wind')
  await page.getByRole('button', { name: 'Save the book' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'The Name of the Wind' })).toBeVisible()
  await page.goto('/')
  await expect(page.getByText('The Name of the Wind').first()).toBeVisible()
})

test('a copy lent, then back', async ({ page, request }) => {
  const id = await addBook(request, { title: 'A Lent Book' })
  await page.goto(`/books/${id}`)
  await page.getByRole('button', { name: /^Lend copy/ }).click()
  await page.getByLabel('Lent to').fill('Sam')
  await page.getByRole('button', { name: 'Lend it' }).click()
  await expect(page.getByText(/Lent to Sam on/)).toBeVisible()
  await page.getByRole('button', { name: /is back: mark it returned today/ }).click()
  await expect(page.getByRole('button', { name: /^Lend copy/ })).toBeVisible()
})

test('a wanted book waits on the wishlist until it is got', async ({ page, request }) => {
  await addBook(request, { title: 'Something Wanted' }, true)
  await page.goto('/wishlist')
  await expect(page.getByRole('link', { name: 'Something Wanted' })).toBeVisible()
  await page.getByRole('button', { name: /Got Something Wanted/ }).click()
  await expect(page.getByRole('link', { name: 'Something Wanted' })).toBeHidden()
  await page.goto('/')
  await expect(page.getByText('Something Wanted').first()).toBeVisible()
})

test('a copy that has gone keeps its record, and comes back', async ({ page, request }) => {
  const id = await addBook(request, { title: 'Given Away' })
  await page.goto(`/books/${id}`)
  await page.getByRole('button', { name: /has left the library$/ }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('How it went').click()
  await page.getByRole('option', { name: 'Donated' }).click()
  await dialog.getByLabel('Note').fill('To the school library')
  await dialog.getByRole('button', { name: 'Mark it gone' }).click()
  await expect(page.getByText(/Donated on .*: To the school library/)).toBeVisible()
  await page.getByRole('button', { name: /is back in the library$/ }).click()
  await expect(page.getByText(/Donated on/)).toBeHidden()
})

test('a series says which volume is missing', async ({ page, request }) => {
  await addBook(request, { title: 'Run, the first', series: 'A Test Run', seriesNumber: '1' })
  await addBook(request, { title: 'Run, the third', series: 'A Test Run', seriesNumber: '3' })
  await page.goto('/series')
  const row = page.getByRole('row', { name: /A Test Run/ })
  await expect(row).toContainText('2')
  await expect(row).toContainText('4')
})

test('a book read again keeps its earlier read', async ({ page, request }) => {
  const id = await addBook(request, {
    title: 'Read Twice',
    readStatus: 'read',
    startedOn: '2025-04-01',
    finishedOn: '2025-05-01',
  })
  await page.goto(`/books/${id}`)
  await page.getByRole('button', { name: 'Read it again' }).click()
  await expect(page.getByRole('heading', { name: 'Earlier reads' })).toBeVisible()
  await expect(page.getByText('May 1, 2025')).toBeVisible()
  // Being read again, and finished before: it is in both lists.
  await page.goto('/reading')
  await expect(page.getByRole('link', { name: 'Read Twice' })).toHaveCount(2)
})

test('the details a book lacks are listed', async ({ page, request }) => {
  await addBook(request, { title: 'Thin Record' })
  await page.goto('/gaps')
  await expect(page.getByRole('heading', { level: 1, name: 'Missing details' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Thin Record' })).toBeVisible()
})

test('a book marked read from the library list, without opening it', async ({ page, request }) => {
  await addBook(request, { title: 'Marked From the List' })
  await page.goto('/')
  await page.getByLabel('Reading status of Marked From the List').click()
  await page.getByRole('option', { name: 'Read', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
  await page.reload()
  await expect(page.getByLabel('Reading status of Marked From the List')).toHaveText('Read')
})

test('a whole series marked read, with no day made up', async ({ page, request }) => {
  const ids = [
    await addBook(request, { title: 'Saga, the first', series: 'A Read Saga', seriesNumber: '1' }),
    await addBook(request, { title: 'Saga, the second', series: 'A Read Saga', seriesNumber: '2' }),
  ]
  await page.goto('/series')
  await page.getByRole('link', { name: 'A Read Saga', exact: true }).click()
  await page.getByRole('button', { name: 'Mark read' }).click()
  await page.getByRole('button', { name: 'Mark them read' }).click()
  await expect(page.getByText('Marked 2 books of A Read Saga read')).toBeVisible()
  for (const id of ids) {
    const book = (await (await request.get(`/api/v1/books/${id}`)).json()) as {
      readStatus: string
      finishedOn: string
    }
    expect(book).toMatchObject({ readStatus: 'read', finishedOn: '' })
  }
})

test('going back to the library lands on the page it was on', async ({ page, request }) => {
  for (let i = 1; i <= 55; i++) {
    await addBook(request, { title: `Paged ${String(i).padStart(3, '0')}` })
  }
  await page.goto('/')
  await page.getByPlaceholder(/^Title, author/).fill('Paged')
  await page.getByRole('button', { name: 'Page 2' }).click()
  await page.getByRole('link', { name: 'Paged 051' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Paged 051' })).toBeVisible()
  await page.goBack()
  await expect(page.getByRole('link', { name: 'Paged 051' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Page 2' })).toHaveAttribute('aria-current', 'page')
})

test('a book edited from a series: Back goes to the list and the series, not the edit', async ({
  page,
  request,
}) => {
  await addBook(request, { title: 'Back Button Book', series: 'A Back Series', seriesNumber: '1' })
  await page.goto('/series')
  await page.getByRole('link', { name: 'A Back Series', exact: true }).click()
  await page.getByRole('link', { name: 'Back Button Book' }).click()
  await page.getByRole('button', { name: 'Edit', exact: true }).click()
  await page.getByLabel('Subtitle').fill('Now With a Subtitle')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Now With a Subtitle').first()).toBeVisible()
  await expect(page).toHaveURL(/\/books\/\d+$/)
  await page.goBack()
  await expect(page).toHaveURL(/\/series\/\d+$/)
  await expect(page.getByRole('link', { name: 'Back Button Book' })).toBeVisible()
  await page.goBack()
  await expect(page).toHaveURL(/\/series$/)
})

test('a book removed: Back does not lead to it', async ({ page, request }) => {
  const id = await addBook(request, { title: 'Soon Removed' })
  await page.goto('/')
  await page.getByPlaceholder(/^Title, author/).fill('Soon Removed')
  await page.getByRole('link', { name: 'Soon Removed' }).click()
  await page.getByRole('button', { name: 'Remove the book' }).click()
  await page.getByRole('button', { name: 'Remove the book' }).last().click()
  await expect(page).toHaveURL(/\/$/)
  await page.goBack()
  await expect(page).not.toHaveURL(new RegExp(`/books/${id}$`))
})

test('the trail over a book leads to its series and the library', async ({ page, request }) => {
  const id = await addBook(request, {
    title: 'Trail Book',
    series: 'A Trail Series',
    seriesNumber: '2',
  })
  await page.goto(`/books/${id}/edit`)
  const trail = page.getByRole('navigation', { name: 'Breadcrumb' })
  await expect(trail.getByRole('link')).toHaveText(['Library', 'A Trail Series', 'Trail Book'])
  await trail.getByRole('link', { name: 'A Trail Series' }).click()
  await expect(page).toHaveURL(/\/series\/\d+$/)
  await expect(page.getByRole('heading', { level: 1, name: 'A Trail Series' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Trail Book' })).toBeVisible()
})

test('a book taken out of its series stays out, and a series said to be none', async ({
  page,
  request,
}) => {
  await addBook(request, { title: 'Line One', series: 'A Publisher Line', seriesNumber: '1' })
  const out = await addBook(request, {
    title: 'Line Two',
    series: 'A Publisher Line',
    seriesNumber: '2',
  })
  await page.goto('/series')
  await page.getByRole('link', { name: 'A Publisher Line', exact: true }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'A Publisher Line' })).toBeVisible()
  await page.getByRole('button', { name: 'Not in this series: Line Two' }).click()
  await page.getByRole('button', { name: 'Take it out' }).click()
  await expect(page.getByRole('link', { name: 'Line Two' })).toBeHidden()
  await expect(page.getByRole('link', { name: 'Line One' })).toBeVisible()
  const book = (await (await request.get(`/api/v1/books/${out}`)).json()) as {
    series: string
    sources: Record<string, string>
  }
  expect(book.series).toBe('')
  expect(book.sources.series).toBe('owner')

  await page.getByRole('button', { name: 'Not a series' }).click()
  await page.getByRole('button', { name: 'It is not a series' }).click()
  await expect(page).toHaveURL(/\/series$/)
  await expect(page.getByRole('link', { name: 'A Publisher Line', exact: true })).toBeHidden()
  // A source's series put aside takes no book; the owner typing it makes it a series again.
  const fromSource = await addBook(request, {
    title: 'Line Three',
    series: 'A Publisher Line',
    sources: { series: 'openlibrary' },
  })
  const again = (await (await request.get(`/api/v1/books/${fromSource}`)).json()) as {
    series: string
  }
  expect(again.series).toBe('')
})

test('a wanted book had in another edition says so, and comes off the list', async ({
  page,
  request,
}) => {
  const author = [{ name: 'Ann Twiceprinted', role: 'author' as const }]
  const had = await addBook(request, { title: 'Two Printings', contributors: author })
  await addBook(request, { title: 'Two Printings: A Second Edition', contributors: author }, true)
  await addBook(
    request,
    { title: 'Two Printings', contributors: [{ name: 'Other Hand', role: 'author' as const }] },
    true,
  )
  await page.goto('/wishlist')
  await expect(page.getByRole('link', { name: 'another edition' })).toHaveAttribute(
    'href',
    `/books/${had}`,
  )
  await page.getByRole('button', { name: 'Remove the one you have' }).click()
  await page.getByRole('button', { name: 'Remove it' }).click()
  await expect(page.getByRole('link', { name: 'Two Printings: A Second Edition' })).toBeHidden()
  // Another author's book of the same title is still wanted, and the one had is still had.
  await expect(page.getByRole('link', { name: 'Two Printings', exact: true })).toBeVisible()
  await page.goto(`/books/${had}`)
  await expect(page.getByRole('heading', { level: 1, name: 'Two Printings' })).toBeVisible()
})

test('the library by category: worked out, chosen, and filtered', async ({ page, request }) => {
  const manga = await addBook(request, {
    title: 'A Category Manga',
    series: 'A Category Manga (Manga)',
    seriesNumber: '1',
  })
  await addBook(request, { title: 'A Category Novel', ddc: '823.92' })
  await page.goto('/')
  await page.getByRole('button', { name: /^Manga \d+$/ }).click()
  await expect(page.getByRole('button', { name: /^Manga \d+$/ })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(page.getByRole('link', { name: 'A Category Manga' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'A Category Novel' })).toBeHidden()
  await page.getByRole('button', { name: /^All \d+$/ }).click()

  // Chosen on the Edit page, it is kept whatever its details say.
  await page.goto(`/books/${manga}/edit`)
  await expect(page.getByText('From its class number, subjects, and publisher')).toBeVisible()
  await page.getByLabel('Category').click()
  await page.getByRole('option', { name: 'Light novel' }).click()
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page).toHaveURL(new RegExp(`/books/${manga}$`))
  await page.reload()
  const facts = page.locator('.detail-list')
  await expect(facts.getByText('Light novel')).toBeVisible()
  await expect(facts.getByText('from you')).toBeVisible()
})

test('a link to the documents of before goes to the papers', async ({ page }) => {
  await page.goto('/documents')
  await expect(page).toHaveURL(/\/papers$/)
  await expect(page.getByRole('heading', { level: 1, name: 'Papers' })).toBeVisible()
})

test('the library shows a change made elsewhere without being reloaded', async ({
  page,
  request,
}) => {
  const id = await addBook(request, { title: 'Changed Elsewhere' })
  await page.goto('/')
  const row = page.getByRole('row', { name: /Changed Elsewhere/ })
  await expect(row).toContainText('Unread')
  // Another device marks it begun, and adds a book: the open list hears of both.
  const res = await request.put(`/api/v1/books/${id}/read-status`, {
    data: { readStatus: 'reading', on: '2026-11-05' },
  })
  expect(res.ok(), await res.text()).toBe(true)
  await expect(row).toContainText('Reading')
  await addBook(request, { title: 'Added Elsewhere' })
  await expect(page.getByRole('row', { name: /Added Elsewhere/ })).toBeVisible()
})
