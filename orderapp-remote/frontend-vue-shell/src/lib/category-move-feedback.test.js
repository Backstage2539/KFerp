import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import vm from 'node:vm'
import { ref } from 'vue'
import { businessGroupMoveAssignmentPayload } from './business-grouping.js'

const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
function functionSource(name) {
  const start = source.search(new RegExp(`(?:async )?function ${name}\\(`))
  if (start < 0) return ''
  const rest = source.slice(start)
  const end = rest.slice(1).search(/\n(?:async )?function /)
  return end < 0 ? rest : rest.slice(0, end + 1)
}
function setup(ids = [1], failures = []) {
  const calls = []
  const state = {
    catalogCustomerID: ref(0), productCatalogBusinessGroups: ref([{ id: 2 }]),
    selectedProductIds: ref(ids), loading: ref(false), error: ref(''), ok: ref(''),
    productCategoryMoveActive: ref(true),
    businessGroupAssignments: ref(ids.map(id => ({ id: 100 + id, object_id: id, object_key: 'product', usage_key: 'product_catalog', group_id: 2, group_item_id: 20 }))),
    businessGroupMoveAssignmentPayload,
    window: { location: { origin: 'http://localhost' } }, URL,
    apiSend: async (url, options) => {
      calls.push({ url, ...options })
      if (failures.includes(options?.body?.object_id)) throw new Error('测试保存失败')
      return { assignment: { id: 200 + Number(options?.body?.object_id || 0), ...options?.body } }
    },
    apiGet: async () => { throw new Error('move must use existing assignment IDs') },
    // An unrelated read never completes: success must not wait for it.
    loadAll: () => { calls.push({ url: 'loadAll' }); return new Promise(() => {}) },
  }
  const context = vm.createContext(state)
  vm.runInContext([
    'saveSelectedProductBusinessGroupAssignment', 'clearProductBusinessGroupAssignment',
    'handleProductCategoryMoveTarget', 'applyProductBusinessGroupAssignment',
  ].map(functionSource).join('\n'), context)
  return { state, calls, move: target => context.handleProductCategoryMoveTarget(target), cancel: () => {
    const event = source.match(/@cancel="([^"]*productCategoryMoveActive[^"]*)"/)[1]
    vm.runInContext(event.replace('productCategoryMoveActive =', 'productCategoryMoveActive.value ='), context)
  } }
}
const target = { group_id: 2, group_item_id: 21 }
const flush = () => new Promise(resolve => setImmediate(resolve))

test('category response updates rows and feedback without waiting for unrelated reads', async () => {
  const ui = setup()
  ui.move(target)
  await flush()
  assert.equal(ui.state.loading.value, false)
  assert.equal(ui.state.productCategoryMoveActive.value, false)
  assert.match(ui.state.ok.value, /已移动 1/)
  assert.equal(ui.state.businessGroupAssignments.value[0].group_item_id, 21)
  assert.equal(ui.calls.some(call => call.url === 'loadAll'), false)
})

test('moving to unclassified removes only confirmed assignments without a discovery reload', async () => {
  const ui = setup()
  await ui.move({ unclassified: true })
  assert.equal(ui.calls[0].url, '/api/business-group-assignments/101')
  assert.equal(ui.calls[0].method, 'DELETE')
  assert.equal(ui.state.businessGroupAssignments.value.length, 0)
  assert.equal(ui.state.selectedProductIds.value.length, 0)
})

test('batch partial failure retains failed selections and applies successful moves', async () => {
  const ui = setup([1, 2, 3], [2])
  ui.move(target); await flush()
  assert.deepEqual([...ui.state.selectedProductIds.value], [2])
  assert.deepEqual(Array.from(ui.state.businessGroupAssignments.value, row => row.group_item_id), [21, 20, 21])
  assert.match(ui.state.error.value, /1.*失败|失败.*1/)
  assert.match(ui.state.ok.value, /2/)
  assert.equal(ui.state.productCategoryMoveActive.value, true)
})

test('duplicate clicks are ignored while a category write is pending', async () => {
  const ui = setup()
  let complete
  ui.state.apiSend = (url, options) => {
    ui.calls.push({ url, ...options })
    return new Promise(resolve => { complete = () => resolve({ assignment: { id: 201, ...options.body } }) })
  }
  ui.move(target); ui.move(target)
  assert.equal(ui.calls.length, 1)
  assert.match(ui.state.ok.value, /正在移动/)
  complete(); await flush()
  assert.equal(ui.state.loading.value, false)
})

test('cancel before choosing a target keeps selections and assignments without a request', () => {
  const ui = setup([1, 2])
  ui.cancel()
  assert.equal(ui.state.productCategoryMoveActive.value, false)
  assert.deepEqual([...ui.state.selectedProductIds.value], [1, 2])
  assert.deepEqual(Array.from(ui.state.businessGroupAssignments.value, row => row.group_item_id), [20, 20])
  assert.equal(ui.calls.length, 0)
})
