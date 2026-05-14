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

test('customer portal settings disables ERP binding for templates without workbench views', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerPortalSettingsView.vue'), 'utf8')
  assert.match(source, /function\s+templateSupportsERPWorkbench\(row\)/)
  assert.match(source, /:disabled="!templateSupportsERPWorkbench\(row\)"/)
  assert.match(source, /!templateSupportsERPWorkbench\(row\)/)
  assert.match(source, /该模板不开放 ERP 工作台/)
})

test('customer portal settings excludes disabled channel accounts from ERP binding selector', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerPortalSettingsView.vue'), 'utf8')
  assert.match(source, /account_type\s*===\s*'channel_customer'/)
  assert.match(source, /login_disabled\s*!==\s*true/)
})

test('customer portal settings preserves unknown template keys for correction', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerPortalSettingsView.vue'), 'utf8')
  assert.match(source, /function\s+unknownTemplateKey\(row\)/)
  assert.match(source, /未知能力模板/)
  assert.match(source, /capability_template_key:\s*trimTemplateKey\(row\.form\.capability_template_key\)/)
  assert.doesNotMatch(source, /capability_template_key:\s*normalizeTemplateKey\(row\.form\.capability_template_key\)/)
  assert.match(source, /!unknownTemplateKey\(row\)/)
})

test('customer processing portal uses the ERP fulfillment order list and document drawers', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerProcessingPortalView.vue'), 'utf8')
  for (const want of [
    '履约客户订单',
    '订单费用',
    'fetchCustomerFulfillmentOrders',
    'fetchCustomerFulfillmentOrderDetail',
    'customerFulfillmentOrderFees',
    'SalesOrderView',
    'DeliveryNoteView',
    'openFulfillmentOrderDetail',
  ]) {
    assert.ok(source.includes(want), `missing ${want}`)
  }
  assert.doesNotMatch(source, /overview\.direct_ship_orders/)
})

test('customer capability templates view supports manual child templates and folding', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerCapabilityTemplatesView.vue'), 'utf8')
  for (const want of [
    '复制模板',
    '模板失效',
    'parent_template_key',
    'active',
    'sort_order',
    'expandedTemplateKey',
    'templateSummary',
    'visibleTemplateEditors',
    'flattenTemplateEditorsForTree',
    'isTemplateExpanded',
    'toggleTemplateExpanded',
    'copyTemplate',
  ]) {
    assert.ok(source.includes(want), `missing ${want}`)
  }
  assert.match(source, /window\.prompt\('请输入新模板名称'/)
  assert.doesNotMatch(source, /请输入新模板 key/)
  assert.doesNotMatch(source, /body:\s*\{\s*new_key:/)
  assert.match(source, /body:\s*\{\s*label\s*\}/)
})

test('customer portal settings only lets customers bind active templates', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerPortalSettingsView.vue'), 'utf8')
  for (const want of [
    'activeTemplates',
    '模板已失效',
    'inactiveTemplateKey',
    'template.active !== false',
  ]) {
    assert.ok(source.includes(want), `missing ${want}`)
  }
})
