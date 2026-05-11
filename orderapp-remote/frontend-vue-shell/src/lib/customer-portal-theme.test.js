import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { customerPortalThemeOptions, defaultCustomerPortalThemeKey, normalizeCustomerPortalThemeKey } from './customer-portal-theme.js'

const currentDir = path.dirname(fileURLToPath(import.meta.url))

test('customer portal exposes the three built-in miniapp themes', () => {
  assert.equal(defaultCustomerPortalThemeKey, 'coffee_factory')
  assert.deepEqual(customerPortalThemeOptions.map((item) => item.key), [
    'coffee_factory',
    'clean_ops',
    'premium_partner',
  ])
  assert.ok(customerPortalThemeOptions.every((item) => item.label && item.description && item.swatchClass))
})

test('customer portal theme normalization falls back to coffee factory', () => {
  assert.equal(normalizeCustomerPortalThemeKey('clean_ops'), 'clean_ops')
  assert.equal(normalizeCustomerPortalThemeKey('premium_partner'), 'premium_partner')
  assert.equal(normalizeCustomerPortalThemeKey(''), 'coffee_factory')
  assert.equal(normalizeCustomerPortalThemeKey('unknown'), 'coffee_factory')
})

test('customer capability template view configures the miniapp theme', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerCapabilityTemplatesView.vue'), 'utf8')
  assert.match(source, /import\s+\{\s*customerPortalThemeOptions,\s*normalizeCustomerPortalThemeKey\s*\}\s+from\s+'\.\.\/lib\/customer-portal-theme'/)
  assert.match(source, /theme_key:\s*normalizeCustomerPortalThemeKey\(template\?\.theme_key\)/)
  assert.match(source, /theme_key:\s*normalizeCustomerPortalThemeKey\(editor\.form\.theme_key\)/)
  assert.match(source, /@click="editor\.form\.theme_key = theme\.key"/)
  assert.match(source, /<span>小程序主题<\/span>/)
  assert.match(source, /:title="theme\.description"/)
  assert.match(source, /grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/)
  assert.doesNotMatch(source, /<small>\{\{\s*theme\.description\s*\}\}<\/small>/)
  for (const want of [
    'theme-option',
    'theme-swatch-coffee',
    'theme-swatch-clean',
    'theme-swatch-premium',
  ]) {
    assert.ok(source.includes(want), `missing ${want}`)
  }
})

test('customer portal settings view only references capability templates', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerPortalSettingsView.vue'), 'utf8')
  assert.match(source, /<span>能力模板<\/span>/)
  assert.match(source, /保存并应用模板/)
  assert.match(source, /selectedTemplate\(row\)/)
  assert.doesNotMatch(source, /customerPortalThemeOptions/)
  assert.doesNotMatch(source, /@click="row\.form\.theme_key = theme\.key"/)
  assert.doesNotMatch(source, /v-model="capability\.enabled"/)
})
