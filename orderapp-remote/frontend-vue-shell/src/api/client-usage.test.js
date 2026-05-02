import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { test } from 'node:test'

test('Vue views use the shared API client instead of raw fetch', () => {
  const viewsDir = new URL('../views', import.meta.url)
  const offenders = []
  for (const file of readdirSync(viewsDir)) {
    if (!file.endsWith('.vue')) continue
    const source = readFileSync(join(viewsDir.pathname, file), 'utf8')
    if (/\bfetch\s*\(/.test(source)) offenders.push(file)
  }
  assert.deepEqual(offenders, [])
})
