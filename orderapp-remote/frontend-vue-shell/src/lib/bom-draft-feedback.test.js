import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import vm from 'node:vm'
import { ref } from 'vue'
import { productionBomSpecTemplateReapplyStrategy, productionBomOutputIdentity, productionBomOutputPayload } from './bom.js'

test('changed output and template create a replacement from the selected draft', () => {
  assert.deepEqual(productionBomSpecTemplateReapplyStrategy('spec_group', [{ id: 10, status: 'published' }, { id: 11, status: 'draft' }], { outputChanged: true, sourceVersionID: 11 }), { mode: 'replacement', sourceVersionID: 11 })
  assert.deepEqual(productionBomSpecTemplateReapplyStrategy('spec_group', [{ id: 11, status: 'draft' }], { outputChanged: true, sourceVersionID: 11 }), { mode: 'convert', sourceVersionID: 0 })
})

const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
test('one in-flight copy selects and loads the returned draft', async () => {
  const calls = []; let complete
  const state = {
    canEditCurrentBomProduct: ref(true), selectedProductionBomVersion: ref({ id: 10 }),
    currentProductionBomID: ref(1), versionNote: ref(''), loading: ref(false), ok: ref(''),
    selectedProductionBomVersionID: ref(10),
    apiSend: async (url, options) => { calls.push({ url, ...options }); return new Promise(resolve => { complete = () => resolve({ id: 11 }) }) },
    loadProductionBomDetailForVersion: async (bom, version) => { calls.push({ bom, version }) },
  }
  state.mutate = async action => { state.loading.value = true; try { await action() } finally { state.loading.value = false } }
  const a = source.indexOf('async function copyVersionAsDraft(')
  const b = source.indexOf('\nasync function ', a + 1)
  const context = vm.createContext(state); vm.runInContext(source.slice(a, b), context)
  const first = context.copyVersionAsDraft(); context.copyVersionAsDraft()
  assert.equal(calls.length, 1)
  complete(); await first
  assert.equal(state.selectedProductionBomVersionID.value, 11)
  assert.deepEqual(calls.at(-1), { bom: 1, version: 11 })
  assert.match(state.ok.value, /已复制/)
})

function reapplySetup({ changedOutput = true, fail = false } = {}) {
  const calls = []
  const state = {
    loading: ref(false), canEditCurrentBomItems: ref(true), selectedProductionBomDraftVersion: ref({ id: 11 }),
    reapplySpecTemplateVersionID: ref(22), reapplyMainInputComponentType: ref('material'),
    reapplyMainInputMaterialID: ref(7), reapplyMainInputProductID: ref(0), reapplyMainInputBomSpecID: ref(0),
    reapplyMainInputReady: ref(true), error: ref(''), ok: ref(''),
    productionBomDetail: ref({ output_type: 'product', output_product_id: 1, specification_mode: 'spec_group' }),
    selectedProductionBomRecord: ref({ id: 1 }), currentProductionBomID: ref(1),
    versions: ref([{ id: 10, status: 'published' }, { id: 11, status: 'draft' }]),
    bomForm: { id: 1, name: 'Green BOM', output_type: 'product', output_id: changedOutput ? 2 : 1 },
    selectedProductionBomVersion: ref({ id: 11, output_qty: 1, output_unit: '袋' }), outputUnitCode: ref('袋'),
    selectedBomVariantID: ref(5), bomWorkspaceDirty: ref(true), bomWorkspaceSaveFailed: ref(false), productionBoms: ref([]), productionBomSpecTemplates: ref([]),
    productionBomSpecTemplateReapplyStrategy, productionBomOutputIdentity, productionBomOutputPayload,
    apiSend: async (url, options) => { calls.push({ url, body: options.body }); if (fail) throw new Error('published specification template version not found'); return { id: 88 } },
    loadAll: async () => {}, openEditProductionBomRecord: async row => calls.push({ opened: row.id }),
    loadProductionBomDetailForVersion: async (bom, version) => calls.push({ loaded: version }),
    loadProductionBomSpecTemplates: async () => [{ id: 2, versions: [{ id: 23, status: 'published' }] }],
  }
  state.mutate = async action => { state.loading.value = true; try { await action(); return true } catch (err) { state.error.value = err.message; return false } finally { state.loading.value = false } }
  const a = source.indexOf('async function reapplyProductionBomSpecTemplate(')
  const b = source.indexOf('\nfunction ', a + 1)
  const context = vm.createContext(state); vm.runInContext(source.slice(a, b), context)
  return { state, calls, run: () => context.reapplyProductionBomSpecTemplate() }
}

test('reapply submits the changed output, selected draft, template and material together', async () => {
  const ui = reapplySetup()
  await ui.run()
  assert.equal(ui.calls[0].url, '/api/production-boms/1/replacement-draft')
  assert.equal(ui.calls[0].body.source_version_id, 11)
  assert.equal(ui.calls[0].body.output_product_id, 2)
  assert.equal(ui.calls[0].body.spec_template_version_id, 22)
  assert.equal(ui.calls[0].body.main_input_material_id, 7)
  assert.equal(ui.calls.at(-1).opened, 88)
})

test('an archived template refreshes options and clears the unavailable selection without switching versions', async () => {
  const ui = reapplySetup({ changedOutput: false, fail: true })
  await ui.run()
  assert.match(ui.state.error.value, /已归档.*重新选择/)
  assert.equal(ui.state.reapplySpecTemplateVersionID.value, 0)
  assert.equal(ui.state.productionBomSpecTemplates.value[0].versions[0].id, 23)
  assert.equal(ui.calls.length, 1)
})
