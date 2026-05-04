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

test('customer portal settings view saves selected miniapp theme', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerPortalSettingsView.vue'), 'utf8')
  for (const want of [
    'customerPortalThemeOptions',
    'normalizeCustomerPortalThemeKey',
    'theme_key',
    '小程序主题',
    'theme-option',
    'theme-swatch-coffee',
    'theme-swatch-clean',
    'theme-swatch-premium',
  ]) {
    assert.ok(source.includes(want), `missing ${want}`)
  }
})
