import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {parse,compileScript,compileTemplate} from '@vue/compiler-sfc'
import {createSSRApp,h} from 'vue'
import {renderToString} from '@vue/server-renderer'

test('selected cross-spec start shows each customer instruction and preserves amounts after conflict',async()=>{
 const file=new URL('../components/OperationStartGroupForm.vue',import.meta.url)
 const {descriptor}=parse(readFileSync(file,'utf8'))
 const script=compileScript(descriptor,{id:'group-start-test'})
 const template=compileTemplate({id:'group-start-test',source:descriptor.template.content,filename:file.pathname,compilerOptions:{bindingMetadata:script.bindings}})
 assert.deepEqual(template.errors,[])
 let code=script.content.replace("import {apiSend} from '../api/client'","const apiSend=(...args)=>globalThis.__workstationGroupApi(...args)")
 code=code.replace('export default','const component =')+'\n'+template.code.replace('export function render','function render')+'\ncomponent.render=render;export default component;'
 code=code.replace(/from (['"])([^'"]+)\1/g,(_,quote,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,file).href:import.meta.resolve(path))}`)
 const View=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
 const sourceTasks=[{job_card_id:153,work_order_no:'WO-153',spec_label:'2.5kg袋装',related_orders:[{order_id:1,order_item_id:1,order_no:'SO-1',customer_name:'甲店',operation_notes:[{operation_name:'包装',note:'贴甲店标签'}]}]},{job_card_id:154,work_order_no:'WO-154',spec_label:'227g袋装',related_orders:[{order_id:2,order_item_id:2,order_no:'SO-2',customer_name:'乙店',operation_notes:[{operation_name:'包装',note:'不贴标签'}]}]}]
 const props={taskIds:[153,154],sourceTasks};let state
 await renderToString(createSSRApp({setup(){state=View.setup(props,{expose(){},emit(){}});return()=>h('div')}}))
 state.preview.value={version:'group-v1',tasks:sourceTasks.map(row=>({job_card_id:row.job_card_id,product_name:'曲奇',planned_qty:10,input_qty:row.job_card_id===153?10:1,output_unit:'袋',input_unit:'袋',version:`v${row.job_card_id}`}))}
 state.previewChecked.value=true
 state.drafts.value=[{actual_input_qty:9,materials:[],difference_reason:'本次9袋'},{actual_input_qty:1,materials:[],difference_reason:''}]
 const html=await renderToString(createSSRApp({...View,setup:()=>state},props))
 for(const value of ['WO-153','WO-154','2.5kg袋装','227g袋装','甲店','乙店','贴甲店标签','不贴标签'])assert.ok(html.includes(value),value)
 let submitted
 globalThis.__workstationGroupApi=async(path,{body})=>{submitted=body;throw Object.assign(new Error('工序卡 154：库存已变化'),{status:409})}
 try {await state.submit()} finally {delete globalThis.__workstationGroupApi}
 assert.deepEqual(submitted.items.map(row=>[row.id,row.actual_input_qty]),[[153,9],[154,1]])
 assert.deepEqual(state.drafts.value.map(row=>row.actual_input_qty),[9,1])
 assert.match(state.error.value,/154.*库存已变化/)
})
