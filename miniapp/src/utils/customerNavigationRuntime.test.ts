import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, h, nextTick, ssrContextKey } from 'vue'
import { createPinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import ts from 'typescript'
import MainTabBar from '../components/MainTabBar.vue'
import EnvironmentBadge from '../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../components/PullUpBrandFooter.vue'
import Service from '../pages/service/service.vue'
import InventoryDetail from '../pages/customer-inventory-detail/customer-inventory-detail.vue'
import ProcessingDetail from '../pages/processing-request-detail/processing-request-detail.vue'
import { useSessionStore } from '../stores/session'

const hooks = vi.hoisted(() => ({ load: [] as Array<(query: Record<string, string>) => void>, show: [] as Array<() => void> }))
const api = vi.hoisted(() => ({ me: vi.fn(), services: vi.fn(), inventory: vi.fn(), batches: vi.fn(), processing: vi.fn(), processingDetail: vi.fn() }))
vi.mock('@dcloudio/uni-app', () => ({
  onLoad: (callback: (query: Record<string, string>) => void) => hooks.load.push(callback),
  onShow: (callback: () => void) => hooks.show.push(callback),
  onHide: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn(),
}))
vi.mock('../api/customerPortal', async importOriginal => ({
  ...await importOriginal<typeof import('../api/customerPortal')>(),
  fetchMe: api.me, fetchServicePage: api.services, fetchCustomerInventory: api.inventory,
  fetchCustomerInventoryBatches: api.batches, fetchProcessingRequests: api.processing,
  fetchProcessingRequestDetail: api.processingDetail,
}))
vi.mock('../components/CustomerDirectShipPanel.vue', async () => {
  const { h } = await import('vue')
  return { default: { props: ['showCreate'], setup: (props: { showCreate?: boolean }) => () => h('view', { 'data-shipment-panel': true }, `创建表单:${props.showCreate}`) } }
})

type Node = { type: string; children: Node[]; parent?: Node; props: Record<string, any>; text?: string; [key: string]: any }
const renderer = createRenderer<Node, Node>({
  createElement: type => ({ type, children: [], props: {}, ownerDocument: { activeElement: null }, tagName: type.toUpperCase(), addEventListener() {}, removeEventListener() {}, getRootNode() { return { activeElement: null } } }),
  createText: text => ({ type: '#text', text, children: [], props: {} }), createComment: text => ({ type: '#comment', text, children: [], props: {} }),
  setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text; node.children = [] }, patchProp: (node, key, _old, value) => { node.props[key] = value },
  insert: (node, parent, anchor) => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node); node.parent = parent; const index = anchor ? parent.children.indexOf(anchor) : -1; if (index < 0) parent.children.push(node); else parent.children.splice(index, 0, node) },
  remove: node => { if (node?.parent) node.parent.children = node.parent.children.filter(child => child !== node) }, parentNode: node => node?.parent || null,
  nextSibling: node => node?.parent?.children[node.parent.children.indexOf(node) + 1] || null,
})
function clientRender(component: any, path: string) {
  const { descriptor } = parse(readFileSync(new URL(path, import.meta.url), 'utf8'))
  const script = compileScript(descriptor, { id: path })
  const template = compileTemplate({ id: path, filename: path, source: descriptor.template!.content, compilerOptions: { hoistStatic: false, bindingMetadata: script.bindings, isCustomElement: tag => ['view', 'text', 'scroll-view', 'picker', 'switch', 'checkbox'].includes(tag) } })
  const code = ts.transpileModule(template.code, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText
    .replace(/import \{([^}]+)\} from ["']vue["'];?/g, (_all, bindings: string) => `const {${bindings.replace(/ as /g, ':')}}=Vue`).replace('export function render', 'function render')
  component.render = new Function('Vue', code + ';return render')(Vue)
}
clientRender(MainTabBar, '../components/MainTabBar.vue')
clientRender(EnvironmentBadge, '../components/EnvironmentBadge.vue')
clientRender(PullUpBrandFooter, '../components/PullUpBrandFooter.vue')
clientRender(Service, '../pages/service/service.vue')
clientRender(InventoryDetail, '../pages/customer-inventory-detail/customer-inventory-detail.vue')
clientRender(ProcessingDetail, '../pages/processing-request-detail/processing-request-detail.vue')

function text(node: Node): string { return (node.type === '#comment' ? '' : node.text || '') + node.children.map(text).join('') }
function find(node: Node, predicate: (node: Node) => boolean): Node[] { return [...(predicate(node) ? [node] : []), ...node.children.flatMap(child => find(child, predicate))] }
function mount(component: any, props: Record<string, unknown> = {}, capabilities = ['processing', 'settlement'], accountType = 'customer') {
  const root: Node = { type: 'root', children: [], props: {} }
  const app = renderer.createApp({ setup: () => () => h(component, props) })
  const pinia = createPinia()
  app.use(pinia)
  const session = useSessionStore(pinia)
  session.$patch({ token: 'isolated', currentCustomerID: 901, accountType, capabilities: capabilities.map(code => ({ code, enabled: true })) })
  app.provide(ssrContextKey, { modules: new Set() }); app.mount(root)
  return { app, root }
}
async function settle() { for (let i = 0; i < 12; i++) await Promise.resolve(); await nextTick() }
async function openPage(query: Record<string, string>) {
  hooks.load.forEach(callback => callback(query))
  await nextTick()
  hooks.show.forEach(callback => callback())
  await settle()
}
function click(node: Node) { node.props.onTap({ stopPropagation() {} }) }

beforeEach(() => {
  vi.clearAllMocks(); hooks.load.length = 0; hooks.show.length = 0
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'production')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://erp.qacoohee.com/app')
  vi.stubGlobal('Document', class Document {})
  vi.stubGlobal('ShadowRoot', class ShadowRoot {})
  vi.stubGlobal('uni', { getStorageSync: vi.fn(), setStorageSync: vi.fn(), removeStorageSync: vi.fn(), reLaunch: vi.fn(), navigateTo: vi.fn(), navigateBack: vi.fn(), setNavigationBarTitle: vi.fn(), showToast: vi.fn(), $on: vi.fn(), $off: vi.fn() })
  api.me.mockResolvedValue({ mini_user_id: 10, current_customer_id: 901, account_type: 'customer', capabilities: [{ code: 'processing', enabled: true }, { code: 'settlement', enabled: true }] })
  api.services.mockResolvedValue({ key: 'orders', title: '旧订单标题', orders: [], has_more: false })
  api.inventory.mockResolvedValue({ rows: [{ product_id: 7, spec_g: 454, warehouses: ['CUSTOMER-901'] }] })
  api.batches.mockResolvedValue({ rows: [{ product_id: 7, spec_g: 454, batch_id: 1, warehouse: 'CUSTOMER-901', related_orders: [{ order_id: 19, order_no: 'SO & 19', reserved_qty: 1 }] }] })
  api.processing.mockResolvedValue({ rows: [] })
  api.processingDetail.mockResolvedValue({ request: { id: 6, request_no: 'PR-6', status: 'pending', items: [{ id: 1, product_id: 7, spec_g: 454, related_orders: [{ order_id: 19, order_no: 'SO & 19', reserved_qty: 1 }] }] } })
})

describe('real customer navigation interactions', () => {
  it('lets a page marked home return to the actual home root', () => {
    const { app, root } = mount(MainTabBar, { current: 'home' })
    try { click(find(root, node => String(node.props.class || '').includes('tab-item') && text(node) === '首页')[0]); expect(uni.reLaunch).toHaveBeenCalledWith({ url: '/pages/index/index' }) }
    finally { app.unmount() }
  })

  it('lets processing shipment pages open official orders even when both highlight orders', () => {
    const { app, root } = mount(MainTabBar, { current: 'orders' })
    try { click(find(root, node => String(node.props.class || '').includes('tab-item') && text(node) === '我的订单')[0]); expect(uni.reLaunch).toHaveBeenCalledWith({ url: '/pages/service/service?key=orders&source=official' }) }
    finally { app.unmount() }
  })

  it('hides fees without settlement and hides the entire customer tab bar for employees', () => {
    let page = mount(MainTabBar, { current: 'home' }, ['processing'])
    try { expect(text(page.root)).not.toContain('费用'); expect(text(page.root)).toContain('我的订单') }
    finally { page.app.unmount() }
    page = mount(MainTabBar, { current: 'home' }, ['settlement'], 'employee')
    try { expect(text(page.root)).toBe(''); expect(find(page.root, node => String(node.props.class || '').includes('tab-item'))).toHaveLength(0) }
    finally { page.app.unmount() }
  })

  it('offers processing shipments from official orders and keeps that destination read-only', async () => {
    const { app, root } = mount(Service)
    try {
      await openPage({ key: 'orders', source: 'official' })
      expect(find(root, node => node.type === 'text' && node.props.class === 'title').map(text)).toEqual(['我的订单'])
      const button = find(root, node => node.type === 'button' && text(node) === '代加工发货')[0]
      expect(button).toBeDefined(); click(button)
      expect(uni.navigateTo).toHaveBeenCalledWith({ url: '/pages/service/service?key=orders' })
      await openPage({ key: 'orders' })
      expect(find(root, node => node.type === 'text' && node.props.class === 'title').map(text)).toEqual(['代加工发货'])
      expect(find(root, node => node.props['data-shipment-panel']).map(text)).toEqual(['创建表单:false'])
      click(find(root, node => String(node.props.class || '').includes('tab-item') && text(node) === '我的订单')[0])
      expect(uni.reLaunch).toHaveBeenCalledWith({ url: '/pages/service/service?key=orders&source=official' })
    } finally { app.unmount() }
  })

  it('keeps the processing-only shipment shortcut out of ordinary customers orders', async () => {
    api.me.mockResolvedValue({ current_customer_id: 901, account_type: 'customer', capabilities: [{ code: 'product_order', enabled: true }] })
    const { app, root } = mount(Service, {}, ['product_order'])
    try {
      await openPage({ key: 'orders', source: 'official' })
      expect(find(root, node => node.type === 'text' && node.props.class === 'title').map(text)).toEqual(['我的订单'])
      expect(find(root, node => node.type === 'button' && text(node) === '代加工发货')).toHaveLength(0)
    }
    finally { app.unmount() }
  })

  it('opens an inventory related order in official orders and preserves the encoded keyword', async () => {
    const { app, root } = mount(InventoryDetail)
    try {
      await openPage({ product_id: '7', spec_g: '454' })
      const order = find(root, node => node.props.class === 'related-order')[0]
      expect(order).toBeDefined(); click(order)
      expect(uni.navigateTo).toHaveBeenCalledWith({ url: '/pages/service/service?key=orders&source=official&q=SO%20%26%2019' })
    } finally { app.unmount() }
  })

  it('opens a processing related order in official orders and preserves the encoded keyword', async () => {
    const { app, root } = mount(ProcessingDetail)
    try {
      await openPage({ id: '6' })
      const order = find(root, node => node.props.class === 'order')[0]
      expect(order).toBeDefined(); click(order)
      expect(uni.navigateTo).toHaveBeenCalledWith({ url: '/pages/service/service?key=orders&source=official&q=SO%20%26%2019' })
    } finally { app.unmount() }
  })
})
