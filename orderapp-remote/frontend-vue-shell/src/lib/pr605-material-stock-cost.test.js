import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const source = (name) => readFileSync(resolve(here, `../views/${name}.vue`), 'utf8')
const materials = source('MaterialsView')
const stockEntries = source('StockEntriesView')
const stockAdjustments = source('StockAdjustmentsView')
const purchase = source('PurchaseView')
const app = readFileSync(resolve(here, '../App.vue'), 'utf8')
const manufacturing = readFileSync(resolve(here, './manufacturing-execution.js'), 'utf8')

test('PR-605 material archive uses standard checkboxes, industry column, and readonly receipt cost', () => {
  assert.match(materials, /class="material-row-checkbox"/)
  assert.match(materials, /\.material-row-checkbox\s*\{[^}]*width:\s*18px[^}]*height:\s*18px[^}]*min-height:\s*18px/s)
  assert.match(materials, /<th>行业字段<\/th>/)
  assert.match(materials, /industryFieldSummary\(row\)/)
  assert.match(materials, /最近采购入库价/)
  assert.match(materials, /<output class="readonly-cost-value">/)
  assert.match(materials, /v-if="!draftMode && draft\.supply_mode !== 'manufacture'"/)
  assert.match(materials, /前往采购入库或盘点调整/)
  assert.doesNotMatch(materials, /v-model\.number="draft\.purchase_price"/)
  assert.doesNotMatch(materials, /purchase_price:\s*draft\.value\.supply_mode/)
})

test('PR-605 stock entries show refreshed source warehouse balances and retire ordinary receipts', () => {
  assert.match(stockEntries, /\/api\/stock\/material-balances/)
  assert.match(stockEntries, /来源仓账面库存/)
  assert.match(stockEntries, /可用库存/)
  assert.match(stockEntries, /冻结库存/)
  assert.match(stockEntries, /loadMaterialBalances/)
  assert.match(stockEntries, /request !== materialBalanceRequest/)
  assert.doesNotMatch(manufacturing, /value:\s*'material_receipt'/)
  assert.doesNotMatch(stockEntries, /material_receipt:\s*\[/)
  assert.match(app, /materialReceipts:\s*'purchase'/)
})

test('PR-605 stock adjustment initializes warehouse quantity and supports discrete batch cost', () => {
  assert.match(stockAdjustments, /\/api\/stock\/material-balances/)
  assert.match(stockAdjustments, /当前仓库账面库存/)
  assert.match(stockAdjustments, /initializeMaterialTargetFromBalance/)
  assert.match(stockAdjustments, /request !== materialBalanceRequest/)
  assert.doesNotMatch(stockAdjustments, /filter\(\(material\) => isMaterialWeight\(material\)\)/)
  assert.doesNotMatch(stockAdjustments, /当前只支持重量物料/)
  assert.doesNotMatch(stockAdjustments, /!isSelectedMaterialWeight/)
  assert.match(stockAdjustments, /remaining_units/)
})

test('PR-605 purchase supports inventory units, target warehouse, and explicit receipt confirmation', () => {
  assert.match(purchase, /orderForm\.qty/)
  assert.match(purchase, /orderForm\.unit_code/)
  assert.match(purchase, /target_warehouse/)
  assert.match(purchase, /收货确认/)
  assert.match(purchase, /receiptForm/)
  assert.match(purchase, /最终单价/)
  assert.match(purchase, /selectedPurchaseMaterialUsesCount \? 1 : 0\.001/)
  assert.doesNotMatch(purchase, /当前采购单按克记录数量/)
  assert.doesNotMatch(purchase, /isSelectedPurchaseMaterialWeight/)
})
