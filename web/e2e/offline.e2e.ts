import { expect, test, type Page } from '@playwright/test'

/** The nonce of the page as it is shown: each answer from the server has its own. */
const pageNonce = (page: Page) =>
  page.evaluate(
    () => document.querySelector<HTMLMetaElement>('meta[property="csp-nonce"]')?.nonce ?? '',
  )

/** The nonce of the shell the service worker precached, the answer it was fetched with. */
const precachedNonce = (page: Page) =>
  page.evaluate(async () => {
    for (const name of await caches.keys()) {
      const cache = await caches.open(name)
      for (const request of await cache.keys()) {
        if (new URL(request.url).pathname === '/index.html') {
          const text = (await (await cache.match(request))?.text()) ?? ''
          return /<meta property="csp-nonce" nonce="([^"]+)"/.exec(text)?.[1] ?? ''
        }
      }
    }
    return ''
  })

// The installed app opens without a connection: the service worker answers the page with the
// shell it precached, under that shell's own policy, and the built files from what it keeps;
// the library is the list the device last had. With a connection, the page is the server's.
test('the app opens without a connection once its worker is in', async ({ context, page }) => {
  const refused: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error' && /Content Security Policy/i.test(message.text())) {
      refused.push(message.text())
    }
  })
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Library' })).toBeVisible()
  // The worker takes the page over once it is active; a reload then fetches each file through
  // it, and it keeps them.
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready
  })
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: 'Library' })).toBeVisible()
  expect(await page.evaluate(() => navigator.serviceWorker.controller !== null)).toBe(true)
  const precached = await precachedNonce(page)
  expect(precached).not.toBe('')
  expect(await pageNonce(page)).not.toBe(precached)

  await context.setOffline(true)
  try {
    await page.reload()
    await expect(page.getByRole('heading', { level: 1, name: 'Library' })).toBeVisible()
    await expect(
      page.getByText('No connection: this is the list as this device kept it'),
    ).toBeVisible()
    expect(await pageNonce(page)).toBe(precached)
  } finally {
    await context.setOffline(false)
  }
  expect(refused).toEqual([])
})
