import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'https://erp.qacoohee.com/app/' })
for (const key of ['window', 'document', 'navigator', 'Element', 'HTMLElement', 'SVGElement', 'Node', 'CustomEvent']) Object.defineProperty(globalThis, key, { value: dom.window[key], configurable: true, writable: true })
const requests = []
globalThis.fetch = async url => {
  requests.push(new URL(url))
  return new Response(JSON.stringify({ rows: [{ date: '2020-01-02', section: 'main_cost', source_type: 'order', source_id: 7, name: '订单 7', link: '/app/vue-shell?view=orders&order_id=7', amount: null, cost_unavailable: true, pending_reason: '成本待核算' }], total: 21, page: 1, page_size: 20, available: true }), { status: 200 })
}
const file = new URL('../views/FinanceDailyPanel.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
let code = compileScript(descriptor, { id: 'financeDaily', inlineTemplate: true }).content
const imports = { vue: import.meta.resolve('vue'), '../api/finance.js': new URL('../api/finance.js', import.meta.url).href, '../lib/finance.js': new URL('./finance.js', import.meta.url).href, '../lib/finance-daily.js': new URL('./finance-daily.js', import.meta.url).href }
code = code.replace(/from (['"])([^'"]+)\1/g, (match, quote, path) => imports[path] ? `from ${JSON.stringify(imports[path])}` : match)
const { default: Panel } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const { createApp, h, nextTick } = await import('vue')
const settle = async () => { await new Promise(resolve => setTimeout(resolve, 0)); await nextTick() }

test('daily chart filters ledger by selected day and source navigation preserves return context', async () => {
  const app = createApp({ render: () => h(Panel, { month: '2020-01', daily: { available: true, days: [{ date: '2020-01-01', revenue: 100, known_cost: 60, period_expenses: 0, cost_expenses: 60, profit: 40 }, { date: '2020-01-02', revenue: 20, known_cost: 5, period_expenses: 2, cost_expenses: null, profit: null, cost_unavailable: true }] } }) })
  app.mount('#app')
  try {
    await settle()
    assert.match(document.body.textContent, /成本待核算/)
    assert.equal(document.querySelector('svg.chart').getAttribute('role'), 'group')
    assert.equal(document.querySelectorAll('svg polyline').length, 3)
    const day = document.querySelector('[aria-label="2020-01-02 查看流水"]')
    assert.ok(day)
    day.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    await settle()
    assert.equal(day.getAttribute('aria-pressed'), 'true')
    assert.equal(requests.at(-1).searchParams.get('date'), '2020-01-02')
    const summary = document.querySelector('[aria-label="2020-01-02 经营摘要"]')
    assert.match(summary.textContent, /不含税收入.*20\.00/); assert.match(summary.textContent, /已知销售成本.*5\.00/); assert.match(summary.textContent, /期间费用.*2\.00/); assert.match(summary.textContent, /毛利.*待核算/); assert.match(summary.textContent, /税前经营利润.*待核算/)
    ;[...document.querySelectorAll('button')].find(button => button.textContent === '下一页').click()
    await settle()
    assert.equal(requests.at(-1).searchParams.get('page'), '2')
    const filter = document.querySelector('select')
    filter.value = 'main_cost'
    filter.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
    await settle()
    assert.equal(requests.at(-1).searchParams.get('section'), 'main_cost')
    assert.equal(requests.at(-1).searchParams.get('page'), '1')
    document.querySelector('input[type=checkbox]').click()
    await settle()
    assert.equal(requests.at(-1).searchParams.get('unavailable'), 'true')
    let nav
    window.addEventListener('kferp:navigate-view', event => { nav = event.detail }, { once: true })
    ;[...document.querySelectorAll('button')].find(button => button.textContent === '订单 7').click()
    assert.equal(nav.key, 'orders')
    assert.equal(nav.params.order_id, '7')
    assert.equal(nav.returnNavigation.key, 'financeDashboard')
    assert.deepEqual(nav.returnNavigation.params, { month: '2020-01', date: '2020-01-02' })
    document.querySelector('[aria-label="2020-01-01 查看流水"]').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    await settle()
    assert.match(document.querySelector('[aria-label="2020-01-01 经营摘要"]').textContent, /毛利.*40\.00/)
  } finally { app.unmount() }
})

test('returning to a dated ledger restores the selected date', async () => {
  requests.length = 0
  const app = createApp({ render: () => h(Panel, { month: '2020-01', initialDate: '2020-01-02', daily: { available: true, days: [{ date: '2020-01-02', revenue: 20, known_cost: 5, period_expenses: 2, cost_expenses: 7, profit: 13 }] } }) })
  app.mount('#app')
  try {
    await settle()
    assert.equal(document.querySelector('input[type=date]').value, '2020-01-02')
    assert.equal(requests.at(-1).searchParams.get('date'), '2020-01-02')
    assert.match(document.querySelector('[aria-label="2020-01-02 经营摘要"]').textContent, /税前经营利润.*13\.00/)
  } finally { app.unmount() }
})
