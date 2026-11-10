import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { miniappTokenStorageKey, useSessionStore } from './session'

const employee = { mini_user_id: 9, current_customer_id: 0, bindings: [], capabilities: [], account_type: 'employee', employee_id: 5, employee_name: '管理员', roles: ['admin'], permissions: ['orders.read'] }
const customer = { mini_user_id: 0, current_customer_id: 74, current_customer_name: '客户甲', bindings: [], capabilities: [], account_type: 'preview', employee_id: 5, roles: ['admin'], permissions: [] }
let storage: Map<string, unknown>
beforeEach(() => {
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
  storage = new Map()
  vi.stubGlobal('uni', { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: unknown) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key) })
  setActivePinia(createPinia())
})
describe('administrator customer preview', () => {
  it('keeps the employee credential in storage and restores it on exit', () => {
    const session = useSessionStore()
    session.setToken('employee-token')
    session.applyContext(employee)
    session.beginCustomerPreview('preview-token', customer)
    expect(session.token).toBe('preview-token')
    expect(session.isCustomerPreview).toBe(true)
    expect(storage.get(miniappTokenStorageKey())).toBe('employee-token')
    session.endCustomerPreview()
    expect(session.token).toBe('employee-token')
    expect(session.isCustomerPreview).toBe(false)
    expect(session.currentCustomerID).toBe(0)
    expect(storage.get(miniappTokenStorageKey())).toBe('employee-token')
  })
  it('restores the employee session after preview authentication expires', () => {
    const session = useSessionStore()
    session.setToken('employee-token')
    session.applyContext(employee)
    session.beginCustomerPreview('preview-token', customer)
    session.clearSession()
    expect(session.token).toBe('employee-token')
    expect(session.isCustomerPreview).toBe(false)
    expect(storage.get(miniappTokenStorageKey())).toBe('employee-token')
  })
  it('does not persist a preview across a full restart', () => {
    const session = useSessionStore()
    session.setToken('employee-token')
    session.applyContext(employee)
    session.beginCustomerPreview('preview-token', customer)
    setActivePinia(createPinia())
    const reopened = useSessionStore()
    expect(reopened.token).toBe('employee-token')
    expect(reopened.isCustomerPreview).toBe(false)
  })
  it('honors explicit logout even during preview', () => {
    const session = useSessionStore()
    session.setToken('employee-token')
    session.applyContext(employee)
    session.beginCustomerPreview('preview-token', customer)
    session.logout()
    expect(session.token).toBe('')
    expect(session.autoRestoreBlocked).toBe(true)
    expect(storage.get(miniappTokenStorageKey())).toBeUndefined()
  })
  it('refuses preview for non-admin or an invalid preview context', () => {
    const session = useSessionStore()
    session.setToken('employee-token')
    session.applyContext({ ...employee, roles: ['sales'] })
    expect(typeof session.beginCustomerPreview).toBe('function')
    expect(() => session.beginCustomerPreview('preview-token', customer)).toThrow()
    session.applyContext(employee)
    expect(() => session.beginCustomerPreview('preview-token', { ...customer, account_type: 'customer' })).toThrow()
    expect(session.token).toBe('employee-token')
  })
})
