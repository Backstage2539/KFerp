import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {parse,compileScript} from '@vue/compiler-sfc'
import {createSSRApp,h} from 'vue'
import {renderToString} from '@vue/server-renderer'
const file=new URL('../components/CustomerRecipientPicker.vue',import.meta.url)
let code=compileScript(parse(fs.readFileSync(file,'utf8'),{filename:file.pathname}).descriptor,{id:'direct-ship-region'}).content
code=code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g,(_,name)=>`const ${name}={render:()=>null}`)
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,file).href:import.meta.resolve(path))}`)
const {default:Picker}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const regions=[{code:'530000',name:'云南省',alias:'云南',level:1,parent_code:''},{code:'532900',name:'大理白族自治州',alias:'大理白族',level:2,parent_code:'530000'},{code:'532901',name:'大理市',alias:'大理',path:'云南省 / 大理白族自治州 / 大理市',level:3,parent_code:'532900'}]
async function harness(portal=false,options={}){const events=[],calls=[];let state
 const previous=globalThis.fetch
 globalThis.fetch=async(url,init={})=>{calls.push({url:String(url),method:init.method||'GET',body:init.body,headers:init.headers});if(String(url).includes('regions'))return options.regionFail?new Response(JSON.stringify({error:'地区不可用'}),{status:503}):new Response(JSON.stringify({regions}));if(init.method==='PUT')return options.saveFail?new Response(JSON.stringify({error:'地址版本已变化'}),{status:409}):new Response(JSON.stringify({...JSON.parse(init.body),revision:5}));return new Response(JSON.stringify({rows:[]}))}
 await renderToString(createSSRApp({setup(){state=Picker.setup({customerId:307,requireSaved:true,requireValidRegion:true,customerPortal:portal,apiBase:portal?'/api/customer-processing/portal/recipient-addresses':''},{expose(){},emit:(...a)=>events.push(a)});return()=>h('div')}}))
 return {state,events,calls,restore(){globalThis.fetch=previous}}
}
test('旧地址纠正行政区保留大理古城，只有明确保存才写地址簿',async()=>{const x=await harness();try{
 await x.state.loadRegions();const row={id:9,revision:4,recipient_name:'收件人',phone:'13800000000',company:'门店',province:'云南省',city:'大理市',district:'大理古城',detail_address:'人民路1号',is_default:true}
 x.state.edit(row);assert.equal(x.events.length,0);assert.equal(x.state.editing.value,true)
 x.state.chooseRegion('532901');assert.equal(x.state.form.city,'大理白族自治州');assert.equal(x.state.form.district,'大理市');assert.match(x.state.form.detail_address,/大理古城.*人民路1号/);assert.equal(x.calls.filter(c=>c.method==='PUT').length,0)
 await x.state.save(true);const saved=x.calls.find(c=>c.method==='PUT');assert.ok(saved);const body=JSON.parse(saved.body);assert.equal(body.expected_revision,4);assert.equal(body.company,'门店');assert.equal(body.is_default,true);assert.equal(body.detail_address,x.state.form.detail_address);assert.equal(x.events.at(-1)[0],'select')
 }finally{x.restore()}})
test('重复旧地名不重复加入详细地址，地区不完整不可提交',async()=>{const x=await harness();try{await x.state.loadRegions();x.state.edit({id:9,revision:4,recipient_name:'收件人',phone:'13800000000',province:'云南省',city:'大理市',district:'大理古城',detail_address:'大理古城人民路1号'});x.state.chooseRegion('532901');assert.equal(x.state.form.detail_address,'大理古城人民路1号');x.state.chooseRegion('');await x.state.save(true);assert.equal(x.calls.filter(c=>c.method==='PUT').length,0);assert.match(x.state.error.value,/省市区/)}finally{x.restore()}})
test('客户门户使用代发专用地区接口且不携带员工认证',async()=>{const x=await harness(true);try{await x.state.loadRegions();assert.equal(x.calls[0].url,'/api/customer-processing/portal/direct-ship/shipping-regions');assert.equal(x.calls[0].headers.Authorization,undefined)}finally{x.restore()}})
test('员工使用当前客户代发专用地区接口',async()=>{const x=await harness();try{await x.state.loadRegions();assert.equal(x.calls[0].url,'/api/customer-fulfillment/307/direct-ship/shipping-regions')}finally{x.restore()}})
test('地区列表不可用或保存版本冲突时保留地址编辑内容',async()=>{const failed=await harness(false,{regionFail:true});try{await failed.state.loadRegions();assert.match(failed.state.error.value,/地区不可用/);assert.equal(failed.state.regions.value.length,0)}finally{failed.restore()};const x=await harness(false,{saveFail:true});try{await x.state.loadRegions();x.state.edit({id:9,revision:4,recipient_name:'收件人',phone:'13800000000',province:'云南省',city:'大理市',district:'大理古城',detail_address:'人民路1号'});x.state.chooseRegion('532901');await x.state.save(true);assert.match(x.state.error.value,/版本/);assert.match(x.state.form.detail_address,/大理古城.*人民路1号/);assert.equal(x.state.editing.value,true);assert.equal(x.events.length,0)}finally{x.restore()}})
test('普通录单仍可使用原有自由文本地区，不强制地区列表',async()=>{const events=[];let state;await renderToString(createSSRApp({setup(){state=Picker.setup({customerId:307,requireSaved:true},{expose(){},emit:(...a)=>events.push(a)});return()=>h('div')}}));state.use({id:9,recipient_name:'收件人',phone:'13800000000',province:'云南省',city:'大理市',district:'大理古城',detail_address:'人民路1号'});assert.equal(events[0][0],'select')})
test('代发继续允许选择旧地址及仅省份地址，也允许不改地区时保存',async()=>{const x=await harness();try{await x.state.loadRegions();for(const row of [{id:9,province:'云南省',city:'大理市',district:'大理古城'},{id:10,province:'云南省',city:'',district:''}]){Object.assign(row,{revision:4,recipient_name:'收件人',phone:'13800000000',detail_address:'人民路1号'});x.state.use(row);assert.equal(x.events.at(-1)[0],'select');x.state.edit(row);await x.state.save(true);assert.equal(x.calls.at(-1).method,'PUT');assert.equal(x.events.at(-1)[0],'select')}}finally{x.restore()}})
