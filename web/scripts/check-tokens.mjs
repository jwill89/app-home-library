// Fails when a literal color appears outside src/app/theme, so every color stays a named token.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join, relative, sep } from 'node:path'

const root = fileURLToPath(new URL('../src', import.meta.url))
const exempt = join(root, 'app', 'theme') + sep
const extensions = ['.vue', '.ts', '.css', '.html']
const literal =
  /#(?:[0-9a-f]{8}|[0-9a-f]{6}|[0-9a-f]{3,4})\b|\b(?:rgba?|hsla?|oklch|oklab|lab|lch|hwb)\(/gi

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) yield* walk(path)
    else if (extensions.some((ext) => name.endsWith(ext))) yield path
  }
}

const findings = []
for (const path of walk(root)) {
  if (path.startsWith(exempt)) continue
  readFileSync(path, 'utf8')
    .split('\n')
    .forEach((line, i) => {
      if (literal.test(line)) findings.push(`${relative(root, path)}:${i + 1}: ${line.trim()}`)
      literal.lastIndex = 0
    })
}

if (findings.length > 0) {
  console.error(`Literal colors outside src/app/theme:\n${findings.join('\n')}`)
  process.exit(1)
}
console.log('check-tokens: no literal colors outside src/app/theme')
