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
const start = source.indexOf('<div v-if="productSpecSelectionIssueForFamily(row)"')
const template = source.slice(start, source.indexOf('<div v-if="visibleItemWarnings(row).length"', start))
const render = new Function('Vue', compile(template, { mode: 'function', prefixIdentifiers: true }).code)(Vue)
const viewFunction = name => {
  const start = source.indexOf(`function ${name}(`)
  return source.slice(start, source.indexOf('\n}\n', start) + 2)
}

test('real picker markup shows the current default badge while preserving old choices and normal checkbox interaction', async () => {
  const row = Vue.ref({ parent_product_id: 1, default_sku_id: 12, sku_options: [
    { sku_id: 12, bom_spec_id: 12, spec_label: '227g袋装' },
    { sku_id: 11, bom_spec_id: 11, spec_label: '454g袋装', bom_version_id: 2, bom_variant_id: 22 },
  ] })
  const pdfProductSpecSelections = Vue.ref(selection.normalizePriceListProductSpecSelections([
    { parent_product_id: 1, sku_id: 11, bom_spec_id: 11, bom_version_id: 1, bom_variant_id: 21,
      selection_source: 'product_default', default_sku_id_at_selection: 11, selection_issue: 'default_bom_changed' },
  ], [row.value]))
  const bindings = { ...selection, pdfProductSpecSelections, priceListPricingPopover: Vue.ref({ type: '' }), closePriceListPricingPopover() {},
    updatePdfProductSpecSelections: rows => { pdfProductSpecSelections.value = selection.normalizePriceListProductSpecSelections(rows, [row.value]) } }
  const methods = new Function(...Object.keys(bindings), ['isPdfProductSpecSelected', 'productSpecSelectionIssueForFamily', 'resolveProductSpecSelectionIssue', 'togglePdfProductSpec'].map(viewFunction).join('\n') + '\nreturn {isPdfProductSpecSelected,productSpecSelectionIssueForFamily,resolveProductSpecSelectionIssue,togglePdfProductSpec}')(...Object.values(bindings))
  const app = Vue.createApp({ render, setup: () => ({ row, ...selection, ...methods,
    priceListProductSpecPricingResolution: () => ({}), priceListProductFixedTiers: () => [],
    priceListProductSpecPricingWarning: () => '', priceListProductTierTemplateWarning: () => '',
  }) })
  try {
    app.mount(document.querySelector('#app'))
    await Vue.nextTick()
    const choices = document.querySelectorAll('.product-spec-check')
    assert.equal(choices.length, 2)
    assert.match(choices[0].textContent, /227g袋装.*默认规格/s)
    assert.equal(choices[0].querySelector('input').checked, false)
    assert.equal(choices[1].querySelector('input').checked, true)
    assert.equal(document.querySelector('.product-spec-selection-warning'), null)
    choices[0].querySelector('input').click()
    await Vue.nextTick()
    assert.deepEqual(pdfProductSpecSelections.value.map(s => s.sku_id), [12, 11])
    row.value = { ...row.value, sku_options: [row.value.sku_options[0]] }
    bindings.updatePdfProductSpecSelections(pdfProductSpecSelections.value)
    await Vue.nextTick()
    assert.match(document.querySelector('.product-spec-selection-warning').textContent, /已停用、被移除或不属于当前商品/)
  } finally { app.unmount() }
})
