import { execFileSync, spawn } from 'node:child_process'
import { appendFileSync, closeSync, mkdtempSync, openSync, rmSync, writeFileSync } from 'node:fs'
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
// What the server writes, and how it ended, kept after the run for when the tests fail.
const logPath = path.join(here, '.server.log')

async function answers(): Promise<boolean> {
  try {
    return (await fetch(`${origin}/healthz`, { signal: AbortSignal.timeout(1000) })).ok
  } catch {
    return false
  }
}

export default async function setup(): Promise<() => Promise<void>> {
  // A server already on the port (another run's, or one a stopped run left behind) would answer
  // these tests with its own library and sessions, and nearly every test would fail on a page
  // that isn't signed in, with nothing to say why.
  if (await answers()) {
    throw new Error(
      `Something is already serving ${origin}: another end-to-end run, or a server one left behind. Stop it and run again.`,
    )
  }
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
  // A code the sign-in test sets a device up with, made as the owner makes one on the host. The
  // tests' workers start after this, with the environment it leaves.
  const made = execFileSync(bin, ['setup-code'], { env }).toString()
  process.env.E2E_SETUP_CODE = /[A-Z2-9]{4}-[A-Z2-9]{4}/.exec(made)?.[0] ?? ''
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
  const log = openSync(logPath, 'w')
  const server = spawn(bin, [], { env, stdio: ['ignore', log, log] })
  closeSync(log)
  let ended = ''
  server.on('exit', (code, signal) => {
    ended = signal ? `stopped by ${signal}` : `exited with code ${String(code)}`
    appendFileSync(logPath, `\n[e2e] the server ${ended}\n`)
  })
  for (let i = 0; i < 100 && !ended && !(await answers()); i++) {
    await new Promise((r) => setTimeout(r, 100))
  }
  if (ended || !(await answers())) {
    server.kill()
    rmSync(dir, { recursive: true, force: true })
    throw new Error(
      `The end-to-end server did not start (${ended || 'no answer in 10 s'}); see ${logPath}.`,
    )
  }
  return async () => {
    server.kill()
    await new Promise((r) => setTimeout(r, 300))
    rmSync(dir, { recursive: true, force: true })
  }
}
