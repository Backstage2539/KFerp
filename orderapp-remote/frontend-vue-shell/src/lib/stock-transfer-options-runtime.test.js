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
