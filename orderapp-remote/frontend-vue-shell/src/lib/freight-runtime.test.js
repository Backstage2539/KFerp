import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {parse,compileScript} from '@vue/compiler-sfc'
import {createSSRApp,h} from 'vue'
import {renderToString} from '@vue/server-renderer'
async function loadView(name,props){const file=new URL(`../components/${name}.vue`,import.meta.url);const {descriptor}=parse(fs.readFileSync(file,'utf8'),{filename:file.pathname});let code=compileScript(descriptor,{id:name}).content;code=code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g,(_,name)=>`const ${name}={render:()=>null}`);code=code.replace(/from (['"])([^'"]+)\1/g,(_,quote,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,file).href:import.meta.resolve(path))}`);const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`);let vm;await renderToString(createSSRApp({setup(){vm=View.setup(props,{expose(){},emit(){}});return()=>h('div')}}));return vm}
test('shipping quote discards late responses and submits only acknowledged current quote',async()=>{
 const original=globalThis.fetch;const pending=[];globalThis.fetch=async(url,opts)=>new Promise(resolve=>pending.push({url,body:JSON.parse(opts.body),resolve}));try{
  const vm=await loadView('OrderFreightEstimate',{customerId:77,items:[{product_id:10,qty:2}],savedDestination:{},customerPortal:false})
  vm.destination.province='云南省';const first=vm.preview();vm.destination.province='四川省';const second=vm.preview();assert.equal(pending.length,2)
  pending[1].resolve({ok:true,status:200,json:async()=>({token:'new',estimated_total_cents:2000,status:'pending_actual'})});await second
  pending[0].resolve({ok:true,status:200,json:async()=>({token:'old',estimated_total_cents:500,status:'pending_actual'})});await first
  assert.equal(vm.quote.value.token,'new');assert.equal(vm.busy.value,false);const p=vm.payload();assert.equal(p.shipping_amount,'0');assert.equal(p.freight_protocol,'deferred_freight_v1');assert.equal(p.freight_quote_token,'new');assert.equal(p.shipping_province,'四川省')
 }finally{globalThis.fetch=original}
})
test('region controls use string names as selection values and template copy is independent',async()=>{
 const source=fs.readFileSync(new URL('../components/OrderFreightEstimate.vue',import.meta.url),'utf8');assert.equal((source.match(/option-value="value"/g)||[]).length,3)
 const vm=await loadView('ShippingFeeTemplates',{companies:[]});const sourceTemplate={id:9,revision:4,name:'原表',carrier_id:1,logistics_product_id:2,groups:[{name:'一区',region_codes:['530000'],tiers:[{upper_weight_g:1000,fee_cents:510}],additional_weight_g:1000,additional_fee_cents:200}]};vm.edit(sourceTemplate);vm.copyTemplate();assert.equal(vm.form.value.id,0);vm.form.value.groups[0].region_codes.push('510000');assert.deepEqual(sourceTemplate.groups[0].region_codes,['530000']);assert.equal(vm.form.value.revision,0)
})

test('actual freight partial preview renders pending and confirmed amounts without hiding rows',async()=>{
 const {compileTemplate}=await import('@vue/compiler-sfc');const file=new URL('../components/ActualFreightImport.vue',import.meta.url);const {descriptor}=parse(fs.readFileSync(file,'utf8'),{filename:file.pathname});const script=compileScript(descriptor,{id:'actual-freight-render'});let code=script.content.replace('export default','const component =');const template=compileTemplate({id:'actual-freight-render',source:descriptor.template.content,filename:file.pathname,compilerOptions:{bindingMetadata:script.bindings}});assert.deepEqual(template.errors,[]);code+='\n'+template.code.replace('export function render','function render')+'\ncomponent.render=render;export default component;';code=code.replace(/from (['"])([^'"]+)\1/g,(_,quote,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,file).href:import.meta.resolve(path))}`);const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`);const setup=View.setup;View.setup=(props,ctx)=>{const vm=setup(props,ctx);vm.batch.value={id:1,filename:'test.xlsx',status:'partial',rows:[{row_no:2,tracking_no:'000SF',entry_id:10,actual_cents:1200,surcharge_cents:1000,target_cents:2200,delta_cents:2200},{row_no:3,tracking_no:'UNKNOWN',actual_cents:1200,delta_cents:0,error:'未匹配订单'}]};return vm};let html;try{html=await renderToString(createSSRApp(View,{companies:[]}))}catch(e){throw new Error(e.message)};assert.match(html,/部分确认，待处理/);assert.match(html,/已确认差额：¥22.00/);assert.match(html,/待确认差额：¥0.00/);assert.match(html,/未匹配订单/)
})
