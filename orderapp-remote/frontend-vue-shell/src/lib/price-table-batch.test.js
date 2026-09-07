import test from 'node:test'
import assert from 'node:assert/strict'
import { createPriceTableBatch, addPriceTable, removePriceTable, validatePriceTableBatch, savePriceTableBatchDraft, readPriceTableBatchDraft, publicationBatchGroups } from './price-table-batch.js'

test('named table drafts isolate all configuration and survive scope-specific restoration', () => {
  const source = { config: { selectedProductIDs: ['12'], version: 'V1' }, content: { price_rows: [{ final_unit_price: 38 }] }, draft: { defaults: { pricing_mode: 'fixed_price' } } }
  let batch = createPriceTableBatch(source, '227g价格表')
  batch = addPriceTable(batch, batch.tables[0].key)
  batch = addPriceTable(batch, batch.tables[0].key)
  assert.equal(batch.tables.length, 3)
  batch.tables[1].payload.config.selectedProductIDs.push('13')
  batch.tables[1].payload.content.price_rows[0].final_unit_price = 88
  assert.deepEqual(batch.tables[0].payload.config.selectedProductIDs, ['12'])
  assert.equal(batch.tables[0].payload.content.price_rows[0].final_unit_price, 38)
  assert.equal(validatePriceTableBatch(batch), '')
  const values = new Map(); const storage = { setItem: (k,v) => values.set(k,v), getItem:k=>values.get(k) }
  savePriceTableBatchDraft('customer:42:coffee', batch, storage)
  assert.equal(readPriceTableBatchDraft('customer:42:coffee', storage).tables.length, 3)
  assert.equal(readPriceTableBatchDraft('customer:43:coffee', storage), null)
})

test('batch validates names and exactly one default without a two-table limit', () => {
  let batch = createPriceTableBatch({}, 'A')
  for (let i=0;i<4;i++) batch=addPriceTable(batch)
  assert.equal(batch.tables.length, 5)
  batch.tables[1].name=' A '
  assert.match(validatePriceTableBatch(batch), /重复/)
  batch.tables[1].name=''
  assert.match(validatePriceTableBatch(batch), /名称/)
  batch.tables[1].name='B';batch.default_table_key='missing'
  assert.match(validatePriceTableBatch(batch), /默认/)
  batch.default_table_key=batch.tables[0].key
  assert.throws(()=>removePriceTable(createPriceTableBatch({}, 'A'), 'missing'), /至少/)
  batch=removePriceTable(batch,batch.tables[0].key)
  assert.equal(batch.default_table_key,batch.tables[0].key)
})

test('published grouping uses release identity and keeps unrelated legacy versions separate', () => {
  const rows=[{id:1,version:'V1',release_id:'x',table_name:'A'},{id:2,version:'V1',release_id:'x',table_name:'B'},{id:3,version:'V1'},{id:4,version:'V1'}]
  const groups=publicationBatchGroups(rows)
  assert.equal(groups.length,3)
  assert.deepEqual(groups[0].tables.map(row=>row.id),[1,2])
})

test('price-table editor places configuration directly before batch publication and exposes named selection', async () => {
  const { readFile } = await import('node:fs/promises')
  const source = await readFile(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
  const template = source.split('<script')[0]
  assert.equal((template.match(/>价格表配置<\/button>/g) || []).length, 1)
  assert.match(template, />价格表配置<\/button>\s*<button[^>]*@click="publishBeanList"[^>]*>发布价格表<\/button>/)
  assert.match(template, /aria-label="当前编辑价格表"/)
  assert.match(template, /默认价格表/)
  assert.match(source, /bean-list\/publication-batches/)
})

test('order drafts retain per-type selections and only submit visible group selections', async () => {
  const { readFile } = await import('node:fs/promises')
  const source = await readFile(new URL('../views/OrderEntryView.vue', import.meta.url), 'utf8')
  assert.match(source, /selectedPriceTableIDs: \{ \.\.\.selectedBeanListPublicationIDs \}/)
  assert.match(source, /Object.assign\(selectedBeanListPublicationIDs, draft.selectedPriceTableIDs \|\| \{\}\)/)
  assert.match(source, /payload.selected_price_table_ids = .*beanListVersionGroups.value.map/)
})
