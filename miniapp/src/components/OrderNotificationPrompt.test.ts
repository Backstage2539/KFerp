import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRenderer, nextTick, reactive, ssrContextKey } from 'vue'

const hooks = vi.hoisted(() => ({ request: vi.fn(), session: null as any, page: {} as any }))
vi.mock('../api/client', () => ({ miniRequest: hooks.request, MiniRequestError: class extends Error { statusCode = 409 } }))
vi.mock('../stores/session', () => ({ useSessionStore: () => hooks.session }))
import OrderNotificationPrompt from './OrderNotificationPrompt.vue'
import { notificationPageMixin } from '../utils/notificationPrompt'
import { MiniRequestError } from '../api/client'

type Node = { type: string; text: string; props: Record<string, any>; children: Node[]; parent?: Node }
function node(type: string, text = ''): Node { return { type, text, props: {}, children: [] } }
const renderer = createRenderer<Node, Node>({
  createElement: type => node(type), createText: text => node('#text', text), createComment: text => node('#comment', text),
  setText: (el, text) => { el.text = text }, setElementText: (el, text) => { el.text = text; el.children = [] },
  parentNode: el => el.parent || null, nextSibling: () => null,
  insert: (el, parent) => { el.parent = parent; parent.children.push(el) }, remove: el => { if (el.parent) el.parent.children = el.parent.children.filter(child => child !== el) },
  patchProp: (el, key, _previous, next) => { el.props[key] = next },
})
async function flush() { await Promise.resolve(); await nextTick(); await Promise.resolve(); await nextTick() }
function response(data: Record<string, unknown> = {}) { return { enabled: true, prompt_required: true, prompt_version: 11, choice: null, rows: [], mini_user_id: 7, customer_id: 2, employee_id: 0, ...data } }
function mount() {
  const app = renderer.createApp(OrderNotificationPrompt)
  app.config.warnHandler = () => {}
  app.provide(ssrContextKey, { modules: new Set() })
  const instance = app.mount(node('root'))
  return { app, state: (instance.$ as unknown as { setupState: Record<string, any> }).setupState }
}
function hideAndUnmount(app: ReturnType<typeof renderer.createApp>) { notificationPageMixin.onHide(); app.unmount() }

describe('notification prompt component', () => {
  beforeEach(() => {
    hooks.request.mockReset()
    hooks.session = reactive({ token: 'token', miniUserID: 7, currentCustomerID: 2, currentCustomerName: '测试客户', employeeID: 0, employeeName: '', accountType: 'customer', isCustomerPreview: false })
    hooks.page = { route: 'pages/home/home' }
    vi.stubGlobal('getCurrentPages', () => [hooks.page])
    vi.stubGlobal('uni', { showModal: vi.fn(), navigateTo: vi.fn() })
  })
  it('loads on page show and presents no preselected notification for a new account', async () => {
    hooks.request.mockResolvedValue(response())
    const { app, state } = mount()
    await flush()
    expect(hooks.request).toHaveBeenCalledWith('/api/mini/order-notifications', { token: 'token' })
    expect(state.visible).toBe(true)
    expect(state.events).toEqual([])
    expect(state.contextLabel).toBe('当前客户：测试客户')
    state.postpone()
    expect(state.visible).toBe(false)
    notificationPageMixin.onHide(); notificationPageMixin.onShow(); await flush()
    expect(state.visible).toBe(false)
    expect(hooks.request).toHaveBeenCalledTimes(2)
    hideAndUnmount(app)
  })
  it('saves selected events explicitly without a binding, then offers the binding step', async () => {
    hooks.request.mockResolvedValueOnce(response({ prompt_version: 12 })).mockResolvedValueOnce({})
    const { app, state } = mount(); await flush()
    state.toggle('order_accepted'); state.toggle('receipt_received')
    await state.save(true); await flush()
    expect(hooks.request).toHaveBeenLastCalledWith('/api/mini/order-notifications/prompt', { token: 'token', method: 'PUT', data: { version: 12, mini_user_id: 7, customer_id: 2, employee_id: 0, enabled: true, events: ['order_accepted','receipt_received'] } })
    expect(state.visible).toBe(false)
    expect(uni.showModal).toHaveBeenCalledOnce()
    hideAndUnmount(app)
  })
  it('persists explicit refusal without turning notifications on', async () => {
    hooks.request.mockResolvedValueOnce(response({ prompt_version: 13 })).mockResolvedValueOnce({})
    const { app, state } = mount(); await flush()
    await state.save(false); await flush()
    expect(hooks.request).toHaveBeenLastCalledWith('/api/mini/order-notifications/prompt', { token: 'token', method: 'PUT', data: { version: 13, mini_user_id: 7, customer_id: 2, employee_id: 0, enabled: false, events: [] } })
    expect(uni.showModal).not.toHaveBeenCalled()
    hideAndUnmount(app)
  })
  it('shows existing choices on a new prompt without saving or enabling them', async () => {
    hooks.request.mockResolvedValueOnce(response({ prompt_version: 14, rows: [{ customer_id: 2, employee_id: 0, enabled: false, subscribed: true, events: ['shipped'] }] }))
    const { app, state } = mount(); await flush()
    expect(state.events).toEqual(['shipped'])
    expect(state.previousHint).toContain('已关闭')
    expect(hooks.request).toHaveBeenCalledTimes(1)
    hideAndUnmount(app)
  })
  it('requires a new binding when a formerly subscribed account was unbound', async () => {
    hooks.request.mockResolvedValueOnce(response({ prompt_version: 16, rows: [{ customer_id: 2, employee_id: 0, active: false, enabled: false, subscribed: true, events: ['shipped'] }] })).mockResolvedValueOnce({})
    const { app, state } = mount(); await flush()
    await state.save(true); await flush()
    expect(uni.showModal).toHaveBeenCalledOnce()
    hideAndUnmount(app)
  })
  it('does not interrupt ordinary app use when loading fails', async () => {
    hooks.request.mockRejectedValue(new Error('network error'))
    const { app, state } = mount(); await flush()
    expect(state.visible).toBe(false)
    expect(uni.showModal).not.toHaveBeenCalled()
    hideAndUnmount(app)
  })
  it('drops an old response when the account changes and never prompts for preview mode', async () => {
    let resolveOld!: (value: unknown) => void
    hooks.request.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockResolvedValue(response({ customer_id: 3, prompt_required: false, prompt_version: 15 }))
    const { app, state } = mount(); await flush()
    hooks.session.currentCustomerID = 3; await flush()
    resolveOld(response({ prompt_version: 15 })); await flush()
    expect(state.visible).toBe(false)
    hooks.session.isCustomerPreview = true; await flush()
    const count = hooks.request.mock.calls.length
    notificationPageMixin.onShow(); await flush()
    expect(hooks.request).toHaveBeenCalledTimes(count)
    hideAndUnmount(app)
  })
  it('only shows the current page across push, back and app foreground events', async () => {
    hooks.request.mockResolvedValue(response({ prompt_version: 17 }))
    const firstPage = hooks.page
    const first = mount(); await flush()
    expect(first.state.visible).toBe(true)
    notificationPageMixin.onHide()
    hooks.page = { route: 'pages/notification-detail/notification-detail' }
    const second = mount(); await flush()
    expect(first.state.visible).toBe(false)
    expect(second.state.visible).toBe(true)
    notificationPageMixin.onHide(); second.app.unmount(); hooks.page = firstPage
    notificationPageMixin.onShow(); await flush()
    expect(first.state.visible).toBe(true)
    notificationPageMixin.onHide()
    expect(first.state.visible).toBe(false)
    notificationPageMixin.onShow(); await flush()
    expect(first.state.visible).toBe(true)
    hideAndUnmount(first.app)
  })
  it('does not show a response belonging to a different server-side customer', async () => {
    hooks.request.mockResolvedValue(response({ customer_id: 99, prompt_version: 18 }))
    const { app, state } = mount(); await flush()
    expect(state.visible).toBe(false)
    hideAndUnmount(app)
  })
  it('reloads the choice after a version or identity conflict instead of saving stale selections', async () => {
    hooks.request.mockResolvedValueOnce(response({ prompt_version: 19 }))
      .mockRejectedValueOnce(new MiniRequestError('版本已变化', 409))
      .mockResolvedValueOnce(response({ prompt_version: 20, choice: { enabled: false, events: [] } }))
    const { app, state } = mount(); await flush()
    state.toggle('order_accepted')
    await state.save(true); await flush()
    expect(state.visible).toBe(true)
    expect(state.events).toEqual([])
    expect(state.error).toContain('重新选择')
    hideAndUnmount(app)
  })
})
