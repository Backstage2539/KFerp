import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h, reactive, nextTick } from 'vue'
import { renderToString } from '@vue/server-renderer'

let sequence = 0
async function loadSetup(name, props, api = null) {
 const file = new URL(`../components/${name}.vue`, import.meta.url)
 const { descriptor } = parse(readFileSync(file, 'utf8'))
 let code = compileScript(descriptor, { id: name }).content
 if (api) {
  const key = `capacityTest${++sequence}`; globalThis[key] = api
  code = code.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/, `const { apiGet, apiSend } = globalThis.${key}`)
 }
 code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, 'const $1 = {}')
 code = code.replace(/from (['"])([^'"]+)\1/g, (_,q,path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
 const Component = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
 let state;const events=[]
 await renderToString(createSSRApp({ setup() { state=Component.setup(reactive(props),{expose(){},emit(...args){events.push(args)}});return()=>h('div') }}))
 return {state,events}
}

test('review can collapse without losing acknowledgement; edits invalidate old acknowledgement', async () => {
 const props={ detail:{status:'draft',items:[]}, rows:[],capacities:[],preview:{operation_coverage:[{status:'over'}]} }
 const {state}=await loadSetup('ProductionPlanCapacityWorkspace',props)
 assert.ok(state.reviewExpanded)
 state.overAcknowledged.value=true
 state.reviewExpanded.value=false
 assert.equal(state.readiness.value.can_confirm,true)
 assert.equal(state.overAcknowledged.value,true)
})

test('batch input emits unrounded user input and shared workstation editor is reusable', async () => {
 const {state,events}=await loadSetup('ProductionPlanCapacityWorkspace',{detail:{status:'draft',items:[]},rows:[],capacities:[],preview:{}})
 const row={planned_batch_count:5}
 state.changeBatchCount(row,{target:{value:'1.5'}})
 assert.deepEqual(events.at(-1),['batch-change',row,'1.5'])
 const source=readFileSync(new URL('../components/ManufacturingWorkstationEditor.vue',import.meta.url),'utf8')
 assert.match(source,/workstationId/)
 assert.match(source,/hasUnsavedChanges/)
 const view=readFileSync(new URL('../views/ManufacturingWorkstationsView.vue',import.meta.url),'utf8')
 assert.match(view,/<ManufacturingWorkstationEditor/)
})

test('saving either workstation or capacity preserves the other unsaved editor form', async () => {
 let station={id:7,name:'工位',status:'active',primary_employee_id:1,applicable_operation_ids:[9],backup_employee_ids:[],machine_hourly_cost:100}
 let capacity={id:8,workstation_id:7,name:'20kg',batch_size_qty:20,batch_size_unit:'kg',standard_minutes:25,status:'active'}
 const api={apiGet:async path=>({rows:path==='/api/manufacturing-workstations'?[station]:path==='/api/manufacturing-workstation-capacities'?[capacity]:[]}),apiSend:async(path,{body})=>{if(path==='/api/manufacturing-workstations')station={...body};else capacity={...body};return {...body}}}
 const {state,events}=await loadSetup('ManufacturingWorkstationEditor',{embedded:true,workstationId:7,capacityId:8},api)
 state.editWorkstation(station);state.editCapacity(capacity)
 state.capacityForm.batch_size_qty=12
 state.form.name='工位改名'
 await state.saveWorkstation()
 assert.equal(state.capacityForm.batch_size_qty,12)
 assert.equal(state.hasUnsavedChanges.value,true)
 assert.equal(events.at(-1)[0],'updated')
 state.form.note='工位未保存备注'
 await state.saveCapacity()
 assert.equal(state.form.note,'工位未保存备注')
 assert.equal(state.hasUnsavedChanges.value,true)
 assert.equal(capacity.batch_size_qty,12)
})

test('closing an unsaved workstation editor can keep or discard its form without mutating split rows', async () => {
 const previousWindow=globalThis.window
 try {
  globalThis.window={confirm:()=>false}
  const rows=[{planned_batch_count:6,planned_qty:120}]
  const {state}=await loadSetup('ProductionPlanCapacityWorkspace',{detail:{status:'draft',items:[]},rows,capacities:[],preview:{}})
  state.editingWorkstation.value={workstation_id:7}
  state.workstationEditor.value={hasUnsavedChanges:true}
  state.closeWorkstation()
  assert.equal(state.editingWorkstation.value.workstation_id,7)
  globalThis.window.confirm=()=>true
  state.closeWorkstation()
  assert.equal(state.editingWorkstation.value,null)
  assert.deepEqual(rows,[{planned_batch_count:6,planned_qty:120}])
 } finally {if(previousWindow===undefined)delete globalThis.window;else globalThis.window=previousWindow}
})

async function capacityEditorForMinutesTest(id) {
 const station = { id: 4, name: '色选机', status: 'active', applicable_operation_ids: [2], backup_employee_ids: [] }
 let capacity = { id, workstation_id: 4, name: '泰禾', code: 'taihe01', batch_size_qty: 1, batch_size_unit: 'kg', standard_minutes: 15, cost_method: 'time', status: 'active' }
 const requests = []
 const api = {
  apiGet: async path => ({ rows: path === '/api/manufacturing-workstations' ? [station] : path === '/api/manufacturing-workstation-capacities' ? [capacity] : [] }),
  apiSend: async (path, { body }) => { requests.push({ path, body }); capacity = { ...body, id: id || 9 }; return capacity },
 }
 const { state, events } = await loadSetup('ManufacturingWorkstationEditor', { embedded: true, workstationId: 4, capacityId: id }, api)
 state.editWorkstation(station)
 if (id) state.editCapacity(capacity)
 else { state.newCapacity(); state.capacityForm.name = '泰禾' }
 return { state, events, requests }
}

for (const id of [0, 9]) {
 test(`capacity ${id ? 'update' : 'create'} rejects invalid minutes without sending or rounding`, async () => {
  const { state, events, requests } = await capacityEditorForMinutesTest(id)
  for (const minutes of [0.1, -1, 'invalid', NaN, Infinity]) {
   state.capacityForm.standard_minutes = minutes
   await state.saveCapacity()
   assert.equal(requests.length, 0)
   assert.equal(state.error.value, '标准分钟/批必须为大于或等于 0 的整数')
   assert.equal(state.capacityForm.standard_minutes, minutes)
   assert.equal(state.hasUnsavedChanges.value, true)
   assert.equal(events.some(([name]) => name === 'updated'), false)
  }
 })

 test(`capacity ${id ? 'update' : 'create'} keeps zero, integer and empty-minute compatibility`, async () => {
  const { state, requests } = await capacityEditorForMinutesTest(id)
  for (const minutes of [0, 1, 15, '']) {
   state.capacityForm.standard_minutes = minutes
   await state.saveCapacity()
   assert.equal(requests.at(-1).body.standard_minutes, Number(minutes))
   assert.equal(state.capacityForm.standard_minutes, Number(minutes))
   assert.equal(state.error.value, '')
  }
  assert.equal(requests.length, 4)
 })
}

test('capacity submit blocks until actual WIP covers non-upstream demand', async () => {
 const {state}=await loadSetup('ProductionPlanCapacityWorkspace',{detail:{status:'draft',items:[],component_sources:[{component_name:'生豆',unit:'kg',required_g:10000,upstream_g:5000,wip_covered_g:9000},{component_name:'包装袋',unit:'件',required_units:3,wip_covered_units:3}]},rows:[],capacities:[],preview:{}})
 assert.equal(state.pendingMaterials.value.length, 1)
 assert.equal(state.pendingMaterials.value[0].missing_label, '1 kg')
})

test('capacity submit still blocks and identifies unposted transfer drafts when WIP is covered', async () => {
 const {state}=await loadSetup('ProductionPlanCapacityWorkspace',{detail:{status:'draft',items:[],component_sources:[],material_transfer_documents:[{id:10,entry_no:'SE-10',status:'draft',source_id:1},{id:10,entry_no:'SE-10',status:'draft',source_id:2},{id:11,entry_no:'SE-11',status:'submitted'}]},rows:[],capacities:[],preview:{}})
 assert.deepEqual(state.pendingTransferDocuments.value.map(row=>row.entry_no), ['SE-10'])
})
