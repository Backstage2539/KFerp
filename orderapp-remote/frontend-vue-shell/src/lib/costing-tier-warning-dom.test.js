import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { JSDOM } from 'jsdom'
import { compile } from '@vue/compiler-dom'
import * as selection from './product-price-list-selection.js'

const dom = new JSDOM('<div id="app"></div>')
for (const key of ['window', 'document', 'Node', 'Element', 'HTMLElement', 'SVGElement', 'Event']) globalThis[key] = dom.window[key]
const Vue = await import('vue')
const source = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const start = source.indexOf('<div class="product-spec-options">')
const template = source.slice(start, source.indexOf('<div v-if="visibleItemWarnings(row).length"', start))
const render = new Function('Vue', compile(template, { mode: 'function', prefixIdentifiers: true }).code)(Vue)
const viewFunction = name => {
  const start = source.indexOf(`function ${name}(`)
  return source.slice(start, source.indexOf('\n}\n', start) + 2)
}

test('tier warnings follow the exact selected product and spec despite colliding IDs', async () => {
  const row = Vue.ref({ parent_product_id: 52, sku_options: [
    { sku_id: 72, spec_label: '36g', missing: true },
    { sku_id: 219, spec_label: '1kg', missing: true },
  ] })
  const pdfProductSpecSelections = Vue.ref([{ parent_product_id: 72, sku_id: 1 }, { parent_product_id: 52, sku_id: 219 }])
  const bindings = {
    ...selection, pdfProductSpecSelections,
    priceListTemplateResolutionForItem: () => ({ pricing_mode: 'tier_template', tier_template_id: 1 }),
    priceTierTemplateByID: () => ({}),
    priceListTierTemplateCompatibilityForItem: item => ({ compatible: !item.missing, message: '规格缺失' }),
  }
  const names = ['itemProductID', 'isPdfProductSelected', 'isPdfProductSpecSelected', 'priceListProductTierTemplateWarning']
  const methods = new Function(...Object.keys(bindings), names.map(viewFunction).join('\n') + `\nreturn {${names.join(',')}}`)(...Object.values(bindings))
  const app = Vue.createApp({ render, setup: () => ({ row, ...selection, ...methods,
    priceListProductSpecPricingResolution: () => ({}), priceListProductFixedTiers: () => [],
    priceListProductSpecPricingWarning: () => '', togglePdfProductSpec: () => {},
  }) })
  try {
    app.mount(document.querySelector('#app'))
    await Vue.nextTick()
    let options = document.querySelectorAll('.product-spec-option')
    assert.equal(options[0].querySelector('.product-picker-tier-warning'), null)
    assert.match(options[1].querySelector('.product-picker-tier-warning').textContent, /规格缺失/)
    row.value.sku_options[1].missing = false
    await Vue.nextTick()
    options = document.querySelectorAll('.product-spec-option')
    assert.equal(options[1].querySelector('.product-picker-tier-warning'), null)
  } finally { app.unmount() }
})
