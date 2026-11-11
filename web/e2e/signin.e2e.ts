import { expect, test, type Page } from '@playwright/test'

// The passkey ceremonies as a browser runs them, with a virtual authenticator in place of the
// device's own: a device set up with a code from the host, signed out, and signed in again. The
// server reads what a real browser sends, which no Go test can make without an authenticator.

async function who(page: Page): Promise<string> {
  const me = (await (await page.request.get('/api/v1/me')).json()) as { role: string }
  return me.role
}

test('a device set up with a code signs out and back in', async ({ browser }) => {
  const code = process.env.E2E_SETUP_CODE ?? ''
  expect(code, 'the setup code the server setup made').not.toBe('')
  const device = await browser.newContext({ storageState: { cookies: [], origins: [] } })
  const page = await device.newPage()
  const cdp = await device.newCDPSession(page)
  await cdp.send('WebAuthn.enable')
  await cdp.send('WebAuthn.addVirtualAuthenticator', {
    options: {
      protocol: 'ctap2',
      transport: 'internal',
      hasResidentKey: true,
      hasUserVerification: true,
      isUserVerified: true,
      automaticPresenceSimulation: true,
    },
  })

  await page.goto('/setup')
  await page.getByLabel('Setup code').fill(code)
  await page.getByLabel('Name for this device').fill('Test browser')
  await page.getByRole('button', { name: 'Make a passkey' }).click()
  await expect(page).toHaveURL('/')
  expect(await who(page)).toBe('owner')

  await page.goto('/settings')
  await expect(page.getByText('Test browser')).toBeVisible()
  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page).toHaveURL('/sign-in')
  expect(await who(page)).toBe('none')

  await page.getByRole('button', { name: 'Sign in with a passkey' }).click()
  await expect(page).toHaveURL('/')
  expect(await who(page)).toBe('owner')
  await device.close()
})

test('a visitor who types a wrong setup code stays a visitor, on the page', async ({
  browser,
  request,
}) => {
  const made = await request.post('/api/v1/shares', {
    data: {
      name: 'Sign-in test',
      expiresAt: '',
      scope: {
        books: 'all',
        categories: [],
        bookIds: [],
        wishlist: true,
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
  expect(await who(page)).toBe('visitor')

  await page.goto('/setup')
  await page.getByLabel('Setup code').fill('ABCD-EFGH')
  await page.getByRole('button', { name: 'Make a passkey' }).click()
  await expect(page.getByRole('alert')).toContainText('That code is wrong, used, or expired')
  await expect(page).toHaveURL('/setup')
  expect(await who(page)).toBe('visitor')
  await visitor.close()
})
