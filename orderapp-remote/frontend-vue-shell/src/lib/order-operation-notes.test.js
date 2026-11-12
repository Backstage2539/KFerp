import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {parse,compileScript,compileTemplate} from '@vue/compiler-sfc'
import {createSSRApp,h,reactive,nextTick} from 'vue'
import {renderToString} from '@vue/server-renderer'

test('order note editor fetches available steps and keeps user-entered notes', () => {
  const src=readFileSync(new URL('../components/OrderOperationNotes.vue',import.meta.url),'utf8')
  const {descriptor,errors}=parse(src)
  assert.deepEqual(errors,[])
  assert.match(src,/apiGet/)
  assert.match(src,/update:modelValue/)
  assert.match(src,/operation_name/)
  assert.match(src,/工序备注/)
  assert.ok(descriptor.template)
})

async function noteEditor(props) {
  const file=new URL('../components/OrderOperationNotes.vue',import.meta.url)
  const {descriptor}=parse(readFileSync(file,'utf8'))
  const script=compileScript(descriptor,{id:'order-notes-test'})
  const template=compileTemplate({id:'order-notes-test',source:descriptor.template.content,filename:file.pathname,compilerOptions:{bindingMetadata:script.bindings}})
  assert.deepEqual(template.errors,[])
  let code=script.content.replace(/import \{ apiGet \} from '\.\.\/api\/client'/,"const apiGet=(...args)=>globalThis.__operationNotesApi(...args)")
  code=code.replace('export default','const component =')+'\n'+template.code.replace('export function render','function render')+'\ncomponent.render=render;export default component;'
  code=code.replace(/from (['"])([^'"]+)\1/g,(_,quote,path)=>`from ${JSON.stringify(import.meta.resolve(path))}`)
  const View=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  let state
  await renderToString(createSSRApp({setup(){state=View.setup(props,{expose(){},emit(name,value){if(name==='update:modelValue')props.modelValue=value;if(name==='update:context')props.context=value}});return()=>h('div')}}))
  return {state,View}
}
const first={output_type:'material',output_id:8,output_name:'曲奇半成品',route_id:3,step_id:4,operation_id:5,operation_name:'烘焙'}
const second={output_type:'product',output_id:9,output_name:'曲奇 1kg',route_id:6,step_id:7,operation_id:8,operation_name:'包装'}

test('operation notes retain multiple inputs, explicit empty text, and explicit clearing',async()=>{
  globalThis.__operationNotesApi=async()=>({options:[first,second]})
  const props=reactive({modelValue:undefined,optionsUrl:'/options',disabled:false})
  const {state}=await noteEditor(props)
  try {
    await state.loadOptions()
    assert.equal(props.modelValue,undefined,'reading candidates must not edit the order')
    state.addNote()
    state.updateNote(0,'浅烘，保留花香')
    state.addNote()
    state.updateNote(1,'')
    assert.equal(props.modelValue.length,2)
    assert.equal(props.modelValue[0].note,'浅烘，保留花香')
    assert.equal(props.modelValue[1].note,'')
    assert.equal(props.modelValue[1].step_id,7)
    state.removeNote(0)
    state.removeNote(0)
    assert.deepEqual(props.modelValue,[])
  } finally {delete globalThis.__operationNotesApi}
})

test('different BOM specifications for the same output and route remain distinct note targets',async()=>{
  const options=[{...second,bom_spec_id:31,bom_variant_id:311},{...second,bom_spec_id:32,bom_variant_id:322}]
  globalThis.__operationNotesApi=async()=>({options})
  const props=reactive({modelValue:undefined,optionsUrl:'/options',disabled:false})
  const {state}=await noteEditor(props)
  try {
    await state.loadOptions()
    assert.notEqual(state.optionKey(options[0]),state.optionKey(options[1]))
    state.addNote();state.updateNote(0,'1kg贴标签')
    state.addNote();state.updateNote(1,'227g不贴标签')
    assert.deepEqual(props.modelValue.map(note=>[note.bom_spec_id,note.bom_variant_id,note.note]),[[31,311,'1kg贴标签'],[32,322,'227g不贴标签']])
  } finally {delete globalThis.__operationNotesApi}
})

test('candidate failure or product change preserves saved and unsaved operation instructions',async()=>{
  const saved={...first,note:'手写输入'}
  const props=reactive({modelValue:[saved],optionsUrl:'/options',disabled:false})
  const {state,View}=await noteEditor(props)
  globalThis.__operationNotesApi=async()=>{throw new Error('工序读取失败')}
  try {
    await state.loadOptions()
    assert.equal(props.modelValue[0].note,'手写输入')
    assert.match(state.error.value,/读取失败/)
    props.optionsUrl='/different-product'
    await nextTick()
    globalThis.__operationNotesApi=async()=>({options:[second]})
    await state.loadOptions()
    assert.equal(props.modelValue[0].step_id,first.step_id)
    const html=await renderToString(createSSRApp({...View,setup:()=>state},props))
    assert.match(html,/手写输入/)
    assert.match(html,/原工序/)
    state.changeTarget(0,state.optionKey(second))
    assert.equal(props.modelValue[0].note,'手写输入')
    assert.equal(props.modelValue[0].step_id,second.step_id)
  } finally {delete globalThis.__operationNotesApi}
})

test('changing product or customer cannot silently submit old notes even when a route is shared',async()=>{
  const props=reactive({modelValue:[{...first,note:'甲客户浅烘'}],context:'customer1:product1',contextKey:'customer1:product1',optionsUrl:'/options',disabled:false})
  globalThis.__operationNotesApi=async()=>({options:[first,second]})
  const {state}=await noteEditor(props)
  try {
    assert.equal(await state.validate(),true)
    props.contextKey='customer2:product2'
    assert.equal(await state.validate(),false)
    assert.match(state.validationError.value,/商品|客户/)
    assert.equal(props.modelValue[0].note,'甲客户浅烘')
    state.confirmContext()
    assert.equal(await state.validate(),true)
    assert.equal(props.context,'customer2:product2')
    globalThis.__operationNotesApi=async()=>({options:[second]})
    assert.equal(await state.validate(),false)
    assert.equal(props.modelValue[0].note,'甲客户浅烘')
    globalThis.__operationNotesApi=async()=>{throw new Error('读取失败')}
    assert.equal(await state.validate(),false)
  } finally {delete globalThis.__operationNotesApi}
})

test('ERP and customer order-entry preserve operation notes when loading or changing price specs',()=>{
  const src=readFileSync(new URL('../views/OrderEntryView.vue',import.meta.url),'utf8')
  const identity=readFileSync(new URL('./order-operation-notes.js',import.meta.url),'utf8')
  assert.match(src,/<OrderOperationNotes[\s\S]*?v-model="row.operation_notes"/)
  assert.match(identity,/\/api\/order-operation-options/)
  assert.match(identity,/\/api\/customer-processing\/portal\/order\/operation-options/)
  assert.match(src,/operation_notes:.*item\.operation_notes/)
  assert.match(src,/operation_notes: row\.operation_notes/)
})

test('operation candidates use the exact parent product and frozen spec identity sent on order save',async()=>{
  const {orderOperationOptionsURL,orderOperationNotesContext}=await import('./order-operation-notes.js')
  const {buildOrderPayload}=await import('./order-entry.js')
  const row={product_id:123,parent_product_id:8,migration_state:'cutover',bom_spec_id:21,bom_variant_id:22,qty:1,unit:'袋',spec_g:1000}
  const payload=buildOrderPayload({form:{customer_id:42},rows:[row]})
  const staff=new URL(orderOperationOptionsURL(row), 'http://localhost')
  assert.equal(staff.searchParams.get('product_id'),payload.product_id[0])
  assert.equal(staff.searchParams.get('bom_spec_id'),payload.bom_spec_id[0])
  assert.equal(staff.searchParams.get('bom_variant_id'),payload.bom_variant_id[0])
  assert.equal(orderOperationNotesContext(row,42),'42:8:21:22')
  const portal=new URL(orderOperationOptionsURL(row,{customerPortal:true,customerId:42,portalService:'direct_ship'}),'http://localhost')
  assert.equal(portal.pathname,'/api/customer-processing/portal/order/operation-options')
  assert.equal(portal.searchParams.get('customer_id'),'42')
  assert.equal(portal.searchParams.get('service'),'direct_ship')
  assert.equal(portal.searchParams.get('product_id'),payload.product_id[0])
  const legacy={...row,migration_state:'legacy'}
  assert.equal(new URL(orderOperationOptionsURL(legacy),'http://localhost').searchParams.get('product_id'),'123')
  assert.equal(new URL(orderOperationOptionsURL(legacy),'http://localhost').searchParams.get('bom_spec_id'),'0')
})

test('mall attaches per-product operation notes and only clears them after successful submit',()=>{
  const src=readFileSync(new URL('../views/CustomerMallView.vue',import.meta.url),'utf8')
  assert.match(src,/<OrderOperationNotes/)
  assert.match(src,/\/api\/customer-processing\/portal\/mall\/operation-options/)
  assert.match(src,/operation_notes: operationNotes\[p.id\]/)
  const failure=src.slice(src.indexOf('} catch (e)',src.indexOf('async function submit')))
  assert.doesNotMatch(failure,/delete operationNotes/)
})

test('mall submits distinct product instructions and retains all inputs after a rejected order',async()=>{
  const file=new URL('../views/CustomerMallView.vue',import.meta.url)
  const {descriptor}=parse(readFileSync(file,'utf8'))
  const script=compileScript(descriptor,{id:'mall-notes-test'})
  let code=script.content
    .replace("import { apiGet, apiSend } from '../api/client'",'const apiGet=async()=>({});const apiSend=(...args)=>globalThis.__mallNotesApi(...args)')
    .replace("import OrderOperationNotes from '../components/OrderOperationNotes.vue'",'const OrderOperationNotes={}')
    .replace(/from (['"])([^'"]+)\1/g,(_,quote,path)=>`from ${JSON.stringify(import.meta.resolve(path))}`)
  const View=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  let state
  await renderToString(createSSRApp({setup(){state=View.setup({customerId:22},{expose(){}});return()=>h('div')}}))
  state.products.value=[{id:1,product_id:8,bom_spec_id:21,bom_variant_id:22,sales_units:['bag']},{id:2,product_id:9,bom_spec_id:31,bom_variant_id:32,sales_units:['bag']}]
  state.counts[1]=2
  state.counts[2]=3
  state.operationNotes[1]=[{...first,note:'保留原文'}]
  state.operationNotes[2]=[]
  let submitted
  globalThis.__mallNotesApi=async(path,{body})=>{submitted=body;throw new Error('工序版本已变化')}
  try {
    await state.submit()
    assert.deepEqual(submitted.items.map(({mall_product_id,product_id,bom_spec_id,bom_variant_id})=>({mall_product_id,product_id,bom_spec_id,bom_variant_id})),[
      {mall_product_id:1,product_id:8,bom_spec_id:21,bom_variant_id:22},
      {mall_product_id:2,product_id:9,bom_spec_id:31,bom_variant_id:32},
    ])
    assert.equal(submitted.items[0].operation_notes[0].note,'保留原文')
    assert.deepEqual(submitted.items[1].operation_notes,[])
    assert.equal(state.counts[1],2)
    assert.equal(state.operationNotes[1][0].note,'保留原文')
    assert.match(state.error.value,/工序版本已变化/)
    globalThis.__mallNotesApi=async()=>({order_no:'SO-TEST'})
    await state.submit()
    assert.deepEqual(Object.keys(state.operationNotes),[])
    assert.deepEqual(Object.keys(state.counts),[])
  } finally {delete globalThis.__mallNotesApi}
})
