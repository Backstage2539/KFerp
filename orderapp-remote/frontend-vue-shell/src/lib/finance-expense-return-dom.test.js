import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'https://erp.qacoohee.com/app/' })
for (const key of ['window', 'document', 'navigator', 'Element', 'HTMLElement', 'SVGElement', 'Node', 'CustomEvent']) Object.defineProperty(globalThis, key, { value: dom.window[key], configurable: true, writable: true })
const requests = []
globalThis.fetch = async url => {
  const parsed = new URL(url); requests.push(parsed)
  return new Response(JSON.stringify(parsed.pathname.endsWith('/employees') ? { rows: [] } : { rows: [{ id: 17, date: '2020-01-02', category: '运费', amount: 8, allocation: 'period_expense' }] }), { status: 200 })
}

function compile(file, replacements = {}) {
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
  let code = compileScript(descriptor, { id: file.pathname, inlineTemplate: true }).content
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(replacements[path] || (path.startsWith('.') ? new URL(path, file).href : import.meta.resolve(path)))}`)
  return `data:text/javascript;base64,${Buffer.from(code).toString('base64')}`
}
const returnLink = compile(new URL('../components/ProductionReturnLink.vue', import.meta.url))
const { default: Expenses } = await import(compile(new URL('../views/FinanceExpensesView.vue', import.meta.url), { '../components/ProductionReturnLink.vue': returnLink }))
const { createApp, nextTick } = await import('vue')
const settle = async () => { await new Promise(resolve => setTimeout(resolve, 0)); await nextTick() }

test('expense source opens the selected month and highlights the row, then returns to the selected finance date', async () => {
  const app = createApp(Expenses, { viewParams: { month: '2020-01', expense_id: '17', return_navigation: { key: 'financeDashboard', label: '返回财务首页', params: { month: '2020-01', date: '2020-01-02' } } } })
  app.mount('#app')
  try {
    await settle(); await settle()
    assert.equal(document.querySelector('input[type=month]').value, '2020-01')
    assert.equal(requests.find(request => request.pathname.endsWith('/expenses'))?.searchParams.get('month'), '2020-01')
    assert.match(document.querySelector('tr.source-expense').textContent, /运费.*来源费用/)
    let detail
    window.addEventListener('kferp:navigate-view', event => { detail = event.detail }, { once: true })
    document.querySelector('.production-return').click()
    assert.equal(detail.key, 'financeDashboard')
    assert.deepEqual(detail.params, { month: '2020-01', date: '2020-01-02' })
  } finally { app.unmount() }
})
