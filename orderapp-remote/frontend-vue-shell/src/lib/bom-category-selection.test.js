import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import vm from 'node:vm'
import { ref, computed, watch, nextTick } from 'vue'
import { businessGroupMoveAssignmentPayload } from './business-grouping.js'

const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
function fn(name) {
  const start = source.indexOf(`function ${name}(`)
  const begin = source.slice(start - 6, start) === 'async ' ? start - 6 : start
  const next = source.slice(start + 1).search(/\n(?:async )?function /)
  return source.slice(begin, start + 1 + next)
}
function setup() {
  const row = { id: 700, production_bom_id: 700, name: '红岩', business_group_id: 1, group_item_id: 12 }
  const state = {
    ref, computed, watch, businessGroupMoveAssignmentPayload,
    productionBomRows: ref([row]), productionBomVisibleRows: ref([row]), selectedBomRowKeys: ref(['bom:700']),
    productionBomCategoryMoveActive: ref(false), productionBomSelectedBusinessGroups: ref([{ id: 1 }]),
    loading: ref(false), error: ref(''), ok: ref(''),
    bomRecordFromRow: row => row, calls: [],
    apiSend: async (url, options) => state.calls.push({ url, ...options }),
    clearProductionBomBusinessGroupAssignment: async id => state.calls.push({ clear: id }),
    loadAll: async () => {},
  }
  state.mutate = async action => { state.loading.value = true; try { await action(); return true } catch(e) { state.error.value = e.message; return false } finally { state.loading.value = false } }
  const context = vm.createContext(state)
  const a = source.indexOf('const visibleMovableBomRows =')
  const b = source.indexOf('const selectedActiveBomRecordsForDeactivate =', a)
  const watcher = source.match(/watch\((?:\[productionBomVisibleRows, productionBomCategoryMoveActive\]|productionBomVisibleRows),[\s\S]*?\n\}\)/)[0]
  vm.runInContext(source.slice(a, b) + ['bomRowKey','isMovableBomRow','productionBomGroupID','productionBomGroupItemID','beginProductionBomCategoryMove','cancelProductionBomCategoryMove','handleProductionBomCategoryMoveTarget','moveSelectedProductBomsToGroup'].map(fn).join('\n') + '\n'+watcher, context)
  return { state, context, selected: () => vm.runInContext('selectedBomRecordsForMove.value',context) }
}

test('selected Hongyan remains movable after movement collapses every visible category', async () => {
  const ui = setup()
  ui.context.beginProductionBomCategoryMove()
  ui.state.productionBomVisibleRows.value = []
  await nextTick()
  assert.equal(ui.selected().length, 1)
  await ui.context.handleProductionBomCategoryMoveTarget({ group_id: 1, group_item_id: 13 })
  assert.equal(ui.state.calls.length, 1)
  assert.equal(ui.state.calls[0].body.object_id, 700)
  assert.equal(ui.state.calls[0].body.group_item_id, 13)
  assert.equal(ui.state.error.value, '')
  assert.equal(ui.state.productionBomCategoryMoveActive.value, false)
})

test('cancel retains checked BOMs until the workspace restores its visible rows', async () => {
  const ui = setup()
  ui.context.beginProductionBomCategoryMove()
  ui.state.productionBomVisibleRows.value = []
  await nextTick()
  ui.context.cancelProductionBomCategoryMove()
  await nextTick()
  ui.state.productionBomVisibleRows.value = ui.state.productionBomRows.value
  await nextTick()
  assert.deepEqual([...ui.state.selectedBomRowKeys.value], ['bom:700'])
  assert.equal(ui.state.calls.length, 0)
})

test('failed move preserves the selected BOM for a retry while collapsed', async () => {
  const ui = setup()
  ui.state.apiSend = async () => { throw new Error('保存失败') }
  ui.context.beginProductionBomCategoryMove()
  ui.state.productionBomVisibleRows.value = []
  await nextTick()
  await ui.context.handleProductionBomCategoryMoveTarget({ group_id: 1, group_item_id: 13 })
  assert.equal(ui.state.error.value, '保存失败')
  assert.equal(ui.selected().length, 1)
  assert.equal(ui.state.productionBomCategoryMoveActive.value, true)
})
