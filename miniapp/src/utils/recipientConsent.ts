import { clearHomeOrderEntry } from './homeOrderHandoff'
import { employeeEntryCache } from './employeeEntryCache'
import { reactive } from 'vue'
import { miniappStorageKey } from '../config/environment'

export const recipientPolicyVersion = '2026-11-24'

type ConsentSession = {
  token: string
  accountType: string
  miniUserID: number
  currentCustomerID: number
  employeeID: number
}

const permissions = reactive(new Map<string, boolean>())

export function consentScope(session: ConsentSession): string {
  if (!session.token) return ''
  const user = session.miniUserID > 0 ? `user-${session.miniUserID}`
    : session.employeeID > 0 ? `employee-${session.employeeID}` : ''
  if (!user) return ''
  return miniappStorageKey(`recipient-consent.${session.accountType}.${user}.${session.currentCustomerID || 0}`)
}

export function hasRecipientConsent(session: ConsentSession): boolean {
  const key = consentScope(session)
  if (!key) return false
  const cached = permissions.get(key)
  if (cached !== undefined) return cached
  try {
    const record = uni.getStorageSync(key)
    const allowed = record?.version === recipientPolicyVersion && Number(record.acceptedAt) > 0
    permissions.set(key, allowed)
    return allowed
  } catch {
    return false
  }
}

export function acceptRecipientConsent(session: ConsentSession): void {
  const key = consentScope(session)
  if (!key) throw new Error('正在确认当前账号，请稍后再试')
  uni.setStorageSync(key, { version: recipientPolicyVersion, acceptedAt: Date.now() })
  permissions.set(key, true)
}

export function revokeRecipientConsent(session: ConsentSession): void {
  const key = consentScope(session)
  if (!key) return
  employeeEntryCache.clear()
  clearHomeOrderEntry()
  uni.removeStorageSync(key)
  permissions.set(key, false)
}
