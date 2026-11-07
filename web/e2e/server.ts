import { execFileSync, spawn } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// The end-to-end tests' own server: the app built from this tree, on its own port, with an
// empty library in a folder of its own, and the owner signed in through a development session.
// No source is asked for a cover or a lookup that a test needs; a book added with an ISBN may
// still be looked up in the background, which a test does not wait on.

export const port = 8140
export const origin = `http://localhost:${port}`
const here = path.dirname(fileURLToPath(import.meta.url))
export const statePath = path.join(here, '.state.json')

export default async function setup(): Promise<() => Promise<void>> {
  const root = path.resolve(here, '../..')
  const dir = mkdtempSync(path.join(tmpdir(), 'hl-e2e-'))
  const bin = path.join(dir, process.platform === 'win32' ? 'homelibrary.exe' : 'homelibrary')
  execFileSync('go', ['build', '-o', bin, './cmd/homelibrary'], { cwd: root, stdio: 'inherit' })
  const env = {
    ...process.env,
    HL_ORIGINS: origin,
    HL_ADDR: `127.0.0.1:${port}`,
    HL_DB_PATH: path.join(dir, 'library.sqlite'),
    HL_MEDIA_DIR: path.join(dir, 'media'),
    HL_DOCS_DIR: path.join(dir, 'documents'),
    HL_GOOGLE_BOOKS_KEY: '',
    HL_HARDCOVER_TOKEN: '',
  }
  execFileSync(bin, ['migrate'], { env })
  const line = execFileSync(bin, ['dev-session'], { env }).toString().trim().split('\n').pop() ?? ''
  const value = line.replace(/^hl_session=/, '').trim()
  writeFileSync(
    statePath,
    JSON.stringify({
      cookies: [
        {
          name: 'hl_session',
          value,
          domain: 'localhost',
          path: '/',
          expires: -1,
          httpOnly: true,
          secure: false,
          sameSite: 'Lax',
        },
      ],
      origins: [],
    }),
  )
  const server = spawn(bin, [], { env, stdio: 'ignore' })
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(`${origin}/healthz`)).ok) {
        break
      }
    } catch {
      // Not listening yet.
    }
    await new Promise((r) => setTimeout(r, 100))
  }
  return async () => {
    server.kill()
    await new Promise((r) => setTimeout(r, 300))
    rmSync(dir, { recursive: true, force: true })
  }
}
