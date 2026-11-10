import { beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, h, nextTick, ssrContextKey } from 'vue'
import { createPinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import Price from '../components/CustomerPricePreview.vue'
import { useSessionStore } from '../stores/session'
const api = vi.hoisted(() => ({ price: vi.fn() }))
vi.mock('../api/customerPortal', async original => ({ ...await original<typeof import('../api/customerPortal')>(), fetchOrderPriceTablePreview: api.price }))
type Node = { type:string; children:Node[]; parent?:Node; props:Record<string, any>; text?:string }
const renderer = createRenderer<Node,Node>({createElement:type=>({type,children:[],props:{}}),createText:text=>({type:'#text',text,children:[],props:{}}),createComment:text=>({type:'#comment',text,children:[],props:{}}),setText:(node,text)=>{node.text=text},setElementText:(node,text)=>{node.text=text;node.children=[]},patchProp:(node,key,_old,value)=>{node.props[key]=value},insert:(node,parent,anchor)=>{if(node.parent)node.parent.children=node.parent.children.filter(child=>child!==node);node.parent=parent;const i=anchor?parent.children.indexOf(anchor):-1;if(i<0)parent.children.push(node);else parent.children.splice(i,0,node)},remove:node=>{if(node?.parent)node.parent.children=node.parent.children.filter(child=>child!==node)},parentNode:node=>node?.parent||null,nextSibling:node=>node?.parent?.children[node.parent.children.indexOf(node)+1]||null})
const filename = new URL('../components/CustomerPricePreview.vue', import.meta.url), { descriptor } = parse(readFileSync(filename,'utf8')), script = compileScript(descriptor,{id:'preview-price'})
const template=compileTemplate({id:'preview-price',filename:filename.pathname,source:descriptor.template!.content,compilerOptions:{bindingMetadata:script.bindings,isCustomElement:tag=>['view','text','scroll-view'].includes(tag)}})
const renderCode=template.code.replace(/import \{([^}]+)\} from "vue"/g,(_all,b)=>`const {${b.replace(/ as /g,':')}}=Vue`).replace('export function render','function render')
;(Price as any).render=new Function('Vue',renderCode+';return render')(Vue)
const text=(node:Node):string=>(node.text||'')+node.children.map(text).join('')
const settle=async()=>{for(let i=0;i<15;i++)await Promise.resolve();await nextTick()}
const employee={mini_user_id:9,current_customer_id:0,bindings:[],capabilities:[],account_type:'employee',roles:['admin']}
const preview={mini_user_id:0,current_customer_id:74,current_customer_name:'客户甲',bindings:[],capabilities:[],account_type:'preview'}
beforeEach(()=>{vi.resetAllMocks();vi.stubEnv('VITE_KFERP_ENVIRONMENT','development');vi.stubEnv('VITE_KFERP_API_BASE','https://dev.qacoohee.com/app');vi.stubGlobal('uni',{getStorageSync:vi.fn(),setStorageSync:vi.fn(),removeStorageSync:vi.fn()})})
function mount(){const root:Node={type:'root',children:[],props:{}},pinia=createPinia(),session=useSessionStore(pinia);session.setToken('employee');session.applyContext(employee);session.beginCustomerPreview('preview-token',preview);const app=renderer.createApp({setup:()=>()=>h(Price,{usageCode:'product_order'})});app.use(pinia);app.provide(ssrContextKey,{modules:new Set()});app.mount(root);return{root,session,app}}
it('loads the selected customer price table using the scoped preview token',async()=>{api.price.mockResolvedValue({price_tables:[{table_name:'客户专属表',version_no:'V3.0.6'}],rows:[]});const view=mount();try{await settle();expect(api.price).toHaveBeenCalledWith('preview-token','product_order');expect(text(view.root)).toContain('客户专属表');expect(text(view.root)).toContain('V3.0.6')}finally{view.app.unmount()}})
it('discards a pending customer price response after leaving preview',async()=>{let complete!:(value:unknown)=>void;api.price.mockReturnValue(new Promise(resolve=>{complete=resolve}));const view=mount();try{view.session.endCustomerPreview();await nextTick();complete({price_tables:[{table_name:'不应展示'}],rows:[]});await settle();expect(text(view.root)).not.toContain('不应展示')}finally{view.app.unmount()}})
