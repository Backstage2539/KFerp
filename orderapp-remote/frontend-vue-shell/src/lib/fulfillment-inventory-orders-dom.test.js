import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'http://localhost/', pretendToBeVisual: true })
for (const key of ['window', 'document', 'Node', 'Element', 'HTMLElement', 'SVGElement', 'Event', 'MouseEvent', 'CustomEvent']) globalThis[key] = dom.window[key]
const { createApp, h, nextTick, ref } = await import('vue')
const file = new URL('../components/FulfillmentInventory.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
let code = compileScript(descriptor, { id: file.pathname, inlineTemplate: true }).content
code = code.replace(/import (\w+) from ['"]([^'"]+\.vue)['"]/g, (_, name) => `const ${name}={render(){return null}}`)
code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
const component = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default

const stock = (sources = []) => ({ inventory_type:'finished_product', item_id:10, bom_spec_id:12, spec_g:0, item_name:'咖啡', unit:'袋', warehouses:[], batches:[], order_sources:sources })
test('库存明细显示各留厂订单付款状态并在数据刷新后更新，不混淆库存余量', async () => {
  const rows = ref([stock([{order_id:1,order_no:'SO-PAID',pay_status:'已付款',pending_inbound_qty:2,received_qty:4},{order_id:2,order_no:'SO-DUE',pay_status:'未付款',pending_inbound_qty:6,received_qty:0}])])
  const app = createApp({render:()=>h(component,{rows:rows.value})})
  try {
    app.mount('#app'); await nextTick()
    document.querySelector('.text-button').click(); await nextTick()
    let drawer=document.querySelector('.inventory-drawer')
    assert.match(drawer.textContent,/SO-PAID.*已付款.*2.*4/s)
    assert.match(drawer.textContent,/SO-DUE.*未付款.*6.*0/s)
    assert.match(drawer.textContent,/累计入库不是当前库存余量/)
    rows.value=[stock([{order_id:1,order_no:'SO-PAID',pay_status:'已付款',pending_inbound_qty:2,received_qty:4},{order_id:2,order_no:'SO-DUE',pay_status:'已付款',pending_inbound_qty:6,received_qty:0}])]
    await nextTick(); await nextTick()
    drawer=document.querySelector('.inventory-drawer')
    assert.match(drawer.textContent,/SO-DUE.*已付款/)
    rows.value=[stock()]; await nextTick(); await nextTick()
    assert.match(document.querySelector('.inventory-drawer').textContent,/无可关联订单/)
  } finally { app.unmount() }
})
