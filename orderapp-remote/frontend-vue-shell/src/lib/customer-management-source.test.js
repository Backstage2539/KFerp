import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { test } from 'node:test'

const root = new URL('..', import.meta.url).pathname

function source(rel) {
  return readFileSync(join(root, rel), 'utf8')
}

test('vue shell remounts page components by view identity when switching after product settings', () => {
  const app = source('App.vue')
  assert.match(app, /<ProductSettingsView\s+v-else-if="isProductSettingsView"/)
  assert.match(app, /function isProductSettingsKey\(key\)/)
  assert.match(app, /'productMaster'/)
  assert.match(app, /'productPriceManagement'/)
  assert.match(app, /'productConfigTemplates'/)
  assert.match(app, /:key="currentViewIdentity"/)
  assert.match(app, /:is="resolveInternalView\(currentKey\)"/)
  assert.match(app, /markRaw\(internalViews\[key\] \|\| UnknownView\)/)
  assert.match(app, /function isProductSettingsKey\(key\)/)
  assert.match(app, /const currentViewIdentity = computed\(\(\) => `\$\{currentKey\.value\}:/)
  assert.match(app, /function open\(key, params = \{\}, options = \{\}\)[\s\S]*?currentKey\.value = key/)
  assert.doesNotMatch(app, /const currentInternalView\s*=/)
})

test('customer portal settings removed bean list version and processing warehouse editors', () => {
  const view = source('views/CustomerPortalSettingsView.vue')
  assert.doesNotMatch(view, /豆单展示版本/)
  assert.doesNotMatch(view, /bean-list-picker/)
  assert.doesNotMatch(view, /processing_warehouse_code/)
  assert.match(view, /客户仓库/)
  assert.match(view, /row\.customer\.warehouses/)
})

test('warehouse inventory supports binding warehouses to customers from the inventory page', () => {
  const view = source('views/WarehouseInventoryView.vue')
  assert.match(view, /绑定客户/)
  assert.match(view, /warehouse-settings-drawer/)
  assert.match(view, /openWarehouseSettingsDrawer/)
  assert.match(view, /saveWarehouseCustomerBinding/)
  assert.match(view, /\/api\/stock\/warehouses\/\$\{encodeURIComponent\(selectedWarehouse\.value\)\}\/customer/)
})

test('customer drawer exposes inline add actions for customer type and order type selectors', () => {
  const view = source('views/CustomersView.vue')
  assert.match(view, /新增客户类型/)
  assert.match(view, /createCustomerTypeOption/)
  assert.match(view, /新增订单类型/)
  assert.match(view, /createOrderTypeOption/)
})

test('customer drawer uses the shared recipient parsing API', () => {
  const view = source('views/CustomersView.vue')
  assert.match(view, /apiSend\('\/api\/customer-recipient\/parse'/)
  assert.match(view, /customerRecipientFieldSnapshot/)
  assert.match(view, /mergeCustomerRecipientFields/)
  assert.match(view, /addressParsing/)
  assert.match(view, /:disabled="loading \|\| addressParsing"/)
  assert.match(view, /async function saveCustomer\(\) \{\s+if \(addressParsing\.value\) return/)
  assert.doesNotMatch(view, /from ['"]\.\.\/lib\/customer-recipient['"]/)
  assert.doesNotMatch(view, /parseRecipientText\(/)
  assert.doesNotMatch(view, /form\.name\s*=\s*parsed\.recipient_name/)
  assert.doesNotMatch(view, /if \(!form\.name && form\.contact\) form\.name = form\.contact/)
  const parserSource = view.slice(
    view.indexOf('async function applyAddressParse()'),
    view.indexOf('async function saveCustomer()'),
  )
  assert.match(parserSource, /const targetFieldsAtRequest = customerRecipientFieldSnapshot\(form\)/)
  assert.ok(
    parserSource.indexOf('const targetFieldsAtRequest = customerRecipientFieldSnapshot(form)') < parserSource.indexOf("await apiSend('/api/customer-recipient/parse'"),
  )
  assert.match(parserSource, /Object\.assign\(form, mergeCustomerRecipientFields\(form, parsed, targetFieldsAtRequest\)\)/)
  assert.equal(
    parserSource.match(/isCurrentAddressParse\(sequence, targetEditingID, source\)/g)?.length || 0,
    2,
    'success and failure responses must share the same drawer, customer and source guard',
  )
  const catchSource = parserSource.slice(parserSource.indexOf('} catch'), parserSource.indexOf('} finally'))
  assert.match(catchSource, /if \(isCurrentAddressParse\(sequence, targetEditingID, source\)\)/)
  assert.match(view, /company_phone: customerPhoneForERPForm\(data\)/)
  assert.match(view, /\{\{ customerPhoneForERPForm\(row\) \}\}/)
  assert.match(view, /company_phone: form\.company_phone,[\s\S]*?phone: form\.company_phone,/)
})
