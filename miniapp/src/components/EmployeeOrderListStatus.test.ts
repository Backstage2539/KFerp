import { readFileSync } from 'node:fs'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import * as Vue from 'vue'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import Editor from './EmployeeOrderListStatus.vue'
import type { EmployeeOrder } from '../api/customerPortal'

const api = vi.hoisted(() => ({ detail: vi.fn(), update: vi.fn(), choose: vi.fn() }))
vi.mock('../api/customerPortal', () => ({ fetchEmployeeOrderDetail: api.detail, updateEmployeeOrderStatus: api.update }))
vi.mock('../utils/employeePaymentVoucher', async original => ({
  ...await original<typeof import('../utils/employeePaymentVoucher')>(), chooseEmployeePaymentVoucher: api.choose,
}))
vi.mock('../stores/session', () => ({ useSessionStore: () => ({ token: 'employee' }) }))

// Render UniApp's real template with Vue so these checks exercise its tap and picker handlers.
const { descriptor } = parse(readFileSync(new URL('./EmployeeOrderListStatus.vue', import.meta.url), 'utf8'))
const id = 'employee-order-list-status-test'
const script = compileScript(descriptor, { id })
const compiled = compileTemplate({ filename: 'EmployeeOrderListStatus.vue', id, source: descriptor.template!.content,
  compilerOptions: { bindingMetadata: script.bindings, isCustomElement: tag => ['view', 'picker', 'text'].includes(tag) },
})
const renderCode = compiled.code.replace(/import \{([^}]+)\} from "vue"/g, (_line, bindings: string) => 'const {' + bindings.replace(/ as /g, ':') + '} = Vue')
  .replace('export function render', 'function render')
Object.assign(Editor, { render: new Function('Vue', renderCode + ';return render')(Vue) })

type Element = {
  type: string; children: Element[]; parent?: Element; text: string; props: Record<string, any>;
  addEventListener: () => void; getRootNode: () => { activeElement: null }
}
const element = (type: string, text = ''): Element => ({ type, text, children: [], props: {}, addEventListener() {}, getRootNode: () => ({ activeElement: null }) })
const renderer = Vue.createRenderer<Element, Element>({
  createElement: type => element(type), createText: text => element('#text', text), createComment: text => element('#comment', text),
  setText: (el, text) => { el.text = text },
  setElementText: (el, text) => { el.text = text; el.children = [] },
  patchProp: (el, key, _old, value) => { el.props[key] = value },
  parentNode: el => el.parent || null,
  nextSibling: el => el.parent?.children[el.parent.children.indexOf(el) + 1] || null,
  insert(el, parent, anchor) {
    if (el.parent) el.parent.children = el.parent.children.filter(child => child !== el)
    el.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    if (index < 0) parent.children.push(el)
    else parent.children.splice(index, 0, el)
  },
  remove: el => { if (el.parent) el.parent.children = el.parent.children.filter(child => child !== el) },
})
function nodes(root: Element): Element[] { return [root, ...root.children.flatMap(nodes)] }
function text(root: Element): string { return nodes(root).map(el => el.text).join('') }
const pay = ['未付款', '预付款（付款未完成）', '已付款', '已收款'].map((name, i) => ({ name, id: i + 1 }))
const ship = [{ id: 1, name: '未发货' }, { id: 2, name: '已发货' }]
const cleanup: Array<() => void> = []
beforeEach(() => {
  vi.resetAllMocks()
  vi.stubGlobal('uni', { showToast: vi.fn() })
  vi.stubGlobal('Document', class {})
  vi.stubGlobal('ShadowRoot', class {})
  api.update.mockResolvedValue({})
  api.choose.mockResolvedValue({ id: 87, filename: '付款截图.png' })
})
afterEach(() => { cleanup.splice(0).forEach(fn => fn()); vi.unstubAllGlobals() })
function mount(overrides: Record<string, unknown> = {}) {
  const order = { id: 42, pay_status_id: 1, pay_status: '未付款', ship_status_id: 1, ship_status: '未发货',
    grand_total: '100.00', prepayment_amount: '0.00', edit_revision: 'r1', payment_method: '现金', confirmation_required: false, ...overrides }
  api.detail.mockResolvedValue({ order, can_update_status: true, can_update_ship_status: !order.confirmation_required,
    status_options: { pay_statuses: pay, ship_statuses: ship } })
  const root = element('root')
  const changed = vi.fn()
  const app = renderer.createApp(Editor, { order: order as unknown as EmployeeOrder, onChanged: changed })
  app.provide(Vue.ssrContextKey, { modules: new Set() })
  app.mount(root)
  cleanup.push(() => app.unmount())
  const find = (predicate: (el: Element) => boolean) => {
    const el = nodes(root).find(predicate)
    if (!el) throw new Error('Missing control in ' + text(root))
    return el
  }
  const tap = async (el: Element) => { await el.props.onTap?.({ stopPropagation() {} }); await Vue.nextTick() }
  const status = (prefix: string) => find(el => el.props.class === 'status' && text(el).includes(prefix))
  const button = (label: string) => find(el => el.type === 'button' && text(el).includes(label))
  const selectStatus = async (index: number) => {
    find(el => el.type === 'picker').props.onChange({ detail: { value: index } })
    await Vue.nextTick()
  }
  const input = async (index: number, value: string) => {
    nodes(root).filter(el => el.type === 'input')[index].props['onUpdate:modelValue'](value)
    await Vue.nextTick()
  }
  return { root, changed, find, tap, status, button, selectStatus, input }
}

it('saves a changed deposit without switching the payment status', async () => {
  const ui = mount({ pay_status_id: 2, pay_status: pay[1].name, prepayment_amount: '20.00' })
  await ui.tap(ui.status('付款'))
  await ui.input(0, '35.50')
  await ui.tap(ui.button('保存'))
  expect(api.update).toHaveBeenCalledWith('employee', 42, expect.objectContaining({ pay_status_id: 2, prepayment_amount: 35.5, edit_revision: 'r1' }))
  expect(ui.changed).toHaveBeenCalledWith(42)
})
it('attaches a missing voucher to an already-paid order', async () => {
  const ui = mount({ pay_status_id: 3, pay_status: '已付款' })
  await ui.tap(ui.status('付款'))
  await ui.tap(ui.button('相册'))
  await ui.tap(ui.button('保存'))
  expect(api.update).toHaveBeenCalledWith('employee', 42, expect.objectContaining({ pay_status_id: 3, payment_voucher_asset_id: 87 }))
})
it('discards a voucher or deposit drafted for an abandoned target', async () => {
  const ui = mount()
  await ui.tap(ui.status('付款'))
  await ui.selectStatus(2)
  await ui.tap(ui.button('相册'))
  await ui.selectStatus(1)
  await ui.input(0, '25.00')
  await ui.selectStatus(2)
  await ui.tap(ui.button('保存'))
  expect(api.update).not.toHaveBeenCalled()
  expect(text(ui.root)).toContain('请上传')
  await ui.tap(ui.button('相册'))
  await ui.tap(ui.button('保存'))
  expect(api.update).toHaveBeenCalledWith('employee', 42, expect.objectContaining({ payment_voucher_asset_id: 87, prepayment_amount: 0 }))
})
it.each([{ target: 1, index: 0, value: '.' }, { target: 1, index: 0, value: '100.01' },
  { target: 3, index: 0, value: '.' }, { target: 3, index: 1, value: '3.001' }])(
  'keeps invalid money in the editor for correction ($target/$index/$value)', async ({ target, index, value }) => {
    const ui = mount({ payment_goods_amount: '90.00', payment_shipping_amount: '10.00' })
    await ui.tap(ui.status('付款'))
    await ui.selectStatus(target)
    if (target === 3) await ui.tap(ui.button('相册'))
    await ui.input(index, value)
    await ui.tap(ui.button('保存'))
    expect(api.update).not.toHaveBeenCalled()
    expect(ui.find(el => el.props.class === 'error').text).toBeTruthy()
  })
it('keeps the editor open while saving and shows a late failure after an overlay tap', async () => {
  const ui = mount()
  let reject!: (error: Error) => void
  api.update.mockImplementation(() => new Promise((_, fail) => { reject = fail }))
  await ui.tap(ui.status('发货'))
  await ui.selectStatus(1)
  const saving = ui.tap(ui.button('保存'))
  await Vue.nextTick()
  await ui.tap(ui.find(el => el.props.class === 'overlay'))
  expect(text(ui.root)).toContain('修改发货状态')
  await ui.tap(ui.button('保存'))
  expect(api.update).toHaveBeenCalledTimes(1)
  expect(api.update.mock.calls[0][2]).toMatchObject({ payment_unchanged: true, ship_status_id: 2 })
  reject(new Error('订单版本已更新'))
  await saving
  expect(text(ui.root)).toContain('订单版本已更新')
})
it('renders free-sample payment, fulfillment shipping and retained-order shipping as read-only', async () => {
  const sample = mount({ pay_status: '无需收款', confirmation_required: true })
  expect(sample.status('无需收款').props.onTap).toBeUndefined()
  expect(sample.status('发货').props.onTap).toBeUndefined()
  const retained = mount({ delivery_mode: 'customer_stock' })
  expect(retained.status('无需发货').props.onTap).toBeUndefined()
  expect(api.detail).not.toHaveBeenCalled()
})
