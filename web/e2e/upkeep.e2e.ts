import { expect, test } from '@playwright/test'
import { addBook } from './library'

const goodreads = `Book Id,Title,Author,Author l-f,Additional Authors,ISBN,ISBN13,My Rating,Average Rating,Publisher,Binding,Number of Pages,Year Published,Original Publication Year,Date Read,Date Added,Bookshelves,Bookshelves with positions,Exclusive Shelf,My Review,Spoiler,Private Notes,Read Count,Owned Copies
101,Imported and Owned,Some Author,"Author, Some",,"=""""","=""""",4,4,Press,Paperback,300,2020,2020,2024/01/02,2024/01/01,owned,owned (#1),read,,,,1,1
102,Imported and Wanted,Other Author,"Author, Other",,"=""""","=""9780306406157""",0,4,Press,Hardcover,300,2020,2020,,2024/01/01,to-read,to-read (#1),to-read,,,,0,0
`

test('a Goodreads export: the owned book comes in, the wanted one goes on the wishlist', async ({
  page,
}) => {
  await page.goto('/import')
  await page.locator('input[type=file]').setInputFiles({
    name: 'goodreads_library_export.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from(goodreads),
  })
  await expect(page.getByText('From Goodreads: 1 of 2 books added.')).toBeVisible()
  await expect(page.getByText('1 book put on your wishlist.')).toBeVisible()
})

test('the library exports as a CSV file', async ({ page }) => {
  await page.goto('/settings')
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('link', { name: /Download the CSV/ }).click(),
  ])
  expect(download.suggestedFilename()).toMatch(/^home-library-books-\d{4}-\d{2}-\d{2}\.csv$/)
})

test('spine labels start off, barcode labels on', async ({ page }) => {
  await page.goto('/labels')
  await expect(page.getByRole('switch', { name: 'Barcode labels' })).toBeChecked()
  await expect(page.getByRole('switch', { name: 'Spine labels' })).not.toBeChecked()
})

test('the loans calendar link serves the due dates', async ({ page, request }) => {
  const id = await addBook(request, { title: 'Due Soon' })
  const book = await (await request.get(`/api/v1/books/${id}`)).json()
  await request.post(`/api/v1/copies/${book.copies[0].id}/loans`, {
    data: { borrower: 'Ana', lentOn: '2026-11-01', dueOn: '2026-11-20', returnedOn: '', notes: '' },
  })
  await page.goto('/loans')
  await page.getByRole('button', { name: 'Make a calendar link' }).click()
  const link = (await page.getByText(/\/calendar\/.+\.ics$/).textContent()) ?? ''
  const feed = await request.get(new URL(link).pathname)
  expect(feed.headers()['content-type']).toContain('text/calendar')
  expect(await feed.text()).toContain('DTSTART;VALUE=DATE:20261120')
})

test('Scan says whether a book is had, without a connection', async ({
  page,
  context,
  request,
}) => {
  await addBook(request, { title: 'Owned Offline', isbn: '9780425268285' })
  await page.goto('/scan')
  // Once the service worker has the page, the files load through it and are kept.
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null)
  await page.reload()
  await page.goto('/')
  // The list is kept once the library has loaded it.
  await expect
    .poll(() =>
      page.evaluate(async () =>
        Boolean(await caches.match('/offline/books', { cacheName: 'hl-offline' })),
      ),
    )
    .toBe(true)
  await page.goto('/scan')
  await context.setOffline(true)
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: 'Scan' })).toBeVisible()
  await page.getByLabel('Or type the ISBN').fill('0425268284')
  await page.getByRole('button', { name: 'Add to the list' }).click()
  await expect(
    page.getByRole('status').filter({ hasText: 'In your library: Owned Offline' }),
  ).toHaveCount(1)
  await context.setOffline(false)
})

// A link's choices as the share dialog sends them.
const everyBook = {
  books: 'all',
  categories: [],
  bookIds: [],
  wishlist: true,
  papers: 'none',
  courseIds: [],
  paperIds: [],
}

test('a visitor by a share link reads, and changes nothing', async ({ browser, request }) => {
  await addBook(request, { title: 'Seen by a Visitor' })
  const made = await request.post('/api/v1/shares', {
    data: { name: 'Test', expiresAt: '', scope: everyBook },
  })
  const { token } = (await made.json()) as { token: string }
  const visitor = await browser.newContext({ storageState: { cookies: [], origins: [] } })
  const page = await visitor.newPage()
  await page.goto(`/s/${token}`)
  // The tests share one library, longer than a page: the visitor finds the book by searching.
  await page.getByPlaceholder(/^Title, author/).fill('Seen by a Visitor')
  await expect(page.getByText('Seen by a Visitor').first()).toBeVisible()
  await expect(page.getByRole('button', { name: /Add a book/ })).toHaveCount(0)
  await visitor.close()
})

test('a link of one paper alone: the visitor has that paper, and no books', async ({
  page,
  browser,
  request,
}) => {
  const paper = await request.post('/api/v1/documents', {
    data: {
      kind: 'article',
      title: 'A Paper Shared Alone',
      authors: ['Ada Visitor'],
      editors: [],
      container: '',
      volume: '',
      issue: '',
      pages: '',
      published: '2001',
      publisher: '',
      doi: '',
      url: '',
      courseId: 0,
      abstract: '',
      notes: 'not for visitors',
      subjects: [],
    },
  })
  expect(paper.ok()).toBe(true)
  await addBook(request, { title: 'A Book Not Shared' })

  await page.goto('/settings')
  await page.getByRole('button', { name: 'Make a share link' }).click()
  const dialog = page.getByRole('dialog', { name: 'Make a share link' })
  await dialog.getByLabel(/^Who it is for/).fill('Reading group')
  await dialog.getByRole('group', { name: 'Books' }).getByRole('button', { name: 'None' }).click()
  await dialog.getByRole('group', { name: 'Papers' }).getByRole('button', { name: 'Some' }).click()
  // Nothing chosen yet: the link waits, and says why.
  await expect(dialog.getByRole('button', { name: 'Make the link' })).toBeDisabled()
  await dialog.getByLabel('And these papers').fill('Shared Alone')
  await page.getByRole('option', { name: /A Paper Shared Alone/ }).click()
  await expect(
    dialog.getByRole('cell', { name: 'A Paper Shared Alone', exact: true }),
  ).toBeVisible()
  await dialog.getByRole('button', { name: 'Make the link' }).click()
  const url = await page.getByRole('textbox', { name: 'Link' }).inputValue()
  await page.getByRole('button', { name: 'Done' }).click()
  await expect(page.getByText('No books; 1 paper').first()).toBeVisible()

  const visitor = await browser.newContext({ storageState: { cookies: [], origins: [] } })
  const them = await visitor.newPage()
  await them.goto(new URL(url).pathname)
  await expect(them).toHaveURL(/\/papers$/)
  await expect(them.getByRole('link', { name: 'A Paper Shared Alone' })).toBeVisible()
  await expect(them.getByRole('navigation', { name: 'Library' })).toHaveCount(0)
  // The books are not theirs: the library's address brings them back to the papers.
  await them.goto('/')
  await expect(them).toHaveURL(/\/papers$/)
  await them.getByRole('link', { name: 'A Paper Shared Alone' }).click()
  await expect(them.getByRole('heading', { level: 1, name: 'A Paper Shared Alone' })).toBeVisible()
  await expect(them.getByText('not for visitors')).toHaveCount(0)
  await visitor.close()
})

const endnote = `<?xml version="1.0" encoding="UTF-8" ?><xml><records><record><rec-number>1</rec-number><foreign-keys><key app="EN" db-id="e2e">1</key></foreign-keys><ref-type name="Book">6</ref-type><contributors><authors><author><style face="normal" font="default" size="100%">Cohen, I. Bernard</style></author></authors></contributors><titles><title><style face="normal" font="default" size="100%">Revolution in science</style></title></titles><isbn><style face="normal" font="default" size="100%">0674767772 (alk. paper)</style></isbn></record><record><rec-number>2</rec-number><foreign-keys><key app="EN" db-id="e2e">2</key></foreign-keys><ref-type name="Journal Article">17</ref-type><titles><title><style face="normal" font="default" size="100%">An Imported Article</style></title><secondary-title><style face="normal" font="default" size="100%">A Journal</style></secondary-title></titles><keywords><keyword><style face="normal" font="default" size="100%">Philosophy of science</style></keyword></keywords></record></records></xml>`

test('an EndNote export: its book joins the library, its article the papers', async ({ page }) => {
  await page.goto('/import')
  await page.getByLabel('Books not marked as owned').click()
  await page.getByRole('option', { name: 'Add them to my library' }).click()
  await page.locator('input[type=file]').setInputFiles({
    name: 'HPS Library.xml',
    mimeType: 'text/xml',
    buffer: Buffer.from(endnote),
  })
  await expect(
    page.getByText('From EndNote: 1 book and 1 paper added, of 2 references.'),
  ).toBeVisible()
  await page.goto('/papers')
  await expect(page.getByRole('link', { name: 'An Imported Article' })).toBeVisible()
})

test('the colors follow the device, or the choice made on it', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'light' })
  await page.goto('/settings')
  const dark = () => page.evaluate(() => document.documentElement.classList.contains('scheme-dark'))
  expect(await dark()).toBe(false)
  await page.getByRole('button', { name: 'Dark', exact: true }).click()
  expect(await dark()).toBe(true)
  await page.reload()
  expect(await dark()).toBe(true)
  await page.getByRole('button', { name: 'Match this device' }).click()
  expect(await dark()).toBe(false)
  // Matching the device, the page changes as the device does.
  await page.emulateMedia({ colorScheme: 'dark' })
  await expect.poll(dark).toBe(true)
})

test('every book is looked up again, after asking first', async ({ page, request }) => {
  await addBook(request, { title: 'Looked Up Again', isbn: '9781861972712' })
  await page.goto('/gaps')
  await page.getByRole('button', { name: 'Look up every book' }).click()
  const dialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
  await expect(dialog).toContainText('Only blank details are filled')
  await dialog.getByRole('button', { name: 'Look them all up' }).click()
  await expect(page.getByText(/queued for the background lookup/)).toBeVisible()
})
