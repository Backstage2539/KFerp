import { beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, nextTick, reactive, ssrContextKey } from 'vue'
import { readFileSync } from 'node:fs'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import ts from 'typescript'

const hooks = vi.hoisted(() => ({ show: [] as Array<() => void>, request: vi.fn(), fetchMe: vi.fn(), session: null as any }))
vi.mock('@dcloudio/uni-app', () => ({ onShow: (fn: () => void) => hooks.show.push(fn), onHide: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn() }))
vi.mock('../api/client', () => ({ miniRequest: hooks.request, MiniRequestError: class extends Error {} }))
vi.mock('../api/customerPortal', () => ({ fetchMe: hooks.fetchMe }))
vi.mock('../stores/session', () => ({ useSessionStore: () => hooks.session }))
vi.mock('../utils/miniappShare', () => ({ defaultMiniappShare: vi.fn(), defaultMiniappTimelineShare: vi.fn(), refreshMiniappShareMenu: vi.fn() }))
vi.mock('../components/EnvironmentBadge.vue', () => ({ default: { render: () => null } }))
vi.mock('../components/PullUpBrandFooter.vue', () => ({ default: { render: () => null } }))
import Page from '../pages/order-notifications/order-notifications.vue'

type Node = { type: string; text?: string; props: Record<string, any>; children: Node[]; parent?: Node }
const renderer = createRenderer<Node, Node>({
  createElement: type => ({ type, props: {}, children: [] }), createText: text => ({ type: '#text', text, props: {}, children: [] }), createComment: text => ({ type: '#comment', text, props: {}, children: [] }),
  setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text; node.children = [] }, patchProp: (node, key, _old, value) => { node.props[key] = value },
  insert: (node, parent) => { node.parent = parent; parent.children.push(node) }, remove: node => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node) }, parentNode: node => node.parent || null, nextSibling: () => null,
})
function findText(node: Node, text: string): boolean { return Boolean(node.text?.includes(text) || node.children.some(child => findText(child, text))) }
async function flush() { for (let i = 0; i < 5; i++) { await Promise.resolve(); await nextTick() } }

beforeEach(() => {
  hooks.show.length = 0
  hooks.request.mockReset()
  hooks.fetchMe.mockReset().mockResolvedValue({})
  hooks.session = reactive({ token: 'token', miniUserID: 7, currentCustomerID: 2, employeeID: 0, accountType: 'customer', capabilities: [{ code: 'direct_ship', enabled: true }], applyContext: vi.fn() })
  vi.stubGlobal('uni', { redirectTo: vi.fn(), previewImage: vi.fn() })
})

it('displays binding conflict guidance and clears it after a successful reload', async () => {
  const response = { enabled: true, mini_user_id: 7, customer_id: 2, employee_id: 0, prompt_version: 1, rows: [], choice: null }
  hooks.request.mockResolvedValueOnce({ ...response, binding_error: '微信身份暂时无法自动关联，请在订单通知设置中查看' }).mockResolvedValueOnce({ ...response, binding_error: '' })
  const filename = '../pages/order-notifications/order-notifications.vue'
  const { descriptor } = parse(readFileSync(new URL(filename, import.meta.url), 'utf8'))
  const script = compileScript(descriptor, { id: 'notification-settings' })
  const template = compileTemplate({ id: 'notification-settings', filename, source: descriptor.template!.content, compilerOptions: { hoistStatic: false, bindingMetadata: script.bindings, isCustomElement: tag => ['view','text','button','label','switch','checkbox','image'].includes(tag) } })
  const code = ts.transpileModule(template.code, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText.replace(/import \{([^}]+)\} from ["']vue["'];?/g, (_all, bindings: string) => `const {${bindings.replace(/ as /g, ':')}}=Vue`).replace('export function render', 'function render')
  Page.render = new Function('Vue', code + ';return render')(Vue)
  const root: Node = { type: 'root', props: {}, children: [] }
  const app = renderer.createApp(Page)
  app.provide(ssrContextKey, { modules: new Set() })
  app.mount(root)
  try {
    hooks.show.forEach(fn => fn()); await flush()
    expect(findText(root, '微信身份暂时无法自动关联')).toBe(true)
    expect(findText(root, '请联系管理员处理')).toBe(true)
    hooks.show.forEach(fn => fn()); await flush()
    expect(findText(root, '微信身份暂时无法自动关联')).toBe(false)
  } finally { app.unmount() }
})
