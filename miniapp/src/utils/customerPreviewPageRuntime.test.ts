import { beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, h, nextTick, ssrContextKey } from 'vue'
import { createPinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import ts from 'typescript'
import PreviewPage from '../pages/admin-customer-preview/admin-customer-preview.vue'
import EnvironmentBadge from '../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../components/PullUpBrandFooter.vue'
import { useSessionStore } from '../stores/session'

const lifecycle = vi.hoisted(() => ({ show: [] as Array<() => unknown> }))
const api = vi.hoisted(() => ({ list: vi.fn(), start: vi.fn(), exit: vi.fn(), me: vi.fn() }))
vi.mock('@dcloudio/uni-app', () => ({ onShow: (fn: () => unknown) => lifecycle.show.push(fn), onHide: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn() }))
vi.mock('./miniappShare', () => ({ defaultMiniappShare: vi.fn(), defaultMiniappTimelineShare: vi.fn(), refreshMiniappShareMenu: vi.fn(async () => {}) }))
vi.mock('../api/customerPreview', () => ({ fetchPreviewCustomers: api.list, beginPreview: api.start, exitPreview: api.exit }))
vi.mock('../api/customerPortal', () => ({ fetchMe: api.me }))

type Node = { type: string; children: Node[]; parent?: Node; props: Record<string, any>; text?: string; [key: string]: any }
const renderer = createRenderer<Node, Node>({
  createElement: type => ({ type, children: [], props: {}, tagName: type.toUpperCase(), addEventListener() {}, removeEventListener() {}, getRootNode() { return { activeElement: null } } }),
  createText: text => ({ type: '#text', text, children: [], props: {} }),
  createComment: text => ({ type: '#comment', text, children: [], props: {} }),
  setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text },
  patchProp: (node, key, _old, value) => { node.props[key] = value },
  insert: (node, parent, anchor) => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node); node.parent = parent; const index = anchor ? parent.children.indexOf(anchor) : -1; if (index < 0) parent.children.push(node); else parent.children.splice(index, 0, node) },
  remove: node => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node) },
  parentNode: node => node.parent || null, nextSibling: node => node.parent?.children[node.parent.children.indexOf(node) + 1] || null,
})
function compile(component: any, relative: string) {
  const filename = new URL(relative, import.meta.url), { descriptor } = parse(readFileSync(filename, 'utf8'))
  const script = compileScript(descriptor, { id: relative })
  const template = compileTemplate({ id: relative, filename: filename.pathname, source: descriptor.template!.content, compilerOptions: { bindingMetadata: script.bindings, hoistStatic: false, isCustomElement: tag => ['view','text','input'].includes(tag) } })
  expect(template.errors).toEqual([])
  const javascript = ts.transpileModule(template.code, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText
  const code = javascript.replace(/import \{([^}]+)\} from ["']vue["'];?/g, (_all, bindings: string) => `const {${bindings.replace(/ as /g, ':')}}=Vue`).replace('export function render', 'function render')
  component.render = new Function('Vue', code + ';return render')(Vue)
}
compile(PreviewPage, '../pages/admin-customer-preview/admin-customer-preview.vue')
compile(EnvironmentBadge, '../components/EnvironmentBadge.vue')
compile(PullUpBrandFooter, '../components/PullUpBrandFooter.vue')
function text(node: Node): string { return (node.type === '#comment' ? '' : node.text || '') + node.children.map(text).join('') }
function find(node: Node, predicate: (node: Node) => boolean): Node[] { return [...(predicate(node) ? [node] : []), ...node.children.flatMap(child => find(child, predicate))] }
async function settle() { for (let i = 0; i < 20; i++) await Promise.resolve(); await nextTick() }
function mount() {
  const root: Node = { type: 'root', children: [], props: {} }, pinia = createPinia(), session = useSessionStore(pinia)
  session.setToken('employee-token')
  session.applyContext({ mini_user_id: 9, current_customer_id: 0, bindings: [], capabilities: [], account_type: 'employee', roles: ['admin'] })
  const app = renderer.createApp({ setup: () => () => h(PreviewPage) }); app.use(pinia); app.provide(ssrContextKey, { modules: new Set() }); app.mount(root)
  return { root, session, app, async show() { for (const fn of lifecycle.show) fn(); await settle() } }
}
beforeEach(() => {
  vi.resetAllMocks(); lifecycle.show = []
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development'); vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
  const storage = new Map<string, unknown>()
  vi.stubGlobal('uni', { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: unknown) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key), reLaunch: vi.fn() })
  api.list.mockResolvedValue({ rows: [{id: 74, name: '客户甲'}], has_next: false })
  api.start.mockResolvedValue({ token: 'preview-token' }); api.exit.mockResolvedValue({})
  api.me.mockResolvedValue({ mini_user_id: 0, current_customer_id: 74, current_customer_name: '客户甲', bindings: [], capabilities: [], account_type: 'preview' })
})
it('opens the selected customer without overwriting the employee token', async () => {
  const view = mount()
  try {
    await view.show()
    const button = find(view.root, node => node.type === 'button' && text(node) === '查看')[0]
    await button.props.onTap(); await settle()
    expect(api.start).toHaveBeenCalledWith('employee-token', 74)
    expect(view.session.token).toBe('preview-token')
    expect(view.session.isCustomerPreview).toBe(true)
    expect(uni.reLaunch).toHaveBeenCalledWith({url: '/pages/home/home'})
  } finally { view.app.unmount() }
})
it('rejects a context for a different customer and closes the failed preview', async () => {
  api.me.mockResolvedValue({ mini_user_id: 0, current_customer_id: 75, bindings: [], capabilities: [], account_type: 'preview' })
  const view = mount()
  try {
    await view.show()
    await find(view.root, node => node.type === 'button' && text(node) === '查看')[0].props.onTap(); await settle()
    expect(view.session.token).toBe('employee-token')
    expect(api.exit).toHaveBeenCalledWith('preview-token')
    expect(text(view.root)).toContain('客户预览身份不匹配')
    expect(uni.reLaunch).not.toHaveBeenCalled()
  } finally { view.app.unmount() }
})
