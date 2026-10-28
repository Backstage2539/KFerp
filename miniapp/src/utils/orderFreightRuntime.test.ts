import {it,expect,vi} from 'vitest'
import * as Vue from 'vue'
import {createRenderer,h,reactive,ref,nextTick,ssrContextKey} from 'vue'
import {readFileSync} from 'node:fs'
import {parse,compileScript,compileTemplate} from '@vue/compiler-sfc'
import Estimate from '../components/EmployeeFreightEstimate.vue'
import {emptyFreightState,missingWeightItemNames,type FreightState} from './orderFreight'
const api=vi.hoisted(()=>({preview:vi.fn(),resolve:vi.fn(),regions:vi.fn(),templates:vi.fn()}))
vi.mock('../api/customerPortal',()=>({previewEmployeeFreight:api.preview,resolveEmployeeShippingDestination:api.resolve,fetchEmployeeFreightRegions:api.regions,fetchEmployeeFreightTemplates:api.templates}))
type Node={type:string;children:Node[];parent?:Node;props:Record<string,any>;text?:string}
const rootText=(node:Node):string=>[node.text||'',...node.children.map(rootText)].join(' ')
const find=(node:Node,type:string):Node|undefined=>node.type===type?node:node.children.map(x=>find(x,type)).find(Boolean)
const renderer=createRenderer<Node,Node>({createElement:type=>({type,children:[],props:{}}),createText:text=>({type:'#text',text,children:[],props:{}}),createComment:text=>({type:'#comment',text,children:[],props:{}}),setText:(n,t)=>{n.text=t},setElementText:(n,t)=>{n.text=t},patchProp:(n,k,_p,v)=>{n.props[k]=v},insert:(n,p,anchor)=>{n.parent=p;const i=anchor?p.children.indexOf(anchor):-1;if(i<0)p.children.push(n);else p.children.splice(i,0,n)},remove:n=>{if(n.parent)n.parent.children=n.parent.children.filter(x=>x!==n)},parentNode:n=>n.parent||null,nextSibling:()=>null})
// Vitest loads UniApp SFCs as SSR modules; use the actual template's client render with our host renderer.
const source=readFileSync(new URL('../components/EmployeeFreightEstimate.vue',import.meta.url),'utf8')
const {descriptor}=parse(source),script=compileScript(descriptor,{id:'freight-runtime'})
const compiled=compileTemplate({id:'freight-runtime',filename:'EmployeeFreightEstimate.vue',source:descriptor.template!.content,compilerOptions:{bindingMetadata:script.bindings,isCustomElement:tag=>['scroll-view','view','text','switch'].includes(tag)}})
const renderCode=compiled.code.replace(/import \{([^}]+)\} from "vue"/g,(_all,bindings:string)=>`const {${bindings.replace(/ as /g,':')}}=Vue`).replace('export function render','function render')
;(Estimate as any).render=new Function('Vue',renderCode+';return render')(Vue)
const settle=async()=>{await Promise.resolve();await Promise.resolve();await nextTick()}
it('successful quote does not trigger repeated previews when the amount is returned to the parent',async()=>{
 vi.useFakeTimers();api.regions.mockReset().mockResolvedValue({regions:[]});api.templates.mockReset().mockResolvedValue({templates:[]});api.preview.mockReset().mockResolvedValue({token:'stable',policy:{},estimate_known:true,current_receivable_cents:800})
 const state=ref(emptyFreightState()),root:Node={type:'root',children:[],props:{}}
 const app=renderer.createApp({setup:()=>()=>h(Estimate,{token:'isolated',customerId:1,items:[{product_id:1,qty:1}],modelValue:state.value,'onUpdate:modelValue':v=>{state.value=v}})});app.provide(ssrContextKey,{modules:new Set()});app.mount(root)
 try{await settle();await vi.advanceTimersByTimeAsync(2000);await settle();expect(api.preview).toHaveBeenCalledTimes(1);expect(state.value.autoAmountCents).toBe(800)}finally{app.unmount();vi.useRealTimers()}
})
it('real employee component resolves customer address, auto quotes and retains manual/free choice',async()=>{
 vi.useFakeTimers();api.regions.mockResolvedValue({regions:[]});api.templates.mockResolvedValue({templates:[]});api.resolve.mockResolvedValue({status:'resolved',destination:{province:'云南省',city:'普洱市',district:'思茅区'}});api.preview.mockReset().mockImplementation(async(_t,b)=>({token:JSON.stringify(b),policy:{free_shipping:b.freight_override?.free_shipping===true},estimate_known:true,estimated_total_cents:1000,current_receivable_cents:b.freight_override?.manual_amount_cents||0}))
 const state=ref(emptyFreightState()),p=reactive({customerId:0,address:'',items:[] as any[]}),amount=ref(0),exposed=ref<any>();const root:Node={type:'root',children:[],props:{}}
 const app=renderer.createApp({setup:()=>()=>h(Estimate,{...p,token:'isolated',modelValue:state.value,'onUpdate:modelValue':v=>{state.value=v},'onAmount-change':v=>{amount.value=v},ref:exposed})});app.provide(ssrContextKey,{modules:new Set()});app.mount(root)
 try{
  p.customerId=1;p.address='云南省普洱市思茅区测试路';await settle();await vi.advanceTimersByTimeAsync(500);expect(state.value.destination.province).toBe('云南省');expect(api.preview).not.toHaveBeenCalled()
  p.items=[{product_id:1,qty:2}];await settle();await vi.advanceTimersByTimeAsync(500);expect(api.preview).toHaveBeenCalled()
  find(root,'input')!.props.onInput({detail:{value:'20'}});await settle();await vi.advanceTimersByTimeAsync(500);await settle();expect(exposed.value.payload().shipping_amount).toBe(20)
  find(root,'switch')!.props.onChange({detail:{value:true}});await settle();expect(amount.value).toBe(0);expect(state.value.manual).toBe('20')
  find(root,'switch')!.props.onChange({detail:{value:false}});await settle();expect(amount.value).toBe(20)
 }finally{app.unmount();vi.useRealTimers()}
})
it('editing an existing order preserves the saved freight until a new choice is made',async()=>{
 vi.useFakeTimers();api.regions.mockResolvedValue({regions:[]});api.templates.mockResolvedValue({templates:[]});api.preview.mockReset()
 const state=ref<FreightState>({...emptyFreightState(),preserveExisting:true,preservedAmount:10,manual:'',free:false}),p=reactive({customerId:1,address:'',orderId:42,items:[{product_id:1,qty:1}]}),amount=ref(10),exposed=ref<any>(),root:Node={type:'root',children:[],props:{}}
 const app=renderer.createApp({setup:()=>()=>h(Estimate,{...p,token:'isolated',modelValue:state.value,'onUpdate:modelValue':v=>{state.value=v},'onAmount-change':v=>{amount.value=v},ref:exposed})});app.provide(ssrContextKey,{modules:new Set()});app.mount(root)
 try{await settle();await vi.advanceTimersByTimeAsync(500);await settle();expect(api.preview).not.toHaveBeenCalled();expect(exposed.value.payload().shipping_amount).toBe(10);expect(exposed.value.payload().freight_protocol).toBe('');find(root,'input')!.props.onInput({detail:{value:'12'}});await settle();expect(state.value.preserveExisting).toBe(false)}finally{app.unmount();vi.useRealTimers()}
})
it('names products missing logistics weight when gross parcel weight is needed',async()=>{
 vi.useFakeTimers();api.regions.mockResolvedValue({regions:[]});api.templates.mockResolvedValue({templates:[]});api.preview.mockReset().mockResolvedValue({token:'unknown',estimate_known:false,current_receivable_cents:0,cost_pending_reason:'缺少商品物流重量',item_weights:[{product_id:1,bom_spec_id:2,qty:1,logistics_unit_weight_g:null}]})
 const state=ref(emptyFreightState()),p=reactive({customerId:1,address:'',items:[{product_id:1,bom_spec_id:2,qty:1,name:'测试生豆'}]}),root:Node={type:'root',children:[],props:{}},exposed=ref<any>()
 const app=renderer.createApp({setup:()=>()=>h(Estimate,{...p,token:'isolated',modelValue:state.value,'onUpdate:modelValue':v=>{state.value=v},ref:exposed})});app.provide(ssrContextKey,{modules:new Set()});app.mount(root)
 try{await settle();const quote=await exposed.value.preview();await settle();expect(quote?.estimate_known).toBe(false);expect(api.preview).toHaveBeenCalledTimes(1);expect(missingWeightItemNames(p.items,quote)).toEqual(['测试生豆']);expect(rootText(root)).toContain('本单包裹总重量（克，含包装）');expect(source).toContain('{{missingWeightItems.join(\'、\')}}')}finally{app.unmount();vi.useRealTimers()}
})
