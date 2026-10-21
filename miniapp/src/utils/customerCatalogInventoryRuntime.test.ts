import { beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, h, nextTick, ssrContextKey } from 'vue'
import { createPinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import ts from 'typescript'
import Inventory from '../components/CustomerInventoryPanel.vue'
import DirectShip from '../components/CustomerDirectShipPanel.vue'
import ProductPicker from '../components/ProductFamilyPickerSheet.vue'

const api = vi.hoisted(() => ({ assets: vi.fn(), stock: vi.fn(), directCatalog: vi.fn(), productCatalog: vi.fn(), recipients: vi.fn() }))
vi.mock('../api/customerPortal', () => ({
  fetchCustomerAssetInventory: api.assets, fetchCustomerInventory: api.stock,
  fetchDirectShipCatalog: api.directCatalog, fetchProductOrderCatalog: api.productCatalog,
  fetchRecipientAddresses: api.recipients, fetchDirectShipRequests: vi.fn(),
  cancelDirectShipRequest: vi.fn(), createDirectShipRequest: vi.fn(), createProductOrder: vi.fn(),
  previewDirectShipRequest: vi.fn(), previewProductOrder: vi.fn(),
}))
vi.mock('@dcloudio/uni-app', () => ({ onShow: vi.fn(), onHide: vi.fn() }))

type Node = { type: string; children: Node[]; parent?: Node; props: Record<string, any>; text?: string; [key: string]: any }
const renderer = createRenderer<Node, Node>({ createElement: type => ({ type, children: [], props: {}, tagName: type.toUpperCase(), addEventListener() {}, removeEventListener() {} }), createText: text => ({ type: '#text', text, children: [], props: {} }), createComment: text => ({ type: '#comment', text, children: [], props: {} }), setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text }, patchProp: (node, key, _old, value) => { node.props[key] = value }, insert: (node, parent, anchor) => { node.parent = parent; const index = anchor ? parent.children.indexOf(anchor) : -1; if (index < 0) parent.children.push(node); else parent.children.splice(index, 0, node) }, remove: node => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node) }, parentNode: node => node.parent || null, nextSibling: () => null })
function clientRender(component: any, filename: string) {
  const { descriptor } = parse(readFileSync(new URL(`../components/${filename}.vue`, import.meta.url), 'utf8'))
  const script = compileScript(descriptor, { id: filename })
  const template = compileTemplate({ id: filename, filename: `${filename}.vue`, source: descriptor.template!.content, compilerOptions: { bindingMetadata: script.bindings, isCustomElement: tag => ['view', 'text', 'scroll-view', 'checkbox', 'picker', 'switch'].includes(tag) } })
  const javascript = ts.transpileModule(template.code, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText
  const code = javascript.replace(/import \{([^}]+)\} from ["']vue["'];?/g, (_all, bindings: string) => `const {${bindings.replace(/ as /g, ':')}}=Vue`).replace('export function render', 'function render')
  component.render = new Function('Vue', code + ';return render')(Vue)
}
clientRender(Inventory, 'CustomerInventoryPanel')
clientRender(DirectShip, 'CustomerDirectShipPanel')
clientRender(ProductPicker, 'ProductFamilyPickerSheet')
function mount(component: any, props: Record<string, unknown>) {
  const root: Node = { type: 'root', children: [], props: {} }
  const app = renderer.createApp({ setup: () => () => h(component, { token: 'isolated', customerId: 901, ...props }) })
  app.use(createPinia()); app.provide(ssrContextKey, { modules: new Set() }); app.mount(root)
  return { app, root }
}
function text(node: Node): string { return (node.type === '#comment' ? '' : node.text || '') + node.children.map(text).join('') }
function find(node: Node, predicate: (node: Node) => boolean): Node[] { return [...(predicate(node) ? [node] : []), ...node.children.flatMap(child => find(child, predicate))] }
async function settle() { for (let i = 0; i < 8; i++) await Promise.resolve(); await nextTick() }

beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('uni', { $on: vi.fn(), $off: vi.fn(), navigateTo: vi.fn(), showToast: vi.fn() })
  api.assets.mockResolvedValue({ rows: [], inventory_types: ['finished_product'], warehouses: [{ warehouse_code: 'CUSTOMER-901-FINISHED', warehouse_name: '当前客户成品仓', kind: 'finished' }] })
  api.stock.mockResolvedValue({ rows: [], total_pages: 1 })
  api.recipients.mockResolvedValue({ rows: [] })
})

it('real inventory page shows only the authorized finished category and own warehouse when stock is empty', async () => {
  const { app, root } = mount(Inventory, {})
  try {
    await settle()
    expect(find(root, node => node.type === 'button' && String(node.props.class || '').includes('type-tab')).map(text)).toEqual(['全部', '成品'])
    expect(text(root)).toContain('当前客户成品仓')
    expect(text(root)).not.toContain('生豆、包材和半成品')
  } finally { app.unmount() }
})

it('real inventory page does not treat a failed scope request as missing warehouse authorization', async () => {
  api.assets.mockRejectedValue(new Error('登录状态已失效'))
  const { app, root } = mount(Inventory, {})
  try {
    await settle()
    expect(text(root)).toContain('登录状态已失效')
    expect(text(root)).not.toContain('尚无获权仓库')
    expect(find(root, node => node.type === 'button' && String(node.props.class || '').includes('type-tab')).map(text)).toEqual(['全部'])
  } finally { app.unmount() }
})

it('real product-order entry reports capability denial without claiming the bound price table is missing', async () => {
  api.productCatalog.mockRejectedValue(new Error('capability not enabled'))
  const { app, root } = mount(DirectShip, { orderMode: 'product_order' })
  try {
    await settle()
    expect(text(root)).toContain('尚未开通商品下单')
    expect(text(root)).not.toContain('ERP 尚未绑定可用价格表')
  } finally { app.unmount() }
})

it('real product-order entry distinguishes pending catalog loading from an unbound policy', async () => {
  let resolve!: (value: unknown) => void
  api.productCatalog.mockImplementation(() => new Promise(done => { resolve = done }))
  const { app, root } = mount(DirectShip, { orderMode: 'product_order' })
  try {
    await settle()
    expect(text(root)).toContain('价格表加载中')
    expect(text(root)).not.toContain('ERP 尚未绑定可用价格表')
    resolve({ current_customer_id: 901, product_families: [], price_tables: [{ id: 71, table_name: '客户已绑定表', version_no: 'V1.0', list_type: 'retail' }] })
    await settle()
    expect(text(root)).toContain('客户已绑定表')
    expect(text(root)).not.toContain('价格表加载中')
  } finally { app.unmount() }
})

it('real inventory dropship entry displays authoritative bound table without charging purchased goods twice', async () => {
  api.directCatalog.mockResolvedValue({ current_customer_id: 901, fulfillment_mode: 'stock', fulfillment_settings: { customer_id: 901, revision: 1, modes: [{ mode: 'stock', enabled: true, warehouse_code: 'CUSTOMER-901-FINISHED' }] }, product_families: [], price_tables: [{ id: 71, table_name: '客户已绑定表', version_no: 'V1.0', list_type: 'retail' }] })
  const { app, root } = mount(DirectShip, { entryMode: 'stock' })
  try {
    await settle()
    expect(text(root)).toContain('客户已绑定表')
    expect(text(root)).toContain('库存代发不重复收取商品货款')
    expect(text(root)).not.toContain('ERP 尚未绑定可用价格表')
  } finally { app.unmount() }
})
