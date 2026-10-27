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
import { useCustomerOrderDraftStore } from '../stores/customerOrderDraft'
import { createDirectShipDraftLine, selectDirectShipDraftProduct } from './directShipDraft'

const api = vi.hoisted(() => ({ requests: vi.fn(), assets: vi.fn(), stock: vi.fn(), directCatalog: vi.fn(), productCatalog: vi.fn(), recipients: vi.fn(), previewProductOrder: vi.fn(), createProductOrder: vi.fn(), previewDirectShip: vi.fn() }))
vi.mock('../api/customerPortal', () => ({
  fetchCustomerAssetInventory: api.assets, fetchCustomerInventory: api.stock,
  fetchDirectShipCatalog: api.directCatalog, fetchProductOrderCatalog: api.productCatalog,
  fetchRecipientAddresses: api.recipients, fetchDirectShipRequests: api.requests,
  cancelDirectShipRequest: vi.fn(), createDirectShipRequest: vi.fn(), createProductOrder: api.createProductOrder,
  previewDirectShipRequest: api.previewDirectShip, previewProductOrder: api.previewProductOrder,
}))
vi.mock('@dcloudio/uni-app', () => ({ onShow: vi.fn(), onHide: vi.fn() }))

type Node = { type: string; children: Node[]; parent?: Node; props: Record<string, any>; text?: string; [key: string]: any }
const renderer = createRenderer<Node, Node>({ createElement: type => ({ type, children: [], props: {}, ownerDocument: { activeElement: null }, tagName: type.toUpperCase(), addEventListener() {}, removeEventListener() {}, getRootNode() { return { activeElement: null } } }), createText: text => ({ type: '#text', text, children: [], props: {} }), createComment: text => ({ type: '#comment', text, children: [], props: {} }), setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text }, patchProp: (node, key, _old, value) => { node.props[key] = value }, insert: (node, parent, anchor) => { node.parent = parent; const index = anchor ? parent.children.indexOf(anchor) : -1; if (index < 0) parent.children.push(node); else parent.children.splice(index, 0, node) }, remove: node => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node) }, parentNode: node => node.parent || null, nextSibling: () => null })
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
function mount(component: any, props: Record<string, unknown>, prepare?: (pinia: ReturnType<typeof createPinia>) => void) {
  const root: Node = { type: 'root', children: [], props: {} }
  const app = renderer.createApp({ setup: () => () => h(component, { token: 'isolated', customerId: 901, ...props }) })
  const pinia = createPinia()
  app.use(pinia); prepare?.(pinia); app.provide(ssrContextKey, { modules: new Set() }); app.mount(root)
  return { app, root }
}
function text(node: Node): string { return (node.type === '#comment' ? '' : node.text || '') + node.children.map(text).join('') }
function find(node: Node, predicate: (node: Node) => boolean): Node[] { return [...(predicate(node) ? [node] : []), ...node.children.flatMap(child => find(child, predicate))] }
async function settle() { for (let i = 0; i < 8; i++) await Promise.resolve(); await nextTick() }

beforeEach(() => {
  vi.clearAllMocks()
  api.requests.mockResolvedValue({rows:[],total:0,total_pages:1})
  vi.stubGlobal('Document', class Document {})
  vi.stubGlobal('ShadowRoot', class ShadowRoot {})
  vi.stubGlobal('uni', { $on: vi.fn(), $off: vi.fn(), navigateTo: vi.fn(), navigateBack: vi.fn(), showToast: vi.fn() })
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

const purchaseFamily = {
  parent_product_id: 7, name: '测试成品', default_bom_spec_id: 20,
  specs: [{ product_id: 7, sku_id: 7, bom_spec_id: 20, bom_variant_id: 559, spec_name: '454g袋', spec_label: '454g袋', inventory_unit: '袋', sales_unit: '袋', migration_state: 'cutover', is_default: true }],
}
const purchaseSettings = { customer_id: 901, revision: 1, modes: [{ mode: 'stock', enabled: true, warehouse_code: 'CUSTOMER-901-FINISHED' }] }

function purchaseDraft(pinia: ReturnType<typeof createPinia>, mode: 'stock_purchase' | 'product_order' | 'direct_ship', delivery: 'customer_stock' | 'ship') {
  useCustomerOrderDraftStore(pinia).saveDraft(901, mode, {
    order_date: '2026-10-22', recipient: null, note: '测试购货留厂',
    lines: [selectDirectShipDraftProduct(createDirectShipDraftLine('purchase-line'), purchaseFamily)!],
    idempotency_key: 'runtime-retained-purchase', delivery_mode: delivery,
    delivery_warehouse_code: delivery === 'customer_stock' ? 'CUSTOMER-901-FINISHED' : '',
    return_to_direct_ship: mode === 'stock_purchase',
  })
}

it('real stock-purchase entry previews and submits retained goods without a recipient address', async () => {
  api.productCatalog.mockResolvedValue({ current_customer_id: 901, product_families: [purchaseFamily], price_tables: [{ id: 71, table_name: '测试价格表' }], fulfillment_settings: purchaseSettings })
  api.previewProductOrder.mockResolvedValue({ can_submit: true, total_amount: 20, price_quote_token: 'retained-price-token', items: [] })
  api.createProductOrder.mockResolvedValue({ id: 90101 })
  const { app, root } = mount(DirectShip, { orderMode: 'product_order', stockPurchase: true })
  try {
    await settle()
    expect(text(root)).toContain('提交购货订单')
    expect(text(root)).not.toContain('搜索并选择收件客户')
    const productSelector = find(root, node => node.type === 'view' && String(node.props.class || '').includes('selector-field') && text(node).includes('搜索并选择商品'))[0]
    await productSelector.props.onTap()
    await settle()
    const product = find(root, node => node.type === 'view' && String(node.props.class || '').includes('product-row'))[0]
    await product.props.onTap()
    await settle()
    expect(text(root)).toContain('454g袋')
    const submit = find(root, node => node.type === 'button' && String(node.props.class || '').includes('submit'))[0]
    expect(submit.props.disabled).toBe(false)
    await submit.props.onTap()
    await settle()
    expect(api.createProductOrder).toHaveBeenCalledTimes(1)
    expect(api.previewProductOrder).toHaveBeenCalled()
    const command = api.createProductOrder.mock.calls[0][1]
    expect(command).toMatchObject({ delivery_mode: 'customer_stock', delivery_warehouse_code: 'CUSTOMER-901-FINISHED', recipient_name: '', recipient_phone: '', detail_address: '', price_quote_token: 'retained-price-token', items: [{ product_id: 7, bom_spec_id: 20, bom_variant_id: 559, spec_g: 0, qty: 1 }] })
    expect(command.fulfillment_mode).toBeUndefined()
    expect(api.previewDirectShip).not.toHaveBeenCalled()
    expect(uni.navigateBack).toHaveBeenCalledOnce()
  } finally { app.unmount() }
})

it('real ordinary shipping still refuses preview and submit when the recipient is missing', async () => {
  api.productCatalog.mockResolvedValue({ current_customer_id: 901, product_families: [purchaseFamily], price_tables: [{ id: 71, table_name: '测试价格表' }], fulfillment_settings: { customer_id: 901, revision: 1, modes: [] } })
  const { app, root } = mount(DirectShip, { orderMode: 'product_order' }, pinia => purchaseDraft(pinia, 'product_order', 'ship'))
  try {
    await settle()
    const submit = find(root, node => node.type === 'button' && String(node.props.class || '').includes('submit'))[0]
    await submit.props.onTap()
    await settle()
    expect(text(root)).toContain('请选择完整的收件客户信息')
    expect(api.previewProductOrder).not.toHaveBeenCalled()
    expect(api.createProductOrder).not.toHaveBeenCalled()
  } finally { app.unmount() }
})

it('real three-line retained purchase keeps both BOM specs of the same product and submits the latest quote', async () => {
  const obsidian = {
    parent_product_id: 8001, name: '曜石2.0', migration_state: 'cutover', default_bom_spec_id: 120,
    specs: [
      { product_id: 8001, sku_id: 8001, bom_spec_id: 120, bom_variant_id: 1120, spec_g: 0, spec_name: '454g袋装', spec_label: '454g袋装', inventory_unit: '袋', sales_unit: '袋', migration_state: 'cutover' },
      { product_id: 8001, sku_id: 8001, bom_spec_id: 119, bom_variant_id: 1119, spec_g: 0, spec_name: '227g袋装', spec_label: '227g袋装', inventory_unit: '袋', sales_unit: '袋', migration_state: 'cutover' },
    ],
  }
  const dawn = {
    parent_product_id: 7, name: '初晓', migration_state: 'cutover', default_bom_spec_id: 19,
    specs: [{ product_id: 7, sku_id: 7, bom_spec_id: 19, bom_variant_id: 558, spec_g: 0, spec_name: '227g袋装', spec_label: '227g袋装', inventory_unit: '袋', sales_unit: '袋', migration_state: 'cutover' }],
  }
  const expectedItems = [
    { product_id: 8001, bom_spec_id: 120, bom_variant_id: 1120, spec_g: 0, inventory_unit: '袋', sales_unit: '袋', qty: 1 },
    { product_id: 7, bom_spec_id: 19, bom_variant_id: 558, spec_g: 0, inventory_unit: '袋', sales_unit: '袋', qty: 4 },
    { product_id: 8001, bom_spec_id: 119, bom_variant_id: 1119, spec_g: 0, inventory_unit: '袋', sales_unit: '袋', qty: 4 },
  ]
  api.productCatalog.mockResolvedValue({ current_customer_id: 901, product_families: [obsidian, dawn], price_tables: [{ id: 71, table_name: '测试价格表' }], fulfillment_settings: purchaseSettings })
  let quoteNumber = 0
  api.previewProductOrder.mockImplementation(async (_token, command) => ({
    can_submit: true, total_amount: 90, price_quote_token: `three-line-quote-${++quoteNumber}`,
    items: command.items.map((item: Record<string, unknown>) => ({ ...item, unit_price: 10 })),
  }))
  api.createProductOrder.mockResolvedValue({ id: 90102 })
  vi.useFakeTimers()
  const { app, root } = mount(DirectShip, { orderMode: 'product_order', stockPurchase: true })
  const cards = () => find(root, node => node.type === 'view' && String(node.props.class || '').includes('line-card'))
  async function chooseLine(index: number, name: string, spec: string, quantity: number) {
    const selector = find(cards()[index], node => node.type === 'view' && String(node.props.class || '').includes('selector-field'))[0]
    await selector.props.onTap()
    await settle()
    const product = find(root, node => node.type === 'view' && String(node.props.class || '').includes('product-row') && find(node, child => child.type === 'text' && child.props.class === 'option-name').some(child => text(child) === name))[0]
    await product.props.onTap()
    await settle()
    const specification = find(cards()[index], node => node.type === 'button' && String(node.props.class || '').includes('spec-choice') && text(node).includes(spec))[0]
    await specification.props.onTap()
    await settle()
    const input = find(cards()[index], node => node.type === 'input' && node.props.type === 'number')[0]
    input.props['onUpdate:modelValue'](quantity)
    input.props.onInput()
    await settle()
  }
  async function addLine() {
    await find(root, node => node.type === 'button' && node.props.class === 'add-line')[0].props.onTap()
    await settle()
  }
  try {
    await settle()
    await chooseLine(0, '曜石2.0', '454g袋装', 1)
    await addLine()
    await chooseLine(1, '初晓', '227g袋装', 4)
    await addLine()
    await chooseLine(2, '曜石2.0', '227g袋装', 4)
    await vi.advanceTimersByTimeAsync(350)
    await settle()
    expect(cards()).toHaveLength(3)
    expect(api.previewProductOrder).toHaveBeenCalledTimes(1)
    expect(api.previewProductOrder.mock.calls[0][1].items).toEqual(expectedItems)

    const submit = find(root, node => node.type === 'button' && String(node.props.class || '').includes('submit'))[0]
    expect(submit.props.disabled).toBe(false)
    await submit.props.onTap()
    await settle()
    expect(api.previewProductOrder).toHaveBeenCalledTimes(2)
    expect(api.createProductOrder).toHaveBeenCalledTimes(1)
    const latestPreview = api.previewProductOrder.mock.calls[1][1]
    const command = api.createProductOrder.mock.calls[0][1]
    expect(latestPreview.items).toEqual(expectedItems)
    expect(command.items).toEqual(latestPreview.items)
    expect(command).toMatchObject({ delivery_mode: 'customer_stock', delivery_warehouse_code: 'CUSTOMER-901-FINISHED', recipient_name: '', recipient_phone: '', detail_address: '', price_quote_token: 'three-line-quote-2', idempotency_key: latestPreview.idempotency_key })
    expect(command.price_quote_token).not.toBe('three-line-quote-1')
    expect(command.fulfillment_mode).toBeUndefined()
    expect(api.previewDirectShip).not.toHaveBeenCalled()
    expect(uni.navigateBack).toHaveBeenCalledOnce()
  } finally { app.unmount(); vi.useRealTimers() }
})

it('real customer shipment records render shared ready-to-ship package status',async()=>{
 api.directCatalog.mockResolvedValue({current_customer_id:901,product_families:[],price_tables:[],fulfillment_settings:purchaseSettings})
 api.requests.mockResolvedValue({rows:[{id:8,request_no:'DSR-TEST',order_no:'SO-TEST',status:'reserved',fulfillment_mode:'stock',items:[],packages:[{id:1,order_id:116,order_no:'SO-TEST',status:'reserved',process_status:'库存待发货',ship_status:'待发货',fulfillment_progress:{label:'库存充足 · 可以发货',can_ship:true}}]}],total:1,total_pages:1})
 const {app,root}=mount(DirectShip,{orderMode:'direct_ship',token:'test',showCreate:false})
 try {await settle();expect(text(root)).toContain('库存充足 · 可以发货');expect(text(root)).not.toContain('生产：库存待发货')} finally{app.unmount()}
})
