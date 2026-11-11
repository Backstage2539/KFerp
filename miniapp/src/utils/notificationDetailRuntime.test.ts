import { beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, nextTick, ssrContextKey } from 'vue'
import { readFileSync } from 'node:fs'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import ts from 'typescript'
import Detail from '../pages/notification-detail/notification-detail.vue'
import { MiniRequestError } from '../api/client'

const state = vi.hoisted(() => ({ token: 'token-a', clearSession: vi.fn() }))
const hooks = vi.hoisted(() => ({ load: [] as ((q: Record<string,string>) => void)[], show: [] as (() => void)[], hide: [] as (() => void)[] }))
const request = vi.hoisted(() => vi.fn())
vi.mock('@dcloudio/uni-app', () => ({ onLoad: (f: any) => hooks.load.push(f), onShow: (f: any) => hooks.show.push(f), onHide: (f: any) => hooks.hide.push(f), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn() }))
vi.mock('../stores/session', () => ({ useSessionStore: () => state }))
vi.mock('../components/EnvironmentBadge.vue', () => ({default:{render:()=>null}}))
vi.mock('../components/PullUpBrandFooter.vue', () => ({default:{render:()=>null}}))
vi.mock('../utils/miniappShare', () => ({ defaultMiniappShare: vi.fn(), defaultMiniappTimelineShare: vi.fn(), refreshMiniappShareMenu: vi.fn() }))
vi.mock('../api/client', async (original) => ({ ...await original<typeof import('../api/client')>(), miniRequest: request }))

type Node = { type: string, children: Node[], parent?: Node, props: Record<string,unknown>, text?: string }
const renderer = createRenderer<Node,Node>({
  createElement: type => ({type,children:[],props:{}}), createText: text => ({type:'#text',text,children:[],props:{}}), createComment: text => ({type:'#comment',text,children:[],props:{}}),
  setText: (node,text) => {node.text=text}, setElementText: (node,text) => {node.text=text;node.children=[]}, patchProp: (node,key,_old,value) => {node.props[key]=value},
  insert: (node,parent,anchor) => {if(node.parent)node.parent.children=node.parent.children.filter(child=>child!==node);node.parent=parent;const i=anchor?parent.children.indexOf(anchor):-1;if(i<0)parent.children.push(node);else parent.children.splice(i,0,node)},
  remove: node => {if(node.parent)node.parent.children=node.parent.children.filter(child=>child!==node)}, parentNode: node => node.parent||null, nextSibling: node => node.parent?.children[node.parent.children.indexOf(node)+1]||null,
})
const {descriptor} = parse(readFileSync(new URL('../pages/notification-detail/notification-detail.vue',import.meta.url),'utf8'))
const script = compileScript(descriptor,{id:'detail'})
const compiled = compileTemplate({id:'detail',filename:'detail.vue',source:descriptor.template!.content,compilerOptions:{hoistStatic:false,bindingMetadata:script.bindings,isCustomElement:tag=>['view','text','button'].includes(tag)}})
const code = ts.transpileModule(compiled.code,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}}).outputText.replace(/import \{([^}]+)\} from ["']vue["'];?/g,(_all,bindings:string)=>`const {${bindings.replace(/ as /g,':')}}=Vue`).replace('export function render','function render')
;(Detail as any).render = new Function('Vue',code+';return render')(Vue)
function mount() {const root:Node={type:'root',children:[],props:{}};const app=renderer.createApp(Detail);app.provide(ssrContextKey, {modules:new Set()});app.mount(root);return {root,app}}
function text(node:Node):string{return (node.text||'')+node.children.map(text).join('')}
async function flush(){await Promise.resolve();await nextTick();await Promise.resolve();await nextTick()}
beforeEach(()=>{hooks.load.length=0;hooks.show.length=0;hooks.hide.length=0;state.token='token-a';state.clearSession.mockClear();request.mockReset();(globalThis as any).uni={reLaunch:vi.fn()}})

it('loads only the specified receipt with current token and hides it when leaving',async()=>{
 request.mockResolvedValue({receipt_no:'RC1',receipt_date:'2026-11-10',actual_cents:100,receivable_cents:200})
 const {root,app}=mount();try{hooks.load[0]({type:'receipt',id:'42'});hooks.show[0]();await flush();expect(request).toHaveBeenCalledWith('/api/mini/notification-details/receipts/42',{token:'token-a'});expect(text(root)).toContain('RC1');hooks.hide.forEach(f=>f());await flush();expect(text(root)).not.toContain('RC1')}finally{app.unmount()}
})
it('rejects extra customer parameters and redirects expired login without disclosing details',async()=>{
 const {root,app}=mount();try{hooks.load[0]({type:'bill',id:'42',customer_id:'9'});hooks.show[0]();await flush();expect(request).not.toHaveBeenCalled();expect(text(root)).toContain('通知链接无效')}finally{app.unmount()}
 const next=mount();try{request.mockRejectedValue(new MiniRequestError('expired',401));hooks.load.at(-1)!({type:'bill',id:'42'});hooks.show.at(-1)!();await flush();expect(state.clearSession).toHaveBeenCalled();expect((globalThis as any).uni.reLaunch).toHaveBeenCalledWith({url:expect.stringContaining(encodeURIComponent('/pages/notification-detail/notification-detail?type=bill&id=42'))})}finally{next.app.unmount()}
})
it('does not show stale receipt after hiding while a request is in flight',async()=>{
 let resolve!: (value:unknown)=>void;request.mockReturnValue(new Promise(r=>{resolve=r}))
 const {root,app}=mount();try{hooks.load[0]({type:'receipt',id:'42'});hooks.show[0]();hooks.hide.forEach(f=>f());resolve({receipt_no:'RC-SHOULD-NOT-SHOW'});await flush();expect(text(root)).not.toContain('RC-SHOULD-NOT-SHOW')}finally{app.unmount()}
})
it('shows an access error on 403 without redirecting or exposing prior customer details',async()=>{
 request.mockRejectedValue(new MiniRequestError('无权查看该单据',403))
 const {root,app}=mount();try{hooks.load[0]({type:'bill',id:'42'});hooks.show[0]();await flush();expect(text(root)).toContain('无权查看该单据');expect((globalThis as any).uni.reLaunch).not.toHaveBeenCalled();expect(state.clearSession).not.toHaveBeenCalled()}finally{app.unmount()}
})
