import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h, nextTick } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../views/StockEntriesView.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'stock-transfer-runtime' }).content
code = code.replace("import { apiGet, apiSend } from '../api/client'", 'const apiGet = (...args) => globalThis.transferRuntimeAPI(...args); const apiSend = () => { throw Error("unexpected write") }')
code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)

test('actual transfer editor scopes each row to its warehouse and selects the stocked published specification', async () => {
  let editor
  globalThis.window = { location: { origin: 'http://localhost' }, clearTimeout, setTimeout }
  globalThis.transferRuntimeAPI = async path => {
    if (String(path).startsWith('/api/stock/warehouse-inventory')) {
      const warehouse = new URL(path, 'http://localhost').searchParams.get('warehouse')
      return { rows: warehouse === 'finished_goods'
        ? [{ item_id: 58, item_name: '曜石2.0', bom_spec_id: 69, qty_units: 171 }]
        : [{ item_id: 7, item_name: '初晓', bom_spec_id: 19, qty_units: 2 }], has_next: false }
    }
    if (path === '/api/products/58') return { product: { id: 58, name: '曜石2.0', bom_spec_authoritative: true, variants: [
      { bom_spec_id: 68, bom_variant_id: 1205, name: '227g', inventory_unit: '袋', is_default: true },
      { bom_spec_id: 69, bom_variant_id: 1206, name: '454g', inventory_unit: '袋' },
    ] } }
    throw Error(`unexpected ${path}`)
  }
  try {
    await renderToString(createSSRApp({ setup() { editor = View.setup({ embedded: true, viewParams: {} }, { expose() {}, emit() {} }); return () => h('div') } }))
    editor.form.purpose_key = 'material_transfer'
    const first = editor.form.items[0]
    Object.assign(first, { item_type: 'finished_product', from_warehouse: 'finished_goods' })
    await editor.ensureTransferInventory('finished_goods')
    assert.deepEqual(editor.productOptionsForItem(first).map(p => p.name), ['曜石2.0'])
    editor.selectItemObject(first, 58)
    await new Promise(resolve => setImmediate(resolve))
    await nextTick()
    assert.equal(first.bom_spec_id, 69)
    assert.equal(first.bom_variant_id, 1206)
    assert.deepEqual(editor.itemBomSpecs(first).map(s => s.bom_spec_id), [69])
    first.from_warehouse = 'other'
    editor.handleSourceWarehouseChange(first)
    assert.equal(first.product_id, 0)
    await editor.ensureTransferInventory('other')
    assert.deepEqual(editor.productOptionsForItem(first).map(p => p.name), ['初晓'])
  } finally { delete globalThis.transferRuntimeAPI; delete globalThis.window }
})

test('transfer form selects a real historical specification and batch and writes only one quantity', async () => {
  let editor
  globalThis.window = { location: { origin: 'http://localhost' }, clearTimeout, setTimeout }
  globalThis.transferRuntimeAPI = async path => {
    if (String(path).startsWith('/api/stock/transfer-options')) return { rows: [{ item_type: 'finished_product', item_id: 58, item_name: '曜石', bom_spec_id: 69, bom_variant_id: 1206, spec_name: '旧 454g', inventory_unit: '袋', batch_id: 3, batch_code: 'FP-3', qty_units: 10, available_units: 7 }], has_next: false }
    throw Error(`unexpected ${path}`)
  }
  try {
    await renderToString(createSSRApp({ setup() { editor = View.setup({ embedded: true, viewParams: {} }, { expose() {}, emit() {} }); return () => h('div') } }))
    editor.form.purpose_key = 'material_transfer'
    const item = editor.form.items[0]
    Object.assign(item, { item_type: 'finished_product', from_warehouse: 'finished_goods', to_warehouse: 'shop' })
    await editor.ensureChoiceInventory('finished_goods')
    editor.selectTransferObject(item, 58)
    assert.equal(item.transfer_spec, 'bom:69:1206')
    assert.equal(item.batch_code, '')
    assert.equal(editor.transferSpecBalance(item, true), '7 件')
    item.batch_code = 'FP-3'
    editor.changeTransferBatch(item)
    item.quantity = 4
    const payload = editor.requestBody().items[0]
    assert.equal(payload.batch_code, 'FP-3')
    assert.equal(payload.bom_spec_id, 69)
    assert.equal(payload.bom_variant_id, 1206)
    assert.equal(payload.qty_units, 4)
    assert.equal(payload.qty_g, 0)
    assert.equal(payload.inventory_unit, '袋')
    item.from_warehouse = 'other'
    editor.changeTransferWarehouse(item)
    assert.equal(item.product_id, 0)
    assert.equal(item.batch_code, '')
    assert.equal(item.quantity, 0)
  } finally { delete globalThis.transferRuntimeAPI; delete globalThis.window }
})

test('weight material converts its stock unit using existing gram rounding', async () => {
  let editor
  globalThis.window = { location: { origin: 'http://localhost' }, clearTimeout, setTimeout }
  globalThis.transferRuntimeAPI = async () => ({ rows: [{ item_type: 'material', item_id: 8, item_name: '豆', inventory_unit: 'lb', batch_id: 8, batch_code: 'MB-8', qty_g: 1000, available_g: 1000 }], has_next: false })
  try {
    await renderToString(createSSRApp({ setup() { editor = View.setup({ embedded: true, viewParams: {} }, { expose() {}, emit() {} }); return () => h('div') } }))
    editor.form.purpose_key = 'material_transfer'
    const item = editor.form.items[0]
    Object.assign(item, { item_type: 'material', from_warehouse: 'raw_materials', to_warehouse: 'wip' })
    await editor.ensureChoiceInventory('raw_materials')
    editor.selectTransferObject(item, 8)
    item.batch_code = 'MB-8'; editor.changeTransferBatch(item); item.quantity = 1
    assert.deepEqual(editor.transferQuantity(item), { qty_g: 454, qty_units: 0 })
  } finally { delete globalThis.transferRuntimeAPI; delete globalThis.window }
})

test('company stock into a customer warehouse requires explicit ownership confirmation', async () => {
  let editor
  globalThis.window = { location: { origin: 'http://localhost' }, clearTimeout, setTimeout }
  globalThis.transferRuntimeAPI = async () => ({ rows: [{ item_type: 'finished_product', item_id: 58, item_name: '曜石', bom_spec_id: 69, bom_variant_id: 1167, spec_name: '454g', inventory_unit: '袋', batch_id: 52, batch_code: 'FP-52', owner_customer_id: 0, qty_units: 10, available_units: 10 }], has_next: false })
  try {
    await renderToString(createSSRApp({ setup() { editor = View.setup({ embedded: true, viewParams: {} }, { expose() {}, emit() {} }); return () => h('div') } }))
    editor.form.purpose_key = 'material_transfer'
    editor.warehouses.value = [{ code: 'customer74', name: '喜沁仓库', customer_id: 74 }]
    const item = editor.form.items[0]
    Object.assign(item, { item_type: 'finished_product', from_warehouse: 'finished_goods', to_warehouse: 'customer74' })
    await editor.ensureChoiceInventory('finished_goods')
    editor.selectTransferObject(item, 58)
    item.batch_code = 'FP-52'; editor.changeTransferBatch(item); item.quantity = 10
    assert.throws(() => editor.requestBody(), /确认.*货权/)
    item.target_owner_customer_id = 74
    assert.equal(editor.requestBody().items[0].target_owner_customer_id, 74)
  } finally { delete globalThis.transferRuntimeAPI; delete globalThis.window }
})
