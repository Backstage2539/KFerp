import { afterEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { createRenderer, h, nextTick, ssrContextKey } from 'vue'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import Bills from '../components/CustomerBillsPanel.vue'

const api = vi.hoisted(() => ({ fetch: vi.fn(), detail: vi.fn(), output: vi.fn() }))
vi.mock('../api/customerPortal', async (original) => ({ ...await original<typeof import('../api/customerPortal')>(), fetchCustomerIssuedBills: api.fetch, fetchCustomerIssuedBill: api.detail }))
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

it('defaults to all issued bills, filters unpaid and exports a selected bill', async () => {
  api.fetch.mockResolvedValue({rows:[{batch_id:4,settlement_no:'CB-74-4',issued_at:'2026-11-08',period_from:'2026-09-01',period_to:'2026-10-30',total_amount_cents:225600,current_received_cents:0,current_due_amount_cents:225600,adjustment_cents:0}],total:1,page:1,limit:20,summary:{count:1,unpaid_count:1,total_amount_cents:225600,received_cents:0,due_amount_cents:225600,adjustment_cents:0}})
  api.detail.mockResolvedValue({batch_id:4,lines:[{id:1,description:'运费',amount_cents:1100}]})
  api.output.mockResolvedValue(undefined)
  const root: Node = { type:'root',children:[],props:{} }, app = renderer.createApp({ render: () => h(Bills, { token:'test' }) }); app.provide(ssrContextKey, { modules:new Set() }); app.mount(root)
  try {
    await settle(); expect(api.fetch.mock.calls[0][1]).toMatchObject({date_from:'',date_to:'',status:'',page:1})
    findButton(root,'未付')!.props.onTap(); await settle(); expect(api.fetch.mock.calls.at(-1)![1]).toMatchObject({status:'unpaid',page:1})
    expect(textOf(root)).toContain('CB-74-4')
  } finally { app.unmount() }
})
