import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { createSSRApp, effectScope, ssrContextKey, type Ref } from 'vue'
import { createPinia } from 'pinia'
import Form from '../pages/employee-order-entry/employee-order-entry.vue'
import { useSessionStore } from '../stores/session'
import { acceptRecipientConsent, revokeRecipientConsent, consentScope } from './recipientConsent'
import type { EmployeeOrderDraftPayload, EmployeeOrderForm } from '../api/customerPortal'

const api = vi.hoisted(() => ({
  take: vi.fn(), base: vi.fn(), draft: vi.fn(), catalog: vi.fn(), warehouses: vi.fn(),
  detail: vi.fn(), customers: vi.fn(),
}))
const hooks = vi.hoisted(() => ({ load: (_query: Record<string, string>) => {} }))
vi.mock('./homeOrderHandoff', async original => ({
  ...await original<typeof import('./homeOrderHandoff')>(), takeHomeOrderEntry: api.take,
}))
vi.mock('../api/employeeEntry', () => ({
  fetchEntryBase: api.base, fetchEntryCatalog: api.catalog, fetchEntryWarehouses: api.warehouses,
}))
vi.mock('../api/customerPortal', async original => ({
  ...await original<typeof import('../api/customerPortal')>(),
  fetchEmployeeOrderDraft: api.draft, fetchEmployeeOrderDetail: api.detail, fetchEmployeeCustomers: api.customers,
}))
vi.mock('@dcloudio/uni-app', () => ({
  onLoad: (callback: typeof hooks.load) => { hooks.load = callback },
  onShow: vi.fn(), onHide: vi.fn(), onUnload: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn(),
}))
const base = { revision: 'base-1', customers: [{ id: 42, name: '测试客户', receiver_name: '测试收件人', default_source_id: 7, default_order_type_id: 8 }] }
const catalog = { revision: 'catalog-1', selected_price_table_ids: [13], product_families: [] }
const warehouses = { revision: 'warehouse-1', delivery_warehouses: [{ code: 'C', name: '客户仓' }] }
let identity = 10
const scopes: ReturnType<typeof effectScope>[] = []
function setup(consented = true, permissions: string[] = []) {
  const app = createSSRApp({})
  const pinia = createPinia()
  app.use(pinia)
  app.provide(ssrContextKey, { modules: new Set() })
  const session = useSessionStore(pinia)
  session.$patch({ token: 'employee', employeeID: ++identity, accountType: 'employee', permissions })
  if (consented) acceptRecipientConsent(session)
  const scope = effectScope()
  scopes.push(scope)
  const component = Form as unknown as { setup: (props: object, ctx: object) => {
    form: Ref<EmployeeOrderDraftPayload>, formData: Ref<EmployeeOrderForm | undefined>,
    customerEditorOpen: Ref<boolean>, loadError: Ref<string>, productCatalogLoading: Ref<boolean>,
    refreshCurrentProductCatalog(): Promise<boolean>,
  } }
  const page = app.runWithContext(() => scope.run(() => component.setup({}, { expose: vi.fn() })))!
  return { page, session }
}
async function settle() {
  for (let step = 0; step < 12; step++) await Promise.resolve()
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
  vi.stubGlobal('uni', { getStorageSync: vi.fn(), setStorageSync: vi.fn(), removeStorageSync: vi.fn(), showToast: vi.fn(), setNavigationBarTitle: vi.fn() })
  api.base.mockResolvedValue(base)
  api.catalog.mockResolvedValue(catalog)
  api.warehouses.mockResolvedValue(warehouses)
  api.draft.mockResolvedValue({ draft: null })
  api.customers.mockResolvedValue({ customers: [] })
})
afterEach(() => {
  scopes.splice(0).forEach(scope => scope.stop())
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
})

it('applies the selected home customer, defaults and preloaded data without duplicate requests', async () => {
  const { page, session } = setup()
  api.take.mockReturnValueOnce({ base, intent: { kind: 'customer', customerId: 42 },
    selection: { catalog: Promise.resolve(catalog), warehouses: Promise.resolve(warehouses) } })
  hooks.load({ home_entry: 'entry' })
  await settle()
  expect(api.take).toHaveBeenCalledWith('entry', 'employee', consentScope(session))
  expect(page.form.value).toMatchObject({ customer_id: 42, receiver_name: '测试收件人', source_id: 7, order_type_id: 8, selected_price_table_ids: [13] })
  expect(page.formData.value?.delivery_warehouses).toEqual(warehouses.delivery_warehouses)
  for (const read of [api.base, api.draft, api.catalog, api.warehouses]) expect(read).not.toHaveBeenCalled()
})

it('restores the selected draft and re-reads the server draft on a later direct visit', async () => {
  const { page } = setup()
  const draft = { id: 11, payload: { customer_id: 42, notes: '手工草稿', selected_price_table_ids: [3], items: [] } }
  api.take.mockReturnValueOnce({ base, intent: { kind: 'draft', draft } })
  hooks.load({ home_entry: 'entry' })
  await settle()
  expect(page.form.value.notes).toBe('手工草稿')
  expect(api.catalog).toHaveBeenCalledWith('employee', 42, [3])
  expect(api.base).not.toHaveBeenCalled()
  expect(api.draft).not.toHaveBeenCalled()
  const later = setup()
  api.draft.mockResolvedValueOnce({ draft: { ...draft, payload: { ...draft.payload, notes: '已修改草稿' } } })
  hooks.load({ home_entry: 'used-entry' })
  await settle()
  expect(later.page.form.value.notes).toBe('已修改草稿')
  expect(api.draft).toHaveBeenCalledTimes(1)
})

it.each([[[], false], [['customers.read', 'customers.write'], true]] as const)('checks current permissions before opening the create-customer editor', async (p, allowed) => {
  const { page } = setup(true, [...p])
  api.take.mockReturnValueOnce({ base, intent: { kind: 'create' } })
  hooks.load({ home_entry: 'entry' })
  await settle()
  expect(page.customerEditorOpen.value).toBe(allowed)
  expect(api.customers).toHaveBeenCalledTimes(allowed ? 1 : 0)
  expect(api.base).not.toHaveBeenCalled()
  expect(api.draft).not.toHaveBeenCalled()
})

it('waits for the route before loading when permission is granted during startup, and discards revoked data', async () => {
  const { page, session } = setup(false)
  acceptRecipientConsent(session)
  let complete!: (rows: typeof catalog) => void
  api.take.mockReturnValueOnce({ base, intent: { kind: 'customer', customerId: 42 },
    selection: { catalog: new Promise(resolve => { complete = resolve }), warehouses: Promise.resolve(warehouses) } })
  hooks.load({ home_entry: 'entry' })
  await settle()
  expect(api.base).not.toHaveBeenCalled()
  expect(page.form.value.customer_id).toBe(42)
  revokeRecipientConsent(session)
  complete(catalog)
  await settle()
  expect(page.formData.value).toBeUndefined()
  expect(page.form.value.customer_id).toBe(0)
})

it('falls back to a fresh base and draft when the chosen customer is missing', async () => {
  const { page } = setup()
  api.take.mockReturnValueOnce({ base, intent: { kind: 'customer', customerId: 77 } })
  hooks.load({ home_entry: 'entry' })
  await settle()
  expect(api.base).toHaveBeenCalledTimes(1)
  expect(api.draft).toHaveBeenCalledTimes(1)
  expect(api.catalog).not.toHaveBeenCalled()
  expect(page.form.value.customer_id).toBe(0)
})

it('shows catalog preload failures and retries from the normal form without replaying the handoff', async () => {
  const { page } = setup()
  api.take.mockReturnValueOnce({ base, intent: { kind: 'customer', customerId: 42 },
    selection: { catalog: Promise.reject(new Error('网络超时')), warehouses: Promise.resolve(warehouses) } })
  hooks.load({ home_entry: 'entry' })
  await settle()
  expect(uni.showToast).toHaveBeenCalledWith({ title: '网络超时', icon: 'none' })
  expect(page.productCatalogLoading.value).toBe(false)
  expect(api.catalog).not.toHaveBeenCalled()
  expect(await page.refreshCurrentProductCatalog()).toBe(true)
  expect(api.catalog).toHaveBeenCalledTimes(1)
  expect(page.form.value.selected_price_table_ids).toEqual([13])
})

it.each(['edit_id', 'copy_id'])('ignores home_entry on an explicit %s route', async key => {
  const { page } = setup()
  api.detail.mockRejectedValueOnce(new Error('指定订单不存在'))
  hooks.load({ home_entry: 'entry', [key]: '25' })
  await settle()
  expect(api.take).not.toHaveBeenCalled()
  expect(api.detail).toHaveBeenCalledWith('employee', 25)
  expect(page.loadError.value).toBe('指定订单不存在')
})
