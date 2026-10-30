import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'http://localhost/', pretendToBeVisual: true })
for (const key of ['window', 'document', 'Node', 'Element', 'HTMLElement', 'SVGElement', 'Event', 'MouseEvent']) globalThis[key] = dom.window[key]
const { createApp, nextTick } = await import('vue')
const file = new URL('../components/OrderBatchStatusDialog.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
let code = compileScript(descriptor, { id: 'combined-dom-test', inlineTemplate: true }).content
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
const { default: Dialog } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const rows = [
  { order_id: 1, order_no: 'SO-1', customer_id: 7, edit_revision: 'v1', total_cents: 10000, paid_cents: 3000, due_cents: 7000, fully_paid: false },
  { order_id: 2, order_no: 'SO-2', customer_id: 7, edit_revision: 'v2', total_cents: 5000, paid_cents: 5000, due_cents: 0, fully_paid: true },
]
const response = (value, status = 200) => ({ ok: status >= 200 && status < 300, status, json: async () => value })
async function flush() { for (let i = 0; i < 4; i++) { await nextTick(); await new Promise(resolve => setImmediate(resolve)) } }
const button = text => [...document.querySelectorAll('button')].find(node => node.textContent.trim() === text)
function select(node, value) { node.value = value; node.dispatchEvent(new Event('change', { bubbles: true })) }
function input(node, value) { node.value = value; node.dispatchEvent(new Event('input', { bubbles: true })) }
function mount(ids = [1, 2]) {
  let updated = 0
  const app = createApp(Dialog, { selectedRows: ids.map(id => ({ id, customer: '测试客户' })), payStatuses: [], shipStatuses: [{ id: 4, name: '已发货' }], onUpdated: () => updated++ })
  app.mount(document.querySelector('#app'))
  return { app, updated: () => updated }
}
function reset(app) { app.unmount(); document.body.innerHTML = '<div id="app"></div>'; window.sessionStorage.clear() }

test('页面汇总预付款及已付款订单，阻止不足额并提交超额收款和选定发货状态', async () => {
  const previousFetch = globalThis.fetch
  const requests = []
  globalThis.fetch = async (url, options) => {
    const path = new URL(String(url)).pathname
    requests.push({ path, body: JSON.parse(options.body) })
    return path.endsWith('/preview') ? response({ orders: rows }) : response({ batch_id: 11, actual_cents: 8000, due_cents: 7000, unallocated_cents: 1000 })
  }
  const mounted = mount()
  try {
    await flush()
    const radio = document.querySelector('input[value="combined"]'); radio.checked = true; radio.dispatchEvent(new Event('change', { bubbles: true })); await flush()
    assert.match(document.querySelector('.combined-totals').textContent, /150\.00.*80\.00.*70\.00/s)
    assert.match(document.querySelectorAll('.batch-order')[1].textContent, /已付款，不重复收款/)
    const fields = document.querySelector('.combined-payment .payment-fields')
    input(fields.querySelector('input[type="number"]'), '69.99')
    select(fields.querySelector('select'), '微信支付'); await flush()
    assert.ok(button('确认合并付款').disabled)
    assert.match(document.querySelector('.combined-payment').textContent, /实际收款不足待收合计/)
    input(fields.querySelector('input[type="number"]'), '80')
    select(document.querySelector('.batch-targets select'), '4'); await flush()
    assert.match(document.querySelector('.combined-payment').textContent, /待分配/)
    button('确认合并付款').click(); await flush()
    const payment = requests.find(request => request.path.endsWith('/combined-payment')).body
    assert.equal(payment.actual_cents, 8000); assert.equal(payment.expected_due_cents, 7000); assert.equal(payment.ship_status_id, 4)
    assert.deepEqual(payment.orders, [{ order_id: 1, edit_revision: 'v1' }, { order_id: 2, edit_revision: 'v2' }])
    assert.match(document.querySelector('.combined-payment').textContent, /合并付款已完成.*待分配 ¥10\.00/s)
    assert.equal(mounted.updated(), 1)
  } finally { reset(mounted.app); globalThis.fetch = previousFetch }
})

test('网络不确定后重新打开锁定原请求；再次 403 也不能丢失重试信息', async () => {
  const previousFetch = globalThis.fetch
  const requests = []; let attempts = 0
  globalThis.fetch = async (url, options) => {
    const path = new URL(String(url)).pathname
    if (path.endsWith('/preview')) return response({ orders: rows })
    requests.push(JSON.parse(options.body))
    if (++attempts === 1) throw new Error('network')
    if (attempts === 2) return response({ error: '无权操作' }, 403)
    return response({ batch_id: 11, actual_cents: 7000, due_cents: 7000, unallocated_cents: 0 })
  }
  let mounted = mount()
  try {
    await flush()
    const radio = document.querySelector('input[value="combined"]'); radio.checked = true; radio.dispatchEvent(new Event('change', { bubbles: true })); await flush()
    const fields = document.querySelector('.combined-payment .payment-fields')
    input(fields.querySelector('input[type="number"]'), '70'); select(fields.querySelector('select'), '微信支付'); await flush()
    button('确认合并付款').click(); await flush()
    assert.match(document.querySelector('.combined-warning').textContent, /原订单：#1、#2/)
    mounted.app.unmount(); document.body.innerHTML = '<div id="app"></div>'
    mounted = mount([2]); await flush()
    assert.match(document.querySelector('.combined-warning').textContent, /原订单：#1、#2/)
    assert.equal(document.querySelector('.batch-orders'), null)
    assert.ok(document.querySelector('input[value="individual"]').disabled)
    button('重试原次合并付款').click(); await flush()
    assert.ok(button('重试原次合并付款'))
    assert.ok(window.sessionStorage.getItem('kferp.order-combined-payment.pending'))
    button('重试原次合并付款').click(); await flush()
    assert.deepEqual(requests[1], requests[0]); assert.deepEqual(requests[2], requests[0])
    assert.equal(Object.hasOwn(requests[0], 'ship_status_id'), false)
    assert.match(document.querySelector('.combined-payment').textContent, /订单：#1、#2/)
    assert.equal(window.sessionStorage.getItem('kferp.order-combined-payment.pending'), null)
  } finally { reset(mounted.app); globalThis.fetch = previousFetch }
})

test('首次提交冲突后须重新读取订单才能再次确认', async () => {
  const previousFetch = globalThis.fetch
  let version = 'v1'
  globalThis.fetch = async (url) => {
    const path = new URL(String(url)).pathname
    return path.endsWith('/preview') ? response({ orders: [{ ...rows[0], edit_revision: version }, rows[1]] }) : response({ error: '订单已变化' }, 409)
  }
  const mounted = mount()
  try {
    await flush()
    const radio = document.querySelector('input[value="combined"]'); radio.checked = true; radio.dispatchEvent(new Event('change', { bubbles: true })); await flush()
    const fields = document.querySelector('.combined-payment .payment-fields')
    input(fields.querySelector('input[type="number"]'), '70'); select(fields.querySelector('select'), '微信支付'); await flush()
    button('确认合并付款').click(); await flush()
    assert.match(document.querySelector('.combined-payment').textContent, /重新读取所有订单/)
    assert.ok(button('确认合并付款').disabled)
    assert.equal(window.sessionStorage.getItem('kferp.order-combined-payment.pending'), null)
    version = 'v2'; button('重新读取').click(); await flush()
    assert.equal(button('确认合并付款').disabled, false)
  } finally { reset(mounted.app); globalThis.fetch = previousFetch }
})

test('跨客户的选择在页面中不能确认付款', async () => {
  const previousFetch = globalThis.fetch
  globalThis.fetch = async () => response({ orders: [rows[0], { ...rows[1], customer_id: 8 }] })
  const mounted = mount()
  try {
    await flush()
    const radio = document.querySelector('input[value="combined"]'); radio.checked = true; radio.dispatchEvent(new Event('change', { bubbles: true })); await flush()
    assert.match(document.querySelector('.combined-payment').textContent, /仅支持同一客户/)
    assert.ok(button('确认合并付款').disabled)
  } finally { reset(mounted.app); globalThis.fetch = previousFetch }
})
