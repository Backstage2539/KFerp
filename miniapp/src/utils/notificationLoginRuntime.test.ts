import { beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, nextTick, ssrContextKey } from 'vue'
import { readFileSync } from 'node:fs'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import ts from 'typescript'
import Login from '../pages/login/login.vue'
import Select from '../pages/customer-select/customer-select.vue'

const restore = vi.hoisted(() => vi.fn(async () => {}))
vi.mock('../utils/registrationSession', () => ({ restoreRegistrationSession: restore }))
const hooks = vi.hoisted(() => ({ load: [] as ((q: Record<string,string>) => void)[], show: [] as (() => void)[] }))
const state = vi.hoisted(() => ({ token: '', bindings: [] as any[], autoRestoreBlocked: true, setToken: vi.fn(), applyContext: vi.fn() }))
const api = vi.hoisted(() => ({ loginWithPhoneIdentify: vi.fn(), loginWithPassword: vi.fn(), fetchMiniLoginOptions: vi.fn(), fetchMe: vi.fn(), switchCurrentCustomer: vi.fn() }))
vi.mock('@dcloudio/uni-app', () => ({ onLoad: (f: any) => hooks.load.push(f), onShow: (f: any) => hooks.show.push(f), onHide: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn() }))
vi.mock('../stores/session', () => ({ useSessionStore: () => state }))
vi.mock('../api/customerPortal', () => api)
vi.mock('../components/EnvironmentBadge.vue', () => ({ default: { render: () => null } }))
vi.mock('../components/PullUpBrandFooter.vue', () => ({ default: { render: () => null } }))
vi.mock('../utils/miniappShare', () => ({ defaultMiniappShare: vi.fn(), defaultMiniappTimelineShare: vi.fn(), refreshMiniappShareMenu: vi.fn() }))

type Node = { type: string, children: Node[], parent?: Node, props: Record<string,any>, text?: string }
const renderer = createRenderer<Node,Node>({
  createElement: type => ({type,children:[],props:{}}), createText: text => ({type:'#text',text,children:[],props:{}}), createComment: text => ({type:'#comment',text,children:[],props:{}}),
  setText: (node,text) => {node.text=text}, setElementText: (node,text) => {node.text=text;node.children=[]}, patchProp: (node,key,_old,value) => {node.props[key]=value},
  insert: (node,parent,anchor) => {if(node.parent)node.parent.children=node.parent.children.filter(child=>child!==node);node.parent=parent;const i=anchor?parent.children.indexOf(anchor):-1;if(i<0)parent.children.push(node);else parent.children.splice(i,0,node)},
  remove: node => {if(node.parent)node.parent.children=node.parent.children.filter(child=>child!==node)}, parentNode: node => node.parent||null, nextSibling: node => node.parent?.children[node.parent.children.indexOf(node)+1]||null,
})
function mount(component: any, filename: string) {
  const {descriptor} = parse(readFileSync(new URL(`../pages/${filename}/${filename}.vue`,import.meta.url),'utf8'))
  const script = compileScript(descriptor,{id:filename})
  const template = compileTemplate({id:filename,filename:filename+'.vue',source:descriptor.template!.content,compilerOptions:{hoistStatic:false,bindingMetadata:script.bindings,isCustomElement:tag=>['view','text','button'].includes(tag)}})
  const code = ts.transpileModule(template.code,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}}).outputText.replace(/import \{([^}]+)\} from ["']vue["'];?/g,(_all,bindings:string)=>`const {${bindings.replace(/ as /g,':')}}=Vue`).replace('export function render','function render')
  component.render = new Function('Vue',code+';return render')(Vue)
  const root:Node={type:'root',children:[],props:{}};const app=renderer.createApp(component);app.provide(ssrContextKey, {modules:new Set()});app.mount(root);return {root,app}
}
function find(node: Node, predicate: (n:Node)=>boolean): Node | undefined { if(predicate(node)) return node; for(const child of node.children){const result=find(child,predicate);if(result)return result} }
async function flush(){await Promise.resolve();await nextTick();await Promise.resolve();await nextTick()}
beforeEach(()=>{hooks.load.length=0;hooks.show.length=0;state.token='';state.bindings=[];state.setToken.mockReset();state.applyContext.mockReset();state.autoRestoreBlocked=true;restore.mockReset();restore.mockResolvedValue();Object.values(api).forEach(f=>f.mockReset());api.fetchMiniLoginOptions.mockResolvedValue({show_password_login:false});(globalThis as any).uni={reLaunch:vi.fn(),redirectTo:vi.fn(),login:({success}:any)=>success({code:'wechat-code'})}})

it('returns a phone-authenticated customer to the receipt detail',async()=>{
  const target='/pages/notification-detail/notification-detail?type=receipt&id=42'
  api.loginWithPhoneIdentify.mockResolvedValue({token:'fresh',current_customer_id:74,bindings:[{customer_id:74,status:'approved'}]})
  const {root,app}=mount(Login,'login');try{
    hooks.load[0]({return_to:encodeURIComponent(target)})
    hooks.show[0]()
    const button=find(root,n=>n.props['open-type']==='getPhoneNumber')!
    await button.props.onGetphonenumber({detail:{code:'phone-code'}});await flush()
    expect(api.loginWithPhoneIdentify).toHaveBeenCalledWith('wechat-code','phone-code')
    expect(state.setToken).toHaveBeenCalledWith('fresh')
    expect((globalThis as any).uni.reLaunch).toHaveBeenCalledWith({url:target})
  }finally{app.unmount()}
})
it('returns a phone-authenticated employee to the order detail',async()=>{
  const target='/pages/employee-order-detail/employee-order-detail?id=12'
  api.loginWithPhoneIdentify.mockResolvedValue({token:'employee-token',account_type:'employee',bindings:[]})
  const {root,app}=mount(Login,'login');try{
    hooks.load[0]({return_to:encodeURIComponent(target)})
    hooks.show[0]()
    await find(root,n=>n.props['open-type']==='getPhoneNumber')!.props.onGetphonenumber({detail:{code:'phone-code'}});await flush()
    expect((globalThis as any).uni.reLaunch).toHaveBeenCalledWith({url:target})
  }finally{app.unmount()}
})
it('preserves the bill target through customer selection and waits for the authorized switch',async()=>{
  const target='/pages/notification-detail/notification-detail?type=bill&id=9'
  state.token='fresh';state.bindings=[{customer_id:74,customer_name:'客户',status:'approved'}]
  let finish!: (value:unknown)=>void
  api.switchCurrentCustomer.mockReturnValue(new Promise(resolve=>{finish=resolve}))
  const {root,app}=mount(Select,'customer-select');try{
    hooks.load[0]({return_to:encodeURIComponent(target)})
    const button=find(root,n=>n.type==='button'&&typeof n.props.onTap==='function'&&n.text==='客户')!
    const pending=button.props.onTap();await flush()
    expect(api.switchCurrentCustomer).toHaveBeenCalledWith('fresh',74)
    expect((globalThis as any).uni.reLaunch).not.toHaveBeenCalled()
    finish({current_customer_id:74,bindings:state.bindings});await pending;await flush()
    expect((globalThis as any).uni.reLaunch).toHaveBeenCalledWith({url:target})
  }finally{app.unmount()}
})
it('does not navigate to the bill when selecting a customer is rejected',async()=>{
  state.token='fresh';state.bindings=[{customer_id:74,customer_name:'客户',status:'approved'}]
  api.switchCurrentCustomer.mockRejectedValue(new Error('无权选择该客户'))
  const {root,app}=mount(Select,'customer-select');try{
    hooks.load[0]({return_to:encodeURIComponent('/pages/notification-detail/notification-detail?type=bill&id=9')})
    await find(root,n=>n.type==='button'&&typeof n.props.onTap==='function'&&n.text==='客户')!.props.onTap();await flush()
    expect((globalThis as any).uni.reLaunch).not.toHaveBeenCalled()
    expect(state.applyContext).not.toHaveBeenCalled()
  }finally{app.unmount()}
})

it('does not let a pending session restore redirect while password login is in progress', async () => {
  state.autoRestoreBlocked = false
  state.token = 'old-token'
  let finishRestore!: () => void
  restore.mockReturnValue(new Promise<void>(resolve => { finishRestore = resolve }))
  api.fetchMiniLoginOptions.mockResolvedValue({show_password_login:true})
  let finishPassword!: (value: any) => void
  api.loginWithPassword.mockReturnValue(new Promise(resolve => { finishPassword = resolve }))
  const {app}=mount(Login,'login')
  try {
    hooks.load[0]({})
    hooks.show[0]()
    await flush()
    const vm = (app as any)._instance.setupState
    vm.loginName = 'customer'
    vm.password = 'secret'
    const pending = vm.handlePasswordLogin()
    finishRestore(); await flush()
    expect((globalThis as any).uni.reLaunch).not.toHaveBeenCalled()
    finishPassword({token:'customer-token',current_customer_id:74,bindings:[{customer_id:74,status:'approved'}]})
    await pending; await flush()
    expect(state.setToken).toHaveBeenCalledWith('customer-token')
    expect((globalThis as any).uni.reLaunch).toHaveBeenCalledWith({url:'/pages/home/home'})
  } finally {app.unmount()}
})

it('keeps phone login usable when login options cannot be loaded', async () => {
  api.fetchMiniLoginOptions.mockRejectedValue(new Error('network unavailable'))
  api.loginWithPhoneIdentify.mockResolvedValue({ token:'customer-token', current_customer_id:74, bindings:[{customer_id:74,status:'approved'}] })
  const {root, app} = mount(Login,'login')
  try {
    hooks.load[0]({})
    hooks.show[0]()
    await flush()
    await find(root,n=>n.props['open-type']==='getPhoneNumber')!.props.onGetphonenumber({detail:{code:'phone-code'}})
    await flush()
    expect(state.setToken).toHaveBeenCalledWith('customer-token')
    expect((globalThis as any).uni.reLaunch).toHaveBeenCalledWith({url:'/pages/home/home'})
  } finally {app.unmount()}
})
