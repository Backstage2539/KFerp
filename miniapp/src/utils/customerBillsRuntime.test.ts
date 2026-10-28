import { afterEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, h, nextTick, ssrContextKey } from 'vue'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import Bills from '../components/CustomerBillsPanel.vue'

const api = vi.hoisted(() => ({ fetch: vi.fn(), output: vi.fn() }))
vi.mock('../api/customerPortal', async (original) => ({ ...await original<typeof import('../api/customerPortal')>(), fetchCustomerAccount: api.fetch }))
vi.mock('./fileOutput', () => ({ openMiniappFileOutput: api.output }))
type Node = { type: string; children: Node[]; parent?: Node; props: Record<string, any>; text?: string }
const textOf = (node: Node): string => [node.text || '', ...node.children.map(textOf)].join(' ')
const findButton = (node: Node, label: string): Node | undefined => (node.type === 'button' && textOf(node).includes(label)) ? node : node.children.map(child => findButton(child, label)).find(Boolean)
const renderer = createRenderer<Node, Node>({ createElement: type => ({ type, children: [], props: {} }), createText: text => ({ type: '#text', text, children: [], props: {} }), createComment: text => ({ type: '#comment', text, children: [], props: {} }), setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text; node.children = [] }, patchProp: (node, key, _old, value) => { node.props[key] = value }, insert: (node, parent, anchor) => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node); node.parent = parent; const index = anchor ? parent.children.indexOf(anchor) : -1; if (index < 0) parent.children.push(node); else parent.children.splice(index, 0, node) }, remove: node => { if (node?.parent) node.parent.children = node.parent.children.filter(child => child !== node) }, parentNode: node => node?.parent || null, nextSibling: node => { const siblings = node?.parent?.children; return siblings ? siblings[siblings.indexOf(node) + 1] || null : null } })
const source = readFileSync(new URL('../components/CustomerBillsPanel.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source), script = compileScript(descriptor, { id: 'customer-bills-runtime' })
const template = compileTemplate({ id: 'customer-bills-runtime', filename: 'CustomerBillsPanel.vue', source: descriptor.template!.content, compilerOptions: { bindingMetadata: script.bindings, isCustomElement: tag => ['view', 'text', 'picker'].includes(tag) } })
const renderCode = template.code.replace(/import \{([^}]+)\} from "vue"/g, (_all, bindings: string) => `const {${bindings.replace(/ as /g, ':')}}=Vue`).replace('export function render', 'function render')
;(Bills as any).render = new Function('Vue', renderCode + ';return render')(Vue)
const settle = async () => { await Promise.resolve(); await Promise.resolve(); await nextTick() }
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks() })

it('switches from the current month to all unpaid and exports the same scope', async () => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-10-28T00:00:00Z'))
  api.fetch.mockResolvedValue({ customer_name:'测试', date_from:'2026-10-01', date_to:'2026-10-31', as_of:'2026-10-28 08:00:00', summary:{}, fee_summary:{goods_cents:6000,processing_cents:0,direct_ship_service_cents:0,shipping_cents:910,other_cents:0,payable_cents:6910,paid_cents:0,due_cents:6910,refund_cents:0},order_fee_groups:[],total:0,total_pages:1 })
  api.output.mockResolvedValue(undefined)
  const root: Node = { type:'root',children:[],props:{} }, app = renderer.createApp({ render: () => h(Bills, { token:'test' }) }); app.provide(ssrContextKey, { modules:new Set() }); app.mount(root)
  try {
    await settle(); expect(api.fetch.mock.calls[0][1]).toMatchObject({date_from:'2026-10-01',date_to:'2026-10-31',page:1})
    findButton(root,'所有未付')!.props.onTap(); await settle()
    expect(api.fetch.mock.calls.at(-1)![1]).toMatchObject({unpaid_only:true,page:1})
    expect(textOf(root)).toContain('未付合计')
    findButton(root,'运费')!.props.onTap(); await settle(); expect(api.fetch.mock.calls.at(-1)![1]).toMatchObject({unpaid_only:true,charge_type:'shipping',page:1})
    findButton(root,'下载 Excel')!.props.onTap(); await settle(); const path = api.output.mock.calls.at(-1)![0].path
    expect(path).toContain('unpaid_only=true'); expect(path).not.toContain('charge_type'); expect(path).not.toContain('page=')
  } finally { app.unmount() }
})
