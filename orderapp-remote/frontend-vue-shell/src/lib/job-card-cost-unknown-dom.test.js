import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { JSDOM } from 'jsdom'

const dom = new JSDOM('<html><body></body></html>', { url: 'http://127.0.0.1/' })
for (const key of ['window', 'document', 'HTMLElement', 'Element', 'SVGElement', 'Node', 'Event', 'CustomEvent']) globalThis[key] = dom.window[key]
const { createApp, nextTick } = await import('vue')

test('batch and piece job cards display unknown actual cost after reload', async () => {
  const file = new URL('../views/JobCardsView.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'))
  const rows = [
    { id: 1, work_order_id: 1, product_name: '批次加工', operation: '烘焙', actual_operation_cost: 15, actual_cost_unavailable: true },
    { id: 2, work_order_id: 2, product_name: '计件加工', operation: '包装', actual_operation_cost: 20, actual_cost_unavailable: true },
    { id: 3, work_order_id: 3, product_name: '已核算', operation: '烘焙', actual_operation_cost: 25, actual_cost_unavailable: false },
  ]
  globalThis.jobCardCostFixture = { apiGet: async () => ({ rows }) }
  let code = compileScript(descriptor, { id: 'job-card-cost-ui', inlineTemplate: true }).content
  code = code.replace(/import \{ apiGet \} from ['"][^'"]+['"]/, 'const { apiGet } = globalThis.jobCardCostFixture')
  code = code.replace(/import (ProductionExecutionHubDrawer|ProductionTopNav) from ['"][^'"]+['"]/g, 'const $1 = { render: () => null }')
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  const Component = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  const el = document.createElement('div'); document.body.appendChild(el)
  const app = createApp(Component, { embedded: true }); app.mount(el)
  try {
    await new Promise(resolve => setImmediate(resolve)); await nextTick()
    const rendered = [...el.querySelectorAll('tbody tr')]
    assert.equal(rendered.length, 3)
    for (const row of rendered.slice(0, 2)) {
      const actual = [...row.querySelectorAll('.record-grid span')].find(span => span.textContent.includes('实际工序成本'))
      assert.match(actual.textContent, /待核算/)
    }
    assert.match(rendered[2].textContent, /25\.00/)
  } finally { app.unmount(); el.remove(); delete globalThis.jobCardCostFixture }
})
