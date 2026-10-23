import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createSSRApp, h, reactive } from 'vue'
import { renderToString } from '@vue/server-renderer'

let sequence = 0
const source = { key: 'workstationView', label: '返回工位', params: { scope: 'all', workstation: '烘焙工位', work_order_id: 19, job_card_id: 21, focus: 'workstation_task' } }
const returnEvent = { key: source.key, params: source.params }
const document = (extra = {}) => ({
  id: 37, entry_no: 'SE-QA-37', status: 'draft', purpose: 'material_transfer_for_manufacture',
  work_order_id: 19, job_card_id: 21, work_order_no: 'WO-QA-19', return_source: 'workstation_task',
  items: [{ material_id: 85, item_type: 'material', item_name: '测试生豆', inventory_unit: 'kg', qty_g: 15570, quantity: 15.57, from_warehouse: 'raw_materials', to_warehouse: 'wip', batch_code: 'QA-85', frozen_picking: true }],
  ...extra,
})

async function component(relative, mocks) {
  const file = new URL(relative, import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'), { filename: file.pathname })
  const id = `stock-return-${++sequence}`
  const script = compileScript(descriptor, { id })
  const template = compileTemplate({ id, source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
  assert.deepEqual(template.errors, [])
  globalThis[id] = mocks
  let code = script.content.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/, `const { apiGet, apiSend } = globalThis[${JSON.stringify(id)}]`)
  code = code.replace(/import \{([^}]+)\} from ['"]\.\.\/api\/production(?:\.js)?['"]/g, (_, names) => names.split(',').map(name => `const ${name.trim()} = async () => { throw new Error('Unexpected production request') }`).join('\n'))
  code = code.replace(/import ProductionReturnLink from ['"][^'"]+\.vue['"]/, `const ProductionReturnLink = globalThis[${JSON.stringify(id)}].ReturnLink`)
  code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, 'const $1 = { render() { return null } }')
  code = code.replace('export default', 'const component =') + '\n' + template.code.replace('export function render', 'function render') + '\ncomponent.render = render; export default component;'
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  const View = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  delete globalThis[id]
  return View
}

async function harness({ returnSource = source, failSave = false, failSubmit = false } = {}) {
  const writes = [], events = []
  globalThis.window = { location: { origin: 'https://example.test' }, dispatchEvent: event => { events.push(event.detail); return true } }
  globalThis.CustomEvent = class { constructor(type, options) { this.type = type; this.detail = options.detail } }
  const mocks = {
    apiGet: async value => {
      const url = new URL(value, window.location.origin)
      if (url.pathname === '/api/materials') return { rows: [] }
      if (url.pathname === '/api/stock/warehouses') return { rows: [] }
      if (url.pathname === '/api/stock/material-balances') return { rows: [{ material_id: 85, book_qty: 100, available_qty: 100 }] }
      if (url.pathname === '/api/stock-documents') return { rows: [] }
      throw new Error(`Unexpected read ${value}`)
    },
    apiSend: async (url, options) => {
      writes.push({ url, ...options })
      if (url.includes('stock-document-preview')) return { document: document({ id: 0 }), availability: 'actionable', warnings: [] }
      if (url.endsWith('/submit')) {
        if (failSubmit) throw new Error('库存已变化，请核对')
        return document({ status: 'submitted', items: [{ ...document().items[0], qty_g: 60000, quantity: 60 }] })
      }
      if (failSave) throw new Error('保存暂时失败')
      return { ...document(), ...options.body, id: 37, items: options.body.items }
    },
  }
  mocks.ReturnLink = await component('../components/ProductionReturnLink.vue', mocks)
  const View = await component('../views/StockEntriesView.vue', mocks)
  const props = reactive({ embedded: true, viewParams: returnSource ? { return_navigation: returnSource } : {} })
  let state
  await renderToString(createSSRApp({ setup() { state = View.setup(props, { expose() {} }); return () => h('div') } }))
  const html = () => renderToString(createSSRApp({ ...View, setup: () => state }, props))
  return { state, props, writes, events, html, mocks }
}

test('normal, saved, submitted and failed stock drawers retain the in-memory return link', async () => {
  const { state, html } = await harness()
  for (const status of ['draft', 'submitted']) {
    state.applyDocument(document({ status }))
    assert.equal(((await html()).match(/← 返回工位/g) || []).length, 2)
  }
  state.applyDocument(document())
  state.drawerError.value = '库存已变化，请核对'
  assert.equal(((await html()).match(/← 返回工位/g) || []).length, 2)
  assert.doesNotMatch(await html(), /返回来源：|不能超过尚未领取的预留量/)
  assert.match(await html(), /不限制实际领料/)
})

test('saved source metadata cannot manufacture a return link after refresh or direct entry', async () => {
  const { state, html, events } = await harness({ returnSource: null })
  state.applyDocument(document())
  assert.doesNotMatch(await html(), /← 返回工位/)
  state.requestReturnToSource()
  assert.equal(events.length, 0)
})

test('unchanged saved drafts and posted documents return without inventory writes', async () => {
  const { state, events, writes } = await harness()
  for (const status of ['draft', 'submitted']) {
    state.applyDocument(document({ status }))
    assert.equal(state.hasUnsavedDraft.value, false)
    state.requestReturnToSource()
    assert.deepEqual(events.at(-1), returnEvent)
  }
  assert.equal(writes.length, 0)
})

test('unsaved quantities expose save, discard and continue; continue preserves 60kg', async () => {
  const { state, html, events, writes } = await harness()
  state.applyDocument(document())
  state.form.items[0].quantity = 60
  state.requestReturnToSource()
  assert.equal(state.returnDialogOpen.value, true)
  const markup = await html()
  for (const label of ['保存草稿并返回', '放弃修改并返回', '继续编辑']) assert.ok(markup.includes(label))
  state.continueEditing()
  assert.equal(state.returnDialogOpen.value, false)
  assert.equal(state.form.items[0].quantity, 60)
  assert.equal(events.length, 0)
  assert.equal(writes.length, 0)
})

test('save and return writes 60kg once as a draft, never submits, then restores the source task', async () => {
  const { state, events, writes } = await harness()
  state.applyDocument(document())
  state.form.items[0].quantity = 60
  state.requestReturnToSource()
  await state.saveDraftAndReturn()
  assert.equal(writes.length, 1)
  assert.equal(writes[0].method, 'PUT')
  assert.equal(writes[0].body.items[0].qty_g, 60000)
  assert.equal(writes[0].url.includes('/submit'), false)
  assert.deepEqual(events, [returnEvent])
})

test('failed save stays in the return dialog with every input intact', async () => {
  const { state, events, html } = await harness({ failSave: true })
  state.applyDocument(document())
  state.form.items[0].quantity = 60
  state.form.note = '一整袋'
  state.requestReturnToSource()
  await state.saveDraftAndReturn()
  assert.equal(state.returnDialogOpen.value, true)
  assert.equal(state.form.items[0].quantity, 60)
  assert.equal(state.form.note, '一整袋')
  assert.match(await html(), /保存暂时失败/)
  assert.equal(events.length, 0)
})

test('discard returns without saving; new generated previews also require a decision', async () => {
  const { state, writes, events } = await harness()
  state.applyDocument(document({ id: 0 }))
  state.requestReturnToSource()
  assert.equal(state.returnDialogOpen.value, true)
  state.discardDraftAndReturn()
  assert.deepEqual(events, [returnEvent])
  assert.equal(writes.length, 0)
})

test('workstation preview honors source and submitting never returns automatically', async () => {
  const { state, writes, events, html } = await harness({ failSubmit: true })
  await state.applyViewParams({ action: 'issue', work_order_id: 19, job_card_id: 21, return_source: 'workstation_task' })
  assert.equal(writes[0].body.return_source, 'workstation_task')
  state.form.items[0].quantity = 60
  await state.submitDocument()
  assert.equal(events.length, 0)
  assert.equal(state.form.items[0].quantity, 60)
  assert.match(await html(), /库存已变化，请核对/)
  assert.equal(((await html()).match(/← 返回工位/g) || []).length, 2)
})

test('successful posting preserves the return link and waits for an explicit return click', async () => {
  const { state, writes, events, html } = await harness()
  state.applyDocument(document())
  state.form.items[0].quantity = 60
  await state.submitDocument()
  assert.equal(writes.length, 2)
  assert.equal(writes[1].url, '/api/stock-documents/37/submit')
  assert.equal(state.form.status, 'submitted')
  assert.equal(events.length, 0)
  assert.equal(((await html()).match(/← 返回工位/g) || []).length, 2)
  state.requestReturnToSource()
  assert.deepEqual(events, [returnEvent])
})

test('the shared return button runs this page guard once and stays blocked while saving', async () => {
  const { state, events, writes, mocks } = await harness()
  state.applyDocument(document())
  state.form.items[0].quantity = 60
  const props = reactive({ source, disabled: false, beforeNavigate: state.requestReturnToSource })
  let link
  await renderToString(createSSRApp({ setup() { link = mocks.ReturnLink.setup(props, { expose() {} }); return () => h('div') } }))
  link.goBack()
  assert.equal(state.returnDialogOpen.value, true)
  assert.equal(events.length, 0)
  state.saving.value = true
  props.disabled = true
  link.goBack()
  state.continueEditing()
  state.discardDraftAndReturn()
  await state.saveDraftAndReturn()
  assert.equal(state.returnDialogOpen.value, true)
  assert.equal(events.length, 0)
  assert.equal(writes.length, 0)
})

test('background balance updates do not make a saved draft dirty, but deleted lines do', async () => {
  const { state } = await harness()
  state.applyDocument(document())
  state.form.items[0].source_balance = { book_qty: 65, available_qty: 45 }
  state.form.items[0].local_key = 999
  assert.equal(state.hasUnsavedDraft.value, false)
  state.form.items.splice(0, 1)
  assert.equal(state.hasUnsavedDraft.value, true)
})

test('the actual workstation action preserves current scope and source task', async () => {
  const { mocks, events } = await harness()
  const View = await component('../views/WorkstationView.vue', mocks)
  let state
  await renderToString(createSSRApp({ setup() { state = View.setup({ viewParams: {}, actor: {} }, { expose() {} }); return () => h('div') } }))
  state.selectedWorkstation.value = '烘焙工位'
  for (const scope of ['all', 'mine']) {
    state.scope.value = scope
    state.openStockAction({ work_order_id: 19, job_card_id: 21, work_order_no: 'WO-QA-19', workstation: '烘焙工位' }, 'issue')
    assert.deepEqual(events.at(-1).returnNavigation, { ...source, params: { ...source.params, scope } })
  }
})
