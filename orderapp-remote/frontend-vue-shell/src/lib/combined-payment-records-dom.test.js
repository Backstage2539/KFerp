import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom = new JSDOM('<div id="app"></div>', { url: 'http://localhost/' })
for (const name of ['window', 'document', 'Node', 'Element', 'HTMLElement', 'SVGElement', 'Event']) globalThis[name] = dom.window[name]
const { createApp, nextTick } = await import('vue')
const file = new URL('../components/OrderCombinedPaymentRecords.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
let source = compileScript(descriptor, { id: 'combined-payment-records', inlineTemplate: true }).content
source = source.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
const { default: Records } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`)
const response = (value, status = 200) => ({ ok: status < 400, status, json: async () => value })
async function flush() { for (let i = 0; i < 4; i++) { await nextTick(); await new Promise(resolve => setTimeout(resolve, 2)) } }
function openRecords() { document.querySelector('details').open = true }

test('订单打开记录后显示本单分配与整笔待分配金额', async () => {
  const original = globalThis.fetch, calls = []
  globalThis.fetch = async url => { calls.push(String(url)); return response({ payments: [{ batch_id: 17, created_at: '2026-10-30', payment_method: '微信', actor: '员工', actual_cents: 12000, due_cents: 10000, unallocated_cents: 2000, allocations: [{ order_id: 12, amount_cents: 4000 }] }] }) }
  const app = createApp(Records, { orderId: 12 })
  try {
    app.mount(document.querySelector('#app')); await flush(); assert.equal(calls.length, 0)
    openRecords(); await flush()
    assert.match(calls[0], /\/api\/orders\/12\/combined-payments$/)
    assert.match(document.body.textContent, /本次实际收款 ¥120\.00/)
    assert.match(document.body.textContent, /本订单分配 ¥40\.00.*超额待分配 ¥20\.00/)
    assert.match(document.body.textContent, /不自动抵扣其他订单/)
  } finally { app.unmount(); globalThis.fetch = original; document.body.innerHTML = '<div id="app"></div>' }
})

test('无权限的记录请求显示原因并可重试', async () => {
  const original = globalThis.fetch; let calls = 0
  globalThis.fetch = async () => ++calls === 1 ? response({ error: 'forbidden' }, 403) : response({ payments: [] })
  const app = createApp(Records, { orderId: 13 })
  try {
    app.mount(document.querySelector('#app')); openRecords(); await flush()
    assert.match(document.querySelector('[role="alert"]').textContent, /无权查看/)
    document.querySelector('button').click(); await flush()
    assert.match(document.body.textContent, /暂无合并收款记录/)
  } finally { app.unmount(); globalThis.fetch = original; document.body.innerHTML = '<div id="app"></div>' }
})
