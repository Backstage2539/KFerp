import { expect, it, vi } from 'vitest'
import { createSSRApp, effectScope, nextTick, ssrContextKey, type Ref } from 'vue'
import { createPinia } from 'pinia'
import EmployeeCustomerEditor from '../components/EmployeeCustomerEditor.vue'
import AddressBook from '../pages/customer-addresses/customer-addresses.vue'
import { useSessionStore } from '../stores/session'
import { acceptRecipientConsent, revokeRecipientConsent } from './recipientConsent'

const hooks = vi.hoisted(() => ({ show: () => {} }))
const api = vi.hoisted(() => ({ fetchEmployeeCustomer: vi.fn(), fetchRecipientAddresses: vi.fn(), parseEmployeeCustomerRecipient: vi.fn() }))
vi.mock('../api/customerPortal', () => api)
vi.mock('@dcloudio/uni-app', () => ({
  onShow: (callback: () => void) => { hooks.show = callback },
  onLoad: vi.fn(), onHide: vi.fn(), onUnload: vi.fn(), onShareAppMessage: vi.fn(), onShareTimeline: vi.fn(),
}))
vi.mock('./miniappShare', () => ({
  defaultMiniappShare: vi.fn(), defaultMiniappTimelineShare: vi.fn(), refreshMiniappShareMenu: vi.fn(),
}))

it('gates address requests and drops an old customer response after an account switch', async () => {
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
  vi.stubGlobal('uni', { getStorageSync: vi.fn(), setStorageSync: vi.fn(), removeStorageSync: vi.fn() })
  const pinia = createPinia()
  const app = createSSRApp({})
  app.use(pinia)
  app.provide(ssrContextKey, { modules: new Set() })
  const session = useSessionStore(pinia)
  session.$patch({ token: 'first-user', miniUserID: 301, currentCustomerID: 401, accountType: 'customer' })
  let resolveFirst!: (result: { rows: { id: number }[] }) => void
  api.fetchRecipientAddresses.mockReturnValueOnce(new Promise(resolve => { resolveFirst = resolve }))
    .mockResolvedValueOnce({ rows: [{ id: 2 }] })
  api.parseEmployeeCustomerRecipient.mockResolvedValue({})
  const scope = effectScope()
  const component = AddressBook as unknown as {
    setup(props: object, context: { expose: () => void }): {
      pastedRecipient: Ref<string>, rows: Ref<{ id: number }[]>, parseRecipient(): Promise<void>
    }
  }
  try {
    const page = app.runWithContext(() => scope.run(() => component.setup({}, { expose: () => {} })))!
    hooks.show()
    page.pastedRecipient.value = '待解析的收件信息'
    await page.parseRecipient()
    expect(api.fetchRecipientAddresses).not.toHaveBeenCalled()
    expect(api.parseEmployeeCustomerRecipient).not.toHaveBeenCalled()

    acceptRecipientConsent(session)
    await nextTick()
    expect(api.fetchRecipientAddresses).toHaveBeenCalledTimes(1)
    expect(api.fetchRecipientAddresses).toHaveBeenCalledWith('first-user')
    page.pastedRecipient.value = '当前账号的收件信息'
    await page.parseRecipient()
    expect(api.parseEmployeeCustomerRecipient).toHaveBeenCalledWith('first-user', '当前账号的收件信息')

    session.$patch({ token: 'second-user', miniUserID: 302, currentCustomerID: 402 })
    await nextTick()
    expect(page.pastedRecipient.value).toBe('')
    expect(api.fetchRecipientAddresses).toHaveBeenCalledTimes(1)
    acceptRecipientConsent(session)
    await nextTick()
    await Promise.resolve()
    expect(page.rows.value).toEqual([{ id: 2 }])
    resolveFirst({ rows: [{ id: 1 }] })
    await Promise.resolve()
    expect(page.rows.value).toEqual([{ id: 2 }])
  } finally {
    scope.stop()
    vi.unstubAllGlobals()
    vi.unstubAllEnvs()
  }
})

it('requires employee recipient consent before fetching customer details or parsing pasted text, including after withdrawal', async () => {
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
  vi.stubGlobal('uni', { getStorageSync: vi.fn(), setStorageSync: vi.fn(), removeStorageSync: vi.fn(), showToast: vi.fn() })
  const pinia = createPinia()
  const app = createSSRApp({})
  app.use(pinia)
  app.provide(ssrContextKey, { modules: new Set() })
  const session = useSessionStore(pinia)
  session.$patch({ token:'employee', employeeID:77, accountType:'employee' })
  api.fetchEmployeeCustomer.mockResolvedValue({customer:{name:'验收专用客户'}})
  api.parseEmployeeCustomerRecipient.mockReset().mockResolvedValue({recipient_name:'测试收件人'})
  const scope = effectScope()
  const editor = EmployeeCustomerEditor as unknown as {
    setup(props: object, context: object): { recipientPaste: Ref<string>; parseRecipient(): Promise<void> }
  }
  try {
    const form = app.runWithContext(() => scope.run(() => editor.setup(
      {visible:true, token:'employee', customerId:1}, {expose:vi.fn(), emit:vi.fn()},
    )))!
    form.recipientPaste.value = '测试收件人'
    await form.parseRecipient()
    expect(api.fetchEmployeeCustomer).not.toHaveBeenCalled()
    expect(api.parseEmployeeCustomerRecipient).not.toHaveBeenCalled()
    acceptRecipientConsent(session)
    await nextTick()
    expect(api.fetchEmployeeCustomer).toHaveBeenCalledWith('employee', 1)
    form.recipientPaste.value = '测试收件人'
    await form.parseRecipient()
    expect(api.parseEmployeeCustomerRecipient).toHaveBeenCalledTimes(1)
    revokeRecipientConsent(session)
    await nextTick()
    form.recipientPaste.value = '撤回后的文本'
    await form.parseRecipient()
    expect(api.parseEmployeeCustomerRecipient).toHaveBeenCalledTimes(1)
  } finally {
    scope.stop()
    vi.unstubAllGlobals()
    vi.unstubAllEnvs()
  }
})
