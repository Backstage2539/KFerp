import { beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, h, nextTick, ssrContextKey } from 'vue'
import { createPinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import ts from 'typescript'
import Home from '../pages/home/home.vue'
import Index from '../pages/index/index.vue'
import Profile from '../pages/profile/profile.vue'
import GuestHome from '../components/GuestHome.vue'
import MainTabBar from '../components/MainTabBar.vue'
import EnvironmentBadge from '../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../components/PullUpBrandFooter.vue'
import RegistrationProfile from '../components/RegistrationProfile.vue'
import { useSessionStore } from '../stores/session'

const lifecycle = vi.hoisted(() => ({ show: [] as Array<() => unknown> }))
const api = vi.hoisted(() => ({
  me: vi.fn(), account: vi.fn(), assets: vi.fn(), service: vi.fn(), processing: vi.fn(),
  staff: vi.fn(), switchCustomer: vi.fn(), employeeShare: vi.fn(),
}))
vi.mock('@dcloudio/uni-app', () => ({
  onShow: (callback: () => unknown) => { lifecycle.show.push(callback) },
  onHide: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn(),
}))
vi.mock('../api/customerPortal', () => ({
  fetchMe: api.me, fetchCustomerAccount: api.account, fetchCustomerAssetInventory: api.assets,
  fetchServicePage: api.service, fetchProcessingRequests: api.processing,
  switchCurrentCustomer: api.switchCustomer, fetchEmployeeShareSettings: api.employeeShare,
  saveEmployeeShareSettings: vi.fn(), saveEmployeeShareScope: vi.fn(),
}))
vi.mock('../api/samples', () => ({ fetchStaffNotifications: api.staff }))
vi.mock('../utils/registrationSession', () => ({ restoreRegistrationSession: vi.fn(async () => {}) }))
vi.mock('../utils/miniappShare', () => ({
  defaultMiniappShare: vi.fn(), defaultMiniappTimelineShare: vi.fn(), refreshMiniappShareMenu: vi.fn(async () => {}),
}))

type Node = { type: string; children: Node[]; parent?: Node; props: Record<string, any>; text?: string; [key: string]: any }
const renderer = createRenderer<Node, Node>({
  createElement: type => ({ type, children: [], props: {}, tagName: type.toUpperCase(), addEventListener() {}, removeEventListener() {}, getRootNode() { return { activeElement: null } } }),
  createText: text => ({ type: '#text', text, children: [], props: {} }),
  createComment: text => ({ type: '#comment', text, children: [], props: {} }),
  setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text },
  patchProp: (node, key, _old, value) => { node.props[key] = value },
  insert: (node, parent, anchor) => {
    if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node)
    node.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    if (index < 0) parent.children.push(node)
    else parent.children.splice(index, 0, node)
  },
  remove: node => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node) },
  parentNode: node => node.parent || null,
  nextSibling: node => node.parent?.children[node.parent.children.indexOf(node) + 1] || null,
})
function clientRender(component: any, relative: string) {
  const filename = new URL(relative, import.meta.url)
  const { descriptor } = parse(readFileSync(filename, 'utf8'))
  const script = compileScript(descriptor, { id: relative })
  const template = compileTemplate({ id: relative, filename: filename.pathname, source: descriptor.template!.content, compilerOptions: {
    bindingMetadata: script.bindings,
    hoistStatic: false,
    isCustomElement: tag => ['view', 'text', 'image', 'picker', 'radio-group', 'radio', 'switch'].includes(tag),
  } })
  expect(template.errors).toEqual([])
  const javascript = ts.transpileModule(template.code, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText
  const code = javascript.replace(/import \{([^}]+)\} from ["']vue["'];?/g, (_all, bindings: string) => `const {${bindings.replace(/ as /g, ':')}}=Vue`).replace('export function render', 'function render')
  component.render = new Function('Vue', code + ';return render')(Vue)
}
for (const [component, path] of [
  [Home, '../pages/home/home.vue'], [Index, '../pages/index/index.vue'], [Profile, '../pages/profile/profile.vue'],
  [GuestHome, '../components/GuestHome.vue'], [MainTabBar, '../components/MainTabBar.vue'],
  [EnvironmentBadge, '../components/EnvironmentBadge.vue'], [PullUpBrandFooter, '../components/PullUpBrandFooter.vue'],
  [RegistrationProfile, '../components/RegistrationProfile.vue'],
] as Array<[any, string]>) clientRender(component, path)

function text(node: Node): string { return (node.type === '#comment' ? '' : node.text || '') + node.children.map(text).join('') }
function find(node: Node, predicate: (node: Node) => boolean): Node[] { return [...(predicate(node) ? [node] : []), ...node.children.flatMap(child => find(child, predicate))] }
function hasClass(node: Node, name: string) { return String(node.props.class || '').split(/\s+/).includes(name) }
function labels(root: Node, name: string) { return find(root, node => hasClass(node, name)).map(text) }
async function settle() { for (let index = 0; index < 30; index++) await Promise.resolve(); await nextTick() }
const allCapabilities = ['product_order', 'direct_ship', 'processing', 'inventory_custody', 'settlement'].map(code => ({ code, enabled: true }))
function context(extra: Record<string, unknown> = {}) {
  return { mini_user_id: 91, current_customer_id: 901, current_customer_name: '测试客户', account_type: 'customer', theme_key: 'coffee_factory',
    bindings: [{ customer_id: 901, customer_name: '测试客户', role: 'owner', status: 'approved' }], capabilities: allCapabilities, fulfillment_modes: ['stock', 'on_demand'], ...extra }
}
function mount(component: any, value = context(), signedIn = true) {
  const root: Node = { type: 'root', children: [], props: {} }
  const pinia = createPinia()
  const session = useSessionStore(pinia)
  if (signedIn) { session.setToken('isolated-session'); session.applyContext(value) }
  api.me.mockResolvedValue(value)
  const app = renderer.createApp({ setup: () => () => h(component) })
  app.use(pinia); app.provide(ssrContextKey, { modules: new Set() }); app.mount(root)
  return { root, app, session, async show() { for (const callback of lifecycle.show) await callback(); await settle() } }
}
beforeEach(() => {
  vi.resetAllMocks()
  lifecycle.show = []
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'production')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://erp.qacoohee.com/app')
  vi.stubGlobal('Document', class Document {})
  vi.stubGlobal('ShadowRoot', class ShadowRoot {})
  const storage = new Map<string, unknown>()
  vi.stubGlobal('uni', {
    getStorageSync: vi.fn(key => storage.get(key)), setStorageSync: vi.fn((key, value) => storage.set(key, value)),
    removeStorageSync: vi.fn(key => storage.delete(key)), reLaunch: vi.fn(), navigateTo: vi.fn(), showToast: vi.fn(),
  })
  api.account.mockResolvedValue({ settlements: [], summary: { due_cents: 0 } })
  api.assets.mockResolvedValue({ rows: [] })
  api.service.mockResolvedValue({ orders: [] })
  api.processing.mockResolvedValue({ rows: [] })
  api.staff.mockResolvedValue({ unread_count: 2 })
})

it.each(['coffee_factory', 'clean_ops', 'premium_partner'])('composed %s customer home keeps capability entries once without overview or price banners', async theme => {
  const view = mount(Home, context({ theme_key: theme }))
  try {
    await view.show()
    expect(labels(view.root, 'entry')).toEqual(['一件代发', '无库存代发', '生产工单', '我的库存'])
    expect(labels(view.root, 'tab-label')).toEqual(['首页', '价格表', '我的订单', '费用', '我的'])
    expect(find(view.root, node => hasClass(node, `theme-${theme.replaceAll('_', '-')}`))).toHaveLength(1)
    for (const duplicate of ['待处理订单', '生产进度', '客户库存', '待对账', '业务摘要加载中', '价格表中心 · 查看咖啡价格表']) expect(text(view.root)).not.toContain(duplicate)
    expect(find(view.root, node => hasClass(node, 'overview-grid') || hasClass(node, 'bean-entry'))).toHaveLength(0)
  } finally { view.app.unmount() }
})

it('customer home refresh requests identity only and never the removed overview APIs', async () => {
  const view = mount(Home)
  try {
    await view.show()
    expect(api.me).toHaveBeenCalledTimes(1)
    for (const request of [api.account, api.assets, api.service, api.processing]) expect(request).not.toHaveBeenCalled()
  } finally { view.app.unmount() }
})

it('ordinary customer home keeps authorized order entry while hiding ungranted inventory and dropship scenes', async () => {
  const view = mount(Home, context({ capabilities: [{ code: 'product_order', enabled: true }], fulfillment_modes: [] }))
  try {
    await view.show()
    expect(labels(view.root, 'entry')).toEqual(['商品下单'])
    const entry = find(view.root, node => hasClass(node, 'entry'))[0]
    await entry.props.onTap()
    expect(uni.navigateTo).toHaveBeenCalledWith({ url: '/pages/service/service?key=productOrder' })
    expect(text(view.root)).not.toContain('价格表中心 · 查看咖啡价格表')
  } finally { view.app.unmount() }
})

it.each([
  { role: 'sales', permissions: ['customers.read', 'customers.write'], entries: ['价格表中心', '样品工单', '员工通知 (2)', '录单', '查看订单', '客户维护', '个人中心'] },
  { role: 'warehouse', permissions: [], entries: ['录单', '查看订单', '个人中心'] },
])('composed $role employee home preserves staff entries without customer overview or tabs', async ({ role, permissions, entries }) => {
  const view = mount(Home, context({ account_type: 'employee', current_customer_id: 0, employee_id: 92, employee_name: '测试员工', roles: [role], permissions, capabilities: [] }))
  try {
    await view.show()
    expect(labels(view.root, 'entry')).toEqual(entries)
    expect(labels(view.root, 'tab-label')).toEqual([])
    expect(text(view.root)).not.toContain('待处理订单')
    expect(text(view.root)).not.toContain('价格表中心 · 查看咖啡价格表')
    for (const request of [api.account, api.assets, api.service, api.processing]) expect(request).not.toHaveBeenCalled()
  } finally { view.app.unmount() }
})

it('composed guest startup has one price-table tab and retains service guides, calculator and login', async () => {
  const view = mount(Index, context(), false)
  try {
    await view.show()
    expect(labels(view.root, 'tab-label')).toEqual(['首页', '价格表', '我的'])
    expect(find(view.root, node => hasClass(node, 'bean-entry'))).toHaveLength(0)
    expect(text(view.root)).not.toContain('价格表中心 · 查看咖啡价格表')
    expect(text(view.root)).toContain('了解我们的服务')
    expect(text(view.root)).toContain('包装数量换算')
    const login = find(view.root, node => node.type === 'button' && text(node) === '登录，查看我的业务')[0]
    await login.props.onTap()
    expect(uni.navigateTo).toHaveBeenCalledWith({ url: '/pages/login/login?mode=password' })
  } finally { view.app.unmount() }
})

it.each(['customer', 'employee'])('composed %s profile exposes one logout action which clears the persisted session', async accountType => {
  const view = mount(Profile, context({ account_type: accountType, roles: [], permissions: [] }))
  try {
    await view.show()
    expect(find(view.root, node => node.type === 'button' && text(node) === '切换用户')).toHaveLength(0)
    const logout = find(view.root, node => node.type === 'button' && text(node) === '退出登录')
    expect(logout).toHaveLength(1)
    await logout[0].props.onTap()
    expect(view.session.token).toBe('')
    expect(view.session.autoRestoreBlocked).toBe(true)
    expect(uni.removeStorageSync).toHaveBeenCalledWith('kferp.mini.production.token')
    expect(uni.reLaunch).toHaveBeenCalledWith({ url: '/pages/login/login?return_to=%2Fpages%2Fprofile%2Fprofile&mode=password' })
  } finally { view.app.unmount() }
})

it('composed customer profile preserves the real bound-customer switch without signing out', async () => {
  const bindings = [{ customer_id: 901, customer_name: '测试客户', role: 'owner', status: 'approved' }, { customer_id: 902, customer_name: '另一客户', role: 'member', status: 'approved' }]
  const view = mount(Profile, context({ bindings }))
  api.switchCustomer.mockResolvedValue(context({ current_customer_id: 902, current_customer_name: '另一客户', bindings }))
  try {
    await view.show()
    const picker = find(view.root, node => node.type === 'picker' && Array.isArray(node.props.range))[0]
    await picker.props.onChange({ detail: { value: 1 } })
    await settle()
    expect(api.switchCustomer).toHaveBeenCalledWith('isolated-session', 902)
    expect(view.session.currentCustomerID).toBe(902)
    expect(view.session.token).toBe('isolated-session')
    expect(view.session.autoRestoreBlocked).toBe(false)
    expect(uni.reLaunch).toHaveBeenCalledWith({ url: '/pages/home/home' })
  } finally { view.app.unmount() }
})
