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
