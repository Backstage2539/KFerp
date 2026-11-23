import { computed } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  acceptRecipientConsent, consentScope, hasRecipientConsent,
  recipientPolicyVersion, revokeRecipientConsent,
} from './recipientConsent'

let nextUserID = 0
const session = () => ({
  token: 'login-token', accountType: 'customer', miniUserID: ++nextUserID,
  currentCustomerID: 24, employeeID: 0,
})
let storage: Map<string, unknown>

beforeEach(() => {
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://test.invalid')
  storage = new Map()
  vi.stubGlobal('uni', {
    getStorageSync: (key: string) => storage.get(key),
    setStorageSync: (key: string, value: unknown) => storage.set(key, value),
    removeStorageSync: (key: string) => storage.delete(key),
  })
})
afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals() })

describe('recipient consent', () => {
  it('requires a known identity, including for restored sessions', () => {
    const visitor = { ...session(), miniUserID: 0 }
    expect(hasRecipientConsent(visitor)).toBe(false)
    expect(() => acceptRecipientConsent(visitor)).toThrow()
    expect(consentScope({ ...session(), token: '' })).toBe('')
    expect(storage.size).toBe(0)
  })

  it('updates active views on grant and withdrawal and keeps credentials outside the receipt', () => {
    const user = session()
    const allowed = computed(() => hasRecipientConsent(user))
    expect(allowed.value).toBe(false)
    acceptRecipientConsent(user)
    expect(allowed.value).toBe(true)
    expect(storage.get(consentScope(user))).toMatchObject({version: recipientPolicyVersion, acceptedAt: expect.any(Number)})
    expect(consentScope(user)).not.toContain(user.token)
    revokeRecipientConsent(user)
    expect(allowed.value).toBe(false)
    expect(storage.size).toBe(0)
  })

  it('isolates customer, person, account type and build environment', () => {
    const user = session()
    acceptRecipientConsent(user)
    expect(hasRecipientConsent({ ...user, currentCustomerID: 25 })).toBe(false)
    expect(hasRecipientConsent({ ...user, accountType: 'preview' })).toBe(false)
    expect(hasRecipientConsent(session())).toBe(false)
    vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'production')
    expect(hasRecipientConsent(user)).toBe(false)
  })

  it('reuses a current receipt on this device and requires fresh consent after a policy change', () => {
    const current = session()
    storage.set(consentScope(current), {version: recipientPolicyVersion, acceptedAt: 1})
    expect(hasRecipientConsent(current)).toBe(true)
    const old = session()
    storage.set(consentScope(old), {version: 'previous', acceptedAt: 1})
    expect(hasRecipientConsent(old)).toBe(false)
  })

  it('keeps access closed when a grant cannot be persisted', () => {
    const user = session()
    uni.setStorageSync = () => { throw new Error('full') }
    expect(() => acceptRecipientConsent(user)).toThrow('full')
    expect(hasRecipientConsent(user)).toBe(false)
  })
})
