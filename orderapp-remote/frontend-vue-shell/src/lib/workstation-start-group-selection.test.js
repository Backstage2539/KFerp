import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {JSDOM} from 'jsdom'
import {parse,compileScript} from '@vue/compiler-sfc'

const dom=new JSDOM('<div id="app"></div>',{url:'http://localhost'})
for(const key of ['window','document','Node','Element','HTMLElement','SVGElement','Event','MouseEvent'])globalThis[key]=dom.window[key]
const {createApp,h,reactive,nextTick}=await import('vue')
const file=new URL('../components/OperationStartGroupForm.vue',import.meta.url)
const {descriptor}=parse(readFileSync(file,'utf8'))
let code=compileScript(descriptor,{id:'group-selection-test',inlineTemplate:true}).content
code=code.replace("import {apiSend} from '../api/client'","const apiSend=(...args)=>globalThis.__groupSelectionApi(...args)")
code=code.replace(/from (['"])([^'"]+)\1/g,(_,quote,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,file).href:import.meta.resolve(path))}`)
const View=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
const tick=async()=>{for(let i=0;i<3;i++){await nextTick();await new Promise(r=>setImmediate(r))}}
const preview=ids=>({version:`group-${ids.join('-')}`,tasks:ids.map(id=>({job_card_id:id,work_order_id:id,product_name:`商品${id}`,input_qty:1,planned_qty:1,input_unit:'袋',output_unit:'袋',version:`v${id}`,materials:[]}))})
const findButton=text=>[...document.querySelectorAll('button')].find(x=>x.textContent.includes(text))
const fill=(selector,value)=>{const el=document.querySelector(selector);assert.ok(el);el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}))}

test('changing selected orders reloads preview and ignores an older response',async()=>{
  const pending=[],submitted=[]
  globalThis.__groupSelectionApi=async(path,{body})=>{
    if(path.endsWith('/preview'))return new Promise(resolve=>pending.push({body,resolve}))
    submitted.push(body);return {items:[]}
  }
  const props=reactive({taskIds:[101,102],sourceTasks:[]})
  const app=createApp({render:()=>h(View,props)})
  try {
    app.mount('#app');await tick()
    assert.equal(pending.length,1)
    props.taskIds=[153,154];await tick()
    assert.equal(pending.length,2,'changed selection must fetch a fresh preview')
    pending[1].resolve(preview([153,154]));await tick()
    pending[0].resolve(preview([101,102]));await tick()
    assert.match(document.body.textContent,/商品153/)
    assert.doesNotMatch(document.body.textContent,/商品101/)
    const button=[...document.querySelectorAll('button')].find(x=>x.textContent.includes('确认整组开工'))
    assert.equal(button.disabled,false)
    button.click();await tick()
    assert.deepEqual(submitted[0].items.map(x=>x.id),[153,154])
    assert.equal(submitted[0].expected_version,'group-153-154')
  } finally {app.unmount();document.body.innerHTML='<div id="app"></div>';delete globalThis.__groupSelectionApi}
})

test('switching a loaded group clears old form before the next preview can submit',async()=>{
  let resolveNew;const writes=[],locks=[]
  globalThis.__groupSelectionApi=async(path,{body})=>{
    if(path.endsWith('/preview'))return body.job_card_ids[0]===101?preview([101,102]):new Promise(r=>resolveNew=r)
    writes.push(body);throw Object.assign(new Error('库存变化'),{status:409})
  }
  const props=reactive({taskIds:[101,102],sourceTasks:[],onLockChange:value=>locks.push(value)})
  const app=createApp({render:()=>h(View,props)})
  try {
    app.mount('#app');await tick()
    let button=[...document.querySelectorAll('button')].find(x=>x.textContent.includes('确认整组开工'))
    button.click();await tick()
    const oldRequest=writes[0].request_id
    props.taskIds=[153,154];await tick()
    assert.doesNotMatch(document.body.textContent,/商品101/)
    button=[...document.querySelectorAll('button')].find(x=>x.textContent.includes('确认整组开工'))
    assert.equal(button.disabled,true)
    button.click();await tick();assert.equal(writes.length,1)
    resolveNew(preview([153,154]));await tick()
    button.click();await tick()
    assert.deepEqual(writes[1].items.map(x=>x.id),[153,154])
    assert.notEqual(writes[1].request_id,oldRequest)
    assert.ok(locks.includes(true),'submitting must lock parent selection')
    assert.equal(locks.at(-1),false)
  } finally {app.unmount();document.body.innerHTML='<div id="app"></div>';delete globalThis.__groupSelectionApi}
})

test('refresh after a version conflict preserves each entered quantity and note with a new request version',async()=>{
  const writes=[];let version=1
  const next=()=>{const value=preview([101,102]);value.version=`group-${version}`;for(const p of value.tasks){p.version=`card-${p.job_card_id}-${version}`;p.materials=[{key:'bag',name:'袋子',unit:'pic',qty:1,available_qty:10,warehouse:'wip',owner_customer_id:0,material_id:1}]}return value}
  globalThis.__groupSelectionApi=async(path,{body})=>{if(path.endsWith('/preview'))return next();writes.push(body);throw Object.assign(new Error('任务或库存已变化，请刷新'),{status:409})}
  const app=createApp(View,{taskIds:[101,102],sourceTasks:[]})
  try {
    app.mount('#app');await tick()
    fill('fieldset:first-of-type label input',2)
    fill('fieldset:first-of-type label:nth-last-of-type(2) input',3)
    fill('fieldset:first-of-type label:last-of-type input','保留人工投入')
    findButton('确认整组开工').click();await tick()
    const refresh=findButton('刷新核对');assert.ok(refresh)
    assert.equal(findButton('确认整组开工').disabled,true)
    version=2;refresh.click();await tick()
    assert.equal(document.querySelector('fieldset:first-of-type label input').value,'2')
    assert.equal(document.querySelector('fieldset:first-of-type label:nth-last-of-type(2) input').value,'3')
    assert.equal(document.querySelector('fieldset:first-of-type label:last-of-type input').value,'保留人工投入')
    findButton('确认整组开工').click();await tick()
    assert.equal(writes[1].expected_version,'group-2')
    assert.equal(writes[1].items[0].expected_version,'card-101-2')
    assert.equal(writes[1].items[0].actual_input_qty,2)
    assert.equal(writes[1].items[0].materials[0].qty,3)
    assert.notEqual(writes[1].request_id,writes[0].request_id)
  }finally{app.unmount();document.body.innerHTML='<div id="app"></div>';delete globalThis.__groupSelectionApi}
})

test('refresh errors and unavailable material identities retain visible input and prohibit stale submit',async()=>{
  let phase='initial',writes=0
  globalThis.__groupSelectionApi=async(path)=>{
    if(!path.endsWith('/preview')){writes++;return {}}
    if(phase==='failure')throw Object.assign(new Error('核对网络失败'),{status:503})
    const value=preview([101,102])
    for(const p of value.tasks)p.materials=[{key:phase==='changed'?'new-bag':'bag',name:'袋子',unit:'pic',qty:1,available_qty:10,warehouse:'wip',material_id:phase==='changed'?2:1}]
    return value
  }
  const app=createApp(View,{taskIds:[101,102],sourceTasks:[]})
  try{
    app.mount('#app');await tick();fill('fieldset:first-of-type label:last-of-type input','手写内容')
    phase='failure';const refresh=findButton('刷新核对');assert.ok(refresh);refresh.click();await tick()
    assert.equal(document.querySelector('fieldset:first-of-type label:last-of-type input').value,'手写内容')
    assert.match(document.body.textContent,/核对网络失败/)
    assert.equal(findButton('确认整组开工').disabled,true)
    phase='changed';refresh.click();await tick()
    assert.equal(document.querySelector('fieldset:first-of-type label:last-of-type input').value,'手写内容')
    assert.match(document.body.textContent,/工序卡 101.*用料清单/)
    assert.equal(findButton('确认整组开工').disabled,true)
    assert.equal(writes,0)
  }finally{app.unmount();document.body.innerHTML='<div id="app"></div>';delete globalThis.__groupSelectionApi}
})
