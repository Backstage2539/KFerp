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
  onHide: vi.fn(), onPageScroll: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn(),
}))
vi.mock('../utils/registrationSession', () => ({ restoreRegistrationSession: vi.fn(async () => {}) }))
vi.mock('../components/CustomerPricePreview.vue', async () => {
  const { h } = await import('vue')
  return { default: { props: ['usageCode'], setup: (props: { usageCode: string }) => () => h('view', { 'data-price-preview': props.usageCode }, `价表：${props.usageCode}`) } }
})
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
  return { app, root, pinia, session }
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
  vi.stubGlobal('uni', { getStorageSync: vi.fn(), setStorageSync: vi.fn(), removeStorageSync: vi.fn(), reLaunch: vi.fn(), navigateTo: vi.fn(), navigateBack: vi.fn(), setNavigationBarTitle: vi.fn(), pageScrollTo: vi.fn(), showToast: vi.fn(), $on: vi.fn(), $off: vi.fn() })
  api.me.mockResolvedValue({ mini_user_id: 10, current_customer_id: 901, account_type: 'customer', capabilities: [{ code: 'processing', enabled: true }, { code: 'settlement', enabled: true }] })
  api.services.mockResolvedValue({ key: 'orders', title: '旧订单标题', orders: [], has_more: false })
  api.inventory.mockResolvedValue({ rows: [{ product_id: 7, spec_g: 454, warehouses: ['CUSTOMER-901'] }] })
  api.batches.mockResolvedValue({ rows: [{ product_id: 7, spec_g: 454, batch_id: 1, warehouse: 'CUSTOMER-901', related_orders: [{ order_id: 19, order_no: 'SO & 19', reserved_qty: 1 }] }] })
  api.processing.mockResolvedValue({ rows: [] })
  api.processingDetail.mockResolvedValue({ request: { id: 6, request_no: 'PR-6', status: 'pending', items: [{ id: 1, product_id: 7, spec_g: 454, related_orders: [{ order_id: 19, order_no: 'SO & 19', reserved_qty: 1 }] }] } })
})

describe('real customer navigation interactions', () => {
  it('does not let an older order error clear the latest loading state', async () => {
    let rejectOld!: (error: Error) => void
    let resolveNew!: (value: unknown) => void
    api.services.mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectOld = reject }))
      .mockImplementationOnce(() => new Promise(resolve => { resolveNew = resolve }))
    const { app, root } = mount(Service)
    try {
      await openPage({ key: 'orders' })
      hooks.show.forEach(callback => callback()); await settle()
      expect(api.services).toHaveBeenCalledTimes(2)
      rejectOld(new Error('过期的错误')); await settle()
      expect(text(root)).toContain('加载中')
      expect(text(root)).not.toContain('过期的错误')
      resolveNew({ key: 'orders', title: '我的订单', orders: [], has_more: false }); await settle()
      expect(text(root)).not.toContain('加载中')
    } finally { app.unmount() }
  })

  it('does not overwrite another customer order list with a stale response', async () => {
    let resolveOld!: (value: unknown) => void
    let resolveNew!: (value: unknown) => void
    api.services.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
      .mockImplementationOnce(() => new Promise(resolve => { resolveNew = resolve }))
    const { app, root, session } = mount(Service)
    try {
      await openPage({ key: 'orders' })
      session.currentCustomerID = 902
      await nextTick()
      hooks.show.forEach(callback => callback()); await settle()
      expect(api.services).toHaveBeenCalledTimes(2)
      resolveNew({ key: 'orders', title: '我的订单', orders: [{ id: 2, order_no: 'NEW-CUSTOMER', receiver_address: '地址 B' }], has_more: false }); await settle()
      expect(text(root)).toContain('NEW-CUSTOMER')
      resolveOld({ key: 'orders', title: '我的订单', orders: [{ id: 1, order_no: 'OLD-CUSTOMER', receiver_address: '地址 A' }], has_more: false }); await settle()
      expect(text(root)).toContain('NEW-CUSTOMER')
      expect(text(root)).not.toContain('OLD-CUSTOMER')
    } finally { app.unmount() }
  })
  it('retains customer price usages after opening orders with a partial context', async () => {
    const binding = { customer_id: 901, customer_name: '测试客户', role: 'owner', status: 'approved' }
    const context = { mini_user_id: 10, current_customer_id: 901, account_type: 'customer', bindings: [binding], capabilities: [{ code: 'direct_ship', enabled: true }], fulfillment_modes: ['stock', 'on_demand'] }
    api.me.mockResolvedValue(context)
    api.services.mockResolvedValue({ key: 'orders', title: '我的订单', orders: [], has_more: false, theme_key: 'clean_ops', current_customer_id: 901 })
    const orderPage = mount(Service, {}, ['direct_ship'])
    orderPage.session.applyContext(context as Parameters<typeof orderPage.session.applyContext>[0])
    try {
      await openPage({ key: 'orders', source: 'official' })
      expect(orderPage.session.fulfillmentModes).toEqual(['stock', 'on_demand'])
      orderPage.app.unmount()
      hooks.load.length = 0; hooks.show.length = 0
      const BeanCenter = (await import('../pages/bean-list-center/bean-list-center.vue')).default
      clientRender(BeanCenter, '../pages/bean-list-center/bean-list-center.vue')
      const root: Node = { type: 'root', children: [], props: {} }
      const app = renderer.createApp({ setup: () => () => h(BeanCenter) })
      app.use(orderPage.pinia); app.provide(ssrContextKey, { modules: new Set() }); app.mount(root)
      try {
        hooks.show.forEach(callback => callback()); await settle()
        expect(text(root)).toContain('下单价格表')
        expect(text(root)).toContain('一件代发价格表')
        expect(find(root, node => node.props['data-price-preview'] === 'product_order')).toHaveLength(1)
      } finally { app.unmount() }
    } finally { orderPage.app.unmount() }
  })
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

  it('uses one searchable order page for processing customers from either existing entry', async () => {
    const { app, root } = mount(Service)
    try {
      for (const query of [{ key: 'orders', source: 'official' }, { key: 'orders' }] as Array<Record<string, string>>) {
        await openPage(query)
        expect(find(root, node => node.type === 'text' && node.props.class === 'title').map(text)).toEqual(['我的订单'])
        expect(find(root, node => node.props['data-shipment-panel'])).toHaveLength(0)
        expect(find(root, node => node.type === 'button' && text(node) === '代加工发货')).toHaveLength(0)
        expect(find(root, node => node.type === 'button' && text(node) === '订货')).toHaveLength(1)
        const dropship = find(root, node => node.type === 'button' && text(node) === '代发')[0]
        expect(dropship).toBeDefined()
        click(dropship)
        await settle()
        expect(api.services).toHaveBeenLastCalledWith('isolated', 'orders', expect.objectContaining({ order_kind: 'dropship', page: 1 }))
      }
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
