import { axeViolations } from '@jwill89/test-a11y'
import { expect, test } from '@playwright/test'

// Every main page passes axe at WCAG 2.2 AA, the floor under the AAA target (the theme's
// contrast test holds AAA). The library is the one the other tests fill, so the pages show
// their lists as well as their empty states.
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

for (const path of pages) {
  test(`${path} passes axe`, async ({ page }) => {
    await page.goto(path)
    await expect(page.locator('h1')).toHaveCount(1)
    await page.waitForLoadState('networkidle')
    expect(await axeViolations(page)).toEqual([])
  })
}

test.describe('signed out', () => {
  test.use({ storageState: { cookies: [], origins: [] } })

  test('/sign-in passes axe', async ({ page }) => {
    await page.goto('/sign-in')
    await expect(page.locator('h1')).toHaveCount(1)
    expect(await axeViolations(page)).toEqual([])
  })
})
