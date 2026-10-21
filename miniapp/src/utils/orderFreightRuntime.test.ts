import {it,expect,vi} from 'vitest'
import * as Vue from 'vue'
import {createRenderer,h,reactive,ref,nextTick,ssrContextKey} from 'vue'
import {readFileSync} from 'node:fs'
import {parse,compileScript,compileTemplate} from '@vue/compiler-sfc'
import Estimate from '../components/EmployeeFreightEstimate.vue'
import {emptyFreightState} from './orderFreight'
const api=vi.hoisted(()=>({preview:vi.fn(),resolve:vi.fn(),regions:vi.fn()}))
vi.mock('../api/customerPortal',()=>({previewEmployeeFreight:api.preview,resolveEmployeeShippingDestination:api.resolve,fetchEmployeeFreightRegions:api.regions}))
type Node={type:string;children:Node[];parent?:Node;props:Record<string,any>;text?:string}
const renderer=createRenderer<Node,Node>({createElement:type=>({type,children:[],props:{}}),createText:text=>({type:'#text',text,children:[],props:{}}),createComment:text=>({type:'#comment',text,children:[],props:{}}),setText:(n,t)=>{n.text=t},setElementText:(n,t)=>{n.text=t},patchProp:(n,k,_p,v)=>{n.props[k]=v},insert:(n,p,anchor)=>{n.parent=p;const i=anchor?p.children.indexOf(anchor):-1;if(i<0)p.children.push(n);else p.children.splice(i,0,n)},remove:n=>{if(n.parent)n.parent.children=n.parent.children.filter(x=>x!==n)},parentNode:n=>n.parent||null,nextSibling:()=>null})
// Vitest loads UniApp SFCs as SSR modules; use the actual template's client render with our host renderer.
const source=readFileSync(new URL('../components/EmployeeFreightEstimate.vue',import.meta.url),'utf8')
const {descriptor}=parse(source),script=compileScript(descriptor,{id:'freight-runtime'})
const compiled=compileTemplate({id:'freight-runtime',filename:'EmployeeFreightEstimate.vue',source:descriptor.template!.content,compilerOptions:{bindingMetadata:script.bindings,isCustomElement:tag=>['scroll-view','view','text','switch'].includes(tag)}})
const renderCode=compiled.code.replace(/import \{([^}]+)\} from "vue"/g,(_all,bindings:string)=>`const {${bindings.replace(/ as /g,':')}}=Vue`).replace('export function render','function render')
;(Estimate as any).render=new Function('Vue',renderCode+';return render')(Vue)
const settle=async()=>{await Promise.resolve();await Promise.resolve();await nextTick()}
it('real employee component resolves customer address, auto quotes and retains manual/free choice',async()=>{
 vi.useFakeTimers();api.regions.mockResolvedValue({regions:[]});api.resolve.mockResolvedValue({status:'resolved',destination:{province:'云南省',city:'普洱市',district:'思茅区'}});api.preview.mockImplementation(async(_t,b)=>({token:JSON.stringify(b),policy:{free_shipping:b.freight_override?.free_shipping===true},estimate_known:true,estimated_total_cents:1000,current_receivable_cents:b.freight_override?.manual_amount_cents||0}))
 const state=ref(emptyFreightState()),p=reactive({customerId:0,address:'',items:[] as any[]}),amount=ref(0),exposed=ref<any>();const root:Node={type:'root',children:[],props:{}}
 const app=renderer.createApp({setup:()=>()=>h(Estimate,{...p,token:'isolated',modelValue:state.value,'onUpdate:modelValue':v=>{state.value=v},'onAmount-change':v=>{amount.value=v},ref:exposed})});app.provide(ssrContextKey,{modules:new Set()});app.mount(root)
 try{
  p.customerId=1;p.address='云南省普洱市思茅区测试路';await settle();await vi.advanceTimersByTimeAsync(500);expect(state.value.destination.province).toBe('云南省');expect(api.preview).not.toHaveBeenCalled()
  p.items=[{product_id:1,qty:2}];await settle();await vi.advanceTimersByTimeAsync(500);expect(api.preview).toHaveBeenCalled()
  state.value={...state.value,manual:'20',free:false};await settle();await vi.advanceTimersByTimeAsync(500);expect(exposed.value.payload().shipping_amount).toBe(20)
  const find=(n:Node,type:string):Node|undefined=>n.type===type?n:n.children.map(x=>find(x,type)).find(Boolean)
  find(root,'switch')!.props.onChange({detail:{value:true}});await settle();expect(amount.value).toBe(0);expect(state.value.manual).toBe('20')
  find(root,'switch')!.props.onChange({detail:{value:false}});await settle();expect(amount.value).toBe(20)
 }finally{app.unmount();vi.useRealTimers()}
})
