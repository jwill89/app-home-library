import { expect, test } from '@playwright/test'

const pdf = Buffer.from('%PDF-1.4\n1 0 obj << >> endobj\ntrailer << >>\n%%EOF\n')

// A document's details as the API takes them, all blank.
const documentBody = {
  kind: 'article',
  title: '',
  authors: [] as string[],
  editors: [] as string[],
  container: '',
  volume: '',
  issue: '',
  pages: '',
  published: '',
  publisher: '',
  doi: '',
  url: '',
  courseId: 0,
  abstract: '',
  notes: '',
  subjects: [] as string[],
}

test('a document added with its PDF and a subject, and found by the subject', async ({
  page,
  request,
}) => {
  await page.goto('/papers/new')
  await page.getByLabel(/^Title/).fill('A Chapter With Its PDF')
  await page.getByLabel('Subjects').fill('Folk science')
  await page.getByLabel('Subjects').press('Enter')
  await page.getByLabel('Choose a PDF or an image to add with the paper').setInputFiles({
    name: 'chapter.pdf',
    mimeType: 'application/pdf',
    buffer: pdf,
  })
  await expect(page.getByText('chapter.pdf')).toBeVisible()
  await page.getByRole('button', { name: 'Add the paper' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: 'A Chapter With Its PDF' }),
  ).toBeVisible()
  await expect(page.getByText('chapter.pdf').first()).toBeVisible()
  await expect(page.getByRole('listitem').filter({ hasText: 'Folk science' })).toBeVisible()
  const found = (await (await request.get('/api/v1/documents?q=folk')).json()) as {
    documents: { title: string }[]
  }
  expect(found.documents.map((d) => d.title)).toContain('A Chapter With Its PDF')
})

test('a library export fills in the blanks of a document', async ({ page }) => {
  await page.goto('/papers/new')
  await page.getByLabel(/^Title/).fill('My Own Title')
  await page.getByLabel('Choose a citation exported as RIS').setInputFiles({
    name: 'record.ris',
    mimeType: 'application/x-research-info-systems',
    buffer: Buffer.from(
      [
        'TY  - JOUR',
        'T1  - The Export Title',
        'AU  - Doe, Jane',
        'JO  - A Journal',
        'Y1  - 2019/03/01/',
        'KW  - SCIENTIFIC revolutions',
        'AB  - What it argues.',
        'ER  - ',
      ].join('\n'),
    ),
  })
  await expect(page.getByRole('status')).toContainText('Filled in from the export')
  // The title the owner typed stays; the blanks take the export's.
  await expect(page.getByLabel(/^Title/)).toHaveValue('My Own Title')
  await expect(page.getByLabel('Journal')).toHaveValue('A Journal')
  await expect(page.getByLabel('Abstract')).toHaveValue('What it argues.')
  await expect(page.getByLabel('Notes')).toHaveValue('')
  await expect(page.getByText('Scientific revolutions')).toBeVisible()
})

test('a copy found in the library the owner set, not without one', async ({ page, request }) => {
  const res = await request.post('/api/v1/documents', {
    data: { ...documentBody, title: 'Finding Theory: A Study', authors: ['Thomas Kuhn'] },
  })
  expect(res.status(), await res.text()).toBe(201)
  const { id } = (await res.json()) as { id: number }
  await page.goto(`/papers/${id}`)
  await page.getByRole('button', { name: 'Find a copy' }).click()
  const dialog = page.getByRole('dialog', { name: 'Find a copy' })
  await expect(dialog.getByText('Without a DOI, OpenAlex cannot be asked.')).toBeVisible()
  await expect(dialog.getByRole('link', { name: 'Settings' })).toBeVisible()
  await dialog.getByRole('button', { name: 'Done' }).click()

  await page.goto('/settings')
  await page.getByLabel('Library search').fill('https://research.ebsco.com/c/abc123/search')
  await page.goto(`/papers/${id}`)
  await page.getByRole('button', { name: 'Find a copy' }).click()
  await expect(page.getByRole('link', { name: /Search your library for it/ })).toHaveAttribute(
    'href',
    'https://research.ebsco.com/c/abc123/search/results?q=%22Finding%20Theory%22%20Kuhn',
  )
})

test('a chapter names its book’s editors, and its citation does too', async ({ page }) => {
  await page.goto('/papers/new')
  await expect(page.getByLabel("The book's editors")).toBeHidden()
  await page.getByLabel('Kind').click()
  await page.getByRole('option', { name: 'Chapter' }).click()
  await page.getByLabel(/^Title/).fill('An Edited Chapter')
  await page.getByLabel('Book', { exact: true }).fill('A Collection')
  await page.getByLabel("The book's editors").fill('Huw Price')
  await page.getByLabel("The book's editors").press('Enter')
  await page.getByRole('button', { name: 'Add the paper' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'An Edited Chapter' })).toBeVisible()
  await expect(page.getByText('Huw Price').first()).toBeVisible()
  await expect(page.getByText(/In H\. Price \(Ed\.\), /)).toBeVisible()
})
