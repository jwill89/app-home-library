import { expect, test } from '@playwright/test'

// Every page at a phone's width: one main heading, a skip link first, and nothing wider than
// the screen. The full contrast and target audit is the page audit's; these hold the line.
const pages = [
  '/',
  '/wishlist',
  '/series',
  '/reading',
  '/papers',
  '/scan',
  '/add',
  '/loans',
  '/courses',
  '/shelves',
  '/labels',
  '/gaps',
  '/suggestions',
  '/import',
  '/settings',
]

test.use({ viewport: { width: 390, height: 844 } })

for (const path of pages) {
  test(`${path} at a phone's width`, async ({ page }) => {
    await page.goto(path)
    await expect(page.locator('h1')).toHaveCount(1)
    const first = await page.evaluate(() => {
      const el = document.querySelector('a[href],button,input,select,textarea')
      return el?.getAttribute('href') ?? el?.tagName
    })
    expect(first).toBe('#main-content')
    const wide = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
    expect(wide).toBeLessThanOrEqual(1)
  })
}

test('the skip link shows only with the focus, and moves it to the content', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('h1')).toHaveCount(1)
  const skip = page.getByRole('link', { name: 'Skip to the content' })
  await expect(skip).not.toBeInViewport()
  await page.keyboard.press('Tab')
  await expect(skip).toBeFocused()
  await expect(skip).toBeInViewport()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('main')).toBeFocused()
  expect(new URL(page.url()).hash).toBe('')
})

test('moving to another page puts the focus on its content', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Loans' }).click()
  await expect(page).toHaveURL(/\/loans$/)
  await expect(page.getByRole('main')).toBeFocused()
})
