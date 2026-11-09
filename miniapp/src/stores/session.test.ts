import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { miniappTokenStorageKey, useSessionStore } from './session'

afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs() })

describe('miniapp session storage boundary', () => {
  it('uses separate token keys for development and production', () => {
    expect(miniappTokenStorageKey('development')).toBe('kferp.mini.development.token')
    expect(miniappTokenStorageKey('production')).toBe('kferp.mini.production.token')
    expect(miniappTokenStorageKey('development')).not.toBe('kferp.mini.token')
  })

  it('keeps modes for partial updates of the same customer and never carries them to another customer', () => {
    vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
    vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
    vi.stubGlobal('uni', { getStorageSync: vi.fn(), setStorageSync: vi.fn(), removeStorageSync: vi.fn() })
    setActivePinia(createPinia())
    const session = useSessionStore()
    const context = { mini_user_id: 1, current_customer_id: 74, bindings: [], capabilities: [] }
    session.applyContext({ ...context, fulfillment_modes: ['stock', 'on_demand'] })
    session.applyContext(context)
    expect(session.fulfillmentModes).toEqual(['stock', 'on_demand'])
    session.applyContext({ ...context, fulfillment_modes: ['stock'] })
    expect(session.fulfillmentModes).toEqual(['stock'])
    session.applyContext({ ...context, fulfillment_modes: [] })
    expect(session.fulfillmentModes).toEqual([])
    session.applyContext({ ...context, fulfillment_modes: ['on_demand'] })
    session.applyContext({ ...context, current_customer_id: 75 })
    expect(session.fulfillmentModes).toEqual([])
    session.applyContext({ ...context, current_customer_id: 76, fulfillment_modes: ['stock'] })
    expect(session.fulfillmentModes).toEqual(['stock'])
  })

  it('keeps an explicit logout across page reloads until a new deliberate login', () => {
    vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
    vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
    const storage = new Map<string, unknown>()
    vi.stubGlobal('uni', { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: unknown) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key) })
    setActivePinia(createPinia())
    const session = useSessionStore()
    session.setToken('registered-user')
    session.registrationKnown = true
    session.logout()
    expect(session.token).toBe('')
    expect(session.autoRestoreBlocked).toBe(true)
    expect(session.registrationKnown).toBe(true)
    setActivePinia(createPinia())
    const reopened = useSessionStore()
    expect(reopened.autoRestoreBlocked).toBe(true)
    reopened.setToken('employee-login')
    expect(reopened.autoRestoreBlocked).toBe(false)
    reopened.clearSession(true)
    expect(reopened.autoRestoreBlocked).toBe(false)
    setActivePinia(createPinia())
    expect(useSessionStore().autoRestoreBlocked).toBe(false)
  })
})
