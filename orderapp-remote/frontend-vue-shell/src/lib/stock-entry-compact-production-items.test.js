import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const source = fs.readFileSync(
  new URL('../views/StockEntriesView.vue', import.meta.url),
  'utf8',
)

test('work-order material documents use one aligned compact row per material', () => {
  assert.match(source, /const usesCompactProductionItemRows = computed/)
  assert.match(source, /<div v-if="usesCompactProductionItemRows" class="compact-production-items">/)
  assert.match(source, /class="compact-production-items-head"/)
  assert.match(source, /class="compact-production-item-grid"/)

  const compactStart = source.indexOf('<div v-if="usesCompactProductionItemRows"')
  const regularStart = source.indexOf('<div v-else v-for="(item, index) in form.items"', compactStart)
  assert.ok(compactStart >= 0 && regularStart > compactStart)

  const compactSource = source.slice(compactStart, regularStart)
  for (const label of ['物料', '出库仓', '入库仓', '库存单位', '指定批次', '操作']) {
    assert.match(compactSource, new RegExp(`>${label}<`), `compact header should contain ${label}`)
  }
  assert.match(compactSource, />\{\{ productionQuantityLabel \}\}</)
  assert.doesNotMatch(compactSource, />类型</)
  assert.match(compactSource, /v-model\.number="item\.quantity"/)
  assert.match(compactSource, /item\.inventory_unit \|\| '-'/)
  assert.match(compactSource, /form\.items\.splice\(index, 1\)/)
  for (const label of ['物料', '出库仓', '入库仓', '指定批次（可选）']) {
    assert.ok(compactSource.includes(`aria-label="${label}"`), `compact control should expose ${label}`)
  }
  assert.match(compactSource, /<output class="readonly-value" aria-label="库存单位">/)
})

test('compact work-order rows fit desktop and wrap only on narrow screens', () => {
  assert.match(source, /\.stock-entry-page,\.stock-entry-page \*\{box-sizing:border-box\}/)
  assert.match(
    source,
    /\.compact-production-item-grid\{[^}]*grid-template-columns:minmax\(150px,2fr\) minmax\(90px,1fr\) minmax\(90px,1fr\) minmax\(90px,\.8fr\) minmax\(64px,\.55fr\) minmax\(120px,1\.2fr\) 52px/s,
  )
  assert.match(source, /\.compact-production-item\{[^}]*padding:4px 6px/s)
  assert.match(source, /\.compact-production-item-grid select,\.compact-production-item-grid input,\.compact-production-item-grid \.readonly-value\{[^}]*min-height:32px/s)
  const mobileStart = source.indexOf('@media(max-width:900px)')
  const narrowStart = source.indexOf('@media(max-width:560px)', mobileStart)
  const mobileSource = source.slice(mobileStart, narrowStart)
  assert.match(mobileSource, /\.compact-production-items-head\{display:none\}/)
  assert.match(mobileSource, /\.compact-production-item-grid\{grid-template-columns:1fr 1fr;/)
  assert.match(mobileSource, /\.compact-production-item-grid \.mobile-field-label\{display:block\}/)
})

test('work-order WIP shortage stays a visible suggestion instead of a quantity limit', () => {
  assert.match(source, /const drawerWarnings = ref\(\[\]\)/)
  assert.match(source, /class="warning-list" role="status"/)
  assert.match(source, /drawerWarnings\.value = Array\.isArray\(warnings\)/)
  assert.match(source, /\}, preview\.warnings\)/)
  assert.doesNotMatch(source, /limitsToCurrentWIPShortage/)
  assert.doesNotMatch(source, /:max="[^"]*remaining_qty/)
  assert.doesNotMatch(source, /领用数量超过当前剩余 WIP 缺口/)
  assert.match(source, /工单建议领用量仅用于默认填充/)
  assert.match(source, /超出工单当前需求的部分将保留为可用 WIP 库存/)
  assert.match(source, /生产消耗仍需另行记录/)
  assert.match(source, /remaining_qty: null/)
  assert.match(source, /stockCanonicalQuantity\(item\)/)
  assert.match(source, /缺少库存单位换算/)
})

test('saving omits zero canonical quantities and rejects an all-zero document', () => {
  assert.match(source, /stockDocumentPositiveItems\(form\.items/)
  assert.match(source, /canonicalQuantity\(item\)/)
  assert.match(source, /缺少库存单位换算/)
})

test('opening a bound production draft refreshes it through the work-order preview', () => {
  assert.match(source, /productionStockDocumentPreviewAction\(row\)/)
  assert.match(source, /row\.status === 'draft'/)
  assert.match(source, /\/api\/produce\/work-orders\/\$\{workOrderID\}\/stock-document-preview/)
  assert.match(source, /stock_document_id: Number\(row\.id \|\| 0\)/)
  assert.match(source, /return_source: 'stock_document_list'/)
  assert.match(source, /preview\.warnings/)
  assert.match(source, /applyDocument\(await apiGet\(`\$\{stockEntryEndpoint\(\)\}\/\$\{row\.id\}`\)\)/)
})

test('regular stock documents keep the existing card form', () => {
  assert.match(source, /<div v-else v-for="\(item, index\) in form\.items"[^>]*class="item-card">/)
  assert.match(source, /<template v-if="isReceipt && item\.item_type === 'material'">/)
  assert.match(source, /<span>供应商<\/span>/)
  assert.match(source, /<span>单位成本<\/span>/)
})
