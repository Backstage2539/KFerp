import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'https://erp.qacoohee.com/app/' })
for (const key of ['window', 'document', 'navigator', 'Element', 'HTMLElement', 'SVGElement', 'Node', 'CustomEvent']) Object.defineProperty(globalThis, key, { value: dom.window[key], configurable: true, writable: true })
let resolveSecond
let calls = 0
globalThis.fetch = async () => {
  calls++
  if (calls === 1) return new Response(JSON.stringify({ settings: {}, report: { tax_exclusive_revenue: 76543.21, gross_profit: 50, operating_net_profit: 50 }, exceptions: [], daily: { available: true, days: [] } }), { status: 200 })
  return new Promise(resolve => { resolveSecond = resolve })
}

const file = new URL('../views/FinanceDashboardView.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
let code = compileScript(descriptor, { id: 'financeDashboardLoading', inlineTemplate: true }).content
const stub = `data:text/javascript,${encodeURIComponent("export default { template: '<div>DAILY PANEL</div>' }")}`
const imports = { vue: import.meta.resolve('vue'), './FinanceDailyPanel.vue': stub, '../api/finance.js': new URL('../api/finance.js', import.meta.url).href, '../lib/finance.js': new URL('./finance.js', import.meta.url).href }
code = code.replace(/from (['"])([^'"]+)\1/g, (match, quote, path) => imports[path] ? `from ${JSON.stringify(imports[path])}` : match)
const { default: Dashboard } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const { createApp, nextTick } = await import('vue')
const settle = async () => { await new Promise(resolve => setTimeout(resolve, 0)); await nextTick() }

test('changing month hides stale financial figures during loading and after an error', async () => {
  const app = createApp(Dashboard)
  app.mount('#app')
  try {
    await settle()
    assert.match(document.body.textContent, /76,543\.21/)
    assert.match(document.body.textContent, /DAILY PANEL/)
    const month = document.querySelector('input[type=month]')
    month.value = '2020-01'
    month.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
    month.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
    await settle()
    assert.doesNotMatch(document.body.textContent, /76,543\.21/)
    assert.doesNotMatch(document.body.textContent, /DAILY PANEL/)
    resolveSecond(new Response(JSON.stringify({ error: '查询失败' }), { status: 500 }))
    await settle()
    assert.match(document.body.textContent, /查询失败/)
    assert.doesNotMatch(document.body.textContent, /76,543\.21/)
    assert.doesNotMatch(document.body.textContent, /DAILY PANEL/)
  } finally { app.unmount() }
})

test('returning to the dashboard restores the selected historical month', async () => {
  calls = 0
  const app = createApp(Dashboard, { viewParams: { month: '2020-01', date: '2020-01-02' } })
  app.mount('#app')
  try {
    await settle()
    assert.equal(document.querySelector('input[type=month]').value, '2020-01')
  } finally { app.unmount() }
})
