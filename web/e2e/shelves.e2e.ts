import { expect, test } from '@playwright/test'
import { addBook } from './library'

test('a bookcase whose bottom shelf is taller keeps each shelf’s own height', async ({ page }) => {
  await page.goto('/shelves')
  await page.getByRole('button', { name: 'Add a bookcase' }).click()
  const dialog = page.getByRole('dialog', { name: 'Add a bookcase' })
  // A required field's name ends "(required)"; a length's starts with what it measures.
  const length = (name: string) => dialog.getByRole('spinbutton', { name: new RegExp(`^${name}`) })
  await dialog.getByRole('button', { name: 'Centimetres' }).click()
  await dialog.getByRole('textbox', { name: /^Name/ }).fill('Living room, left')
  await length('Shelves$').fill('3')
  await length('Width, cm').fill('80')
  await length('Height, cm').fill('28')
  await length('Depth, cm').fill('30')

  // Each shelf starts at the shared height; only the bottom one is changed.
  await dialog.getByRole('switch', { name: 'Some shelves are another height' }).check()
  await expect(length('Shelf 1, cm')).toHaveValue('28')
  await length('Shelf 3, cm').fill('38')
  await dialog.getByRole('button', { name: 'Add the bookcase' }).click()
  await expect(dialog).toBeHidden()

  const bookcase = page.getByRole('region', { name: 'Living room, left' })
  await expect(bookcase.getByRole('cell', { name: '80 × 28 × 30 cm' })).toHaveCount(2)
  await expect(bookcase.getByRole('cell', { name: '80 × 38 × 30 cm' })).toHaveCount(1)
})

test('a second bookcase starts from the last one’s measurements, and takes another shelf', async ({
  page,
}) => {
  await page.goto('/shelves')
  await page.getByRole('button', { name: 'Add a bookcase' }).click()
  const dialog = page.getByRole('dialog', { name: 'Add a bookcase' })
  await dialog.getByRole('button', { name: 'Centimetres' }).click()
  await dialog.getByRole('textbox', { name: /^Name/ }).fill('Living room, right')
  await dialog.getByRole('spinbutton', { name: /^Shelves$/ }).fill('2')
  await dialog.getByRole('button', { name: 'Add the bookcase' }).click()
  await expect(dialog).toBeHidden()

  // Its shelves are the first shelf of the bookcase before it, measured again by nobody.
  const bookcase = page.getByRole('region', { name: 'Living room, right' })
  await expect(bookcase.getByRole('cell', { name: '80 × 28 × 30 cm' })).toHaveCount(2)

  await bookcase.getByRole('button', { name: 'Add a shelf' }).click()
  const shelf = page.getByRole('dialog', { name: 'Add a shelf to Living room, right' })
  await shelf.getByRole('button', { name: 'Add the shelf' }).click()
  await expect(shelf).toBeHidden()
  await expect(bookcase.getByRole('cell', { name: '80 × 28 × 30 cm' })).toHaveCount(3)
})

test('no book has a place until a layout by the areas is suggested and put to use', async ({
  page,
  request,
}) => {
  const book = await addBook(request, {
    title: 'Shelf Area Test',
    ddc: '510',
    category: 'nonfiction',
    heightMm: 200,
    thicknessMm: 20,
    pages: 200,
  })
  await page.goto('/shelves')
  await expect(page.getByText('No book has a place on the shelves yet.')).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'Holds' })).toHaveCount(0)

  // An area for the nonfiction, on the left bookcase's top two shelves, by subject then author.
  await page.getByRole('button', { name: 'Add an area' }).first().click()
  const dialog = page.getByRole('dialog', { name: 'Add an area' })
  await dialog.getByRole('textbox', { name: /^Name/ }).fill('Nonfiction')
  await dialog
    .getByRole('group', { name: 'Categories' })
    .getByRole('button', { name: 'Nonfiction' })
    .click()
  const left = dialog.getByRole('group', { name: 'Living room, left' })
  await left.getByRole('button', { name: '1', exact: true }).click()
  await left.getByRole('button', { name: '2', exact: true }).click()
  await dialog.getByRole('button', { name: 'Add the area' }).click()
  await expect(dialog).toBeHidden()
  const areas = page.getByRole('region', { name: 'Areas' })
  await expect(areas.getByText('Living room, left, shelves 1 and 2')).toBeVisible()

  await page.getByRole('button', { name: 'Suggest a layout' }).click()
  await expect(page.getByRole('columnheader', { name: 'Would hold' }).first()).toBeVisible()
  await page.getByRole('button', { name: 'Put it to use' }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Put it to use' }).click()
  await expect(page.getByText(/In use since/)).toBeVisible()

  await page.goto(`/books/${String(book)}`)
  await expect(
    page.getByText(/On Living room, left, shelf [12] from the top \(Nonfiction\)/),
  ).toBeVisible()
})
