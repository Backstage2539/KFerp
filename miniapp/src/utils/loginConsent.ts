import type { LoginResponse } from '../api/customerPortal'

export const loginPolicyVersion = '2026-11-24'
export type LoginConsent = { account: string; version: string; acceptedAt: number }

export function loginAccount(identity: Pick<LoginResponse, 'account_type' | 'employee_id' | 'mini_user_id'>): string {
  if (identity.account_type === 'employee' && Number(identity.employee_id) > 0) return 'employee-' + identity.employee_id
  return Number(identity.mini_user_id) > 0 ? 'user-' + identity.mini_user_id : ''
}

export function currentLoginConsent(record?: Partial<LoginConsent> | null): record is LoginConsent {
  return record?.version === loginPolicyVersion && typeof record.account === 'string'
    && /^(employee|user)-[1-9]\d*$/.test(record.account) && Number(record.acceptedAt) > 0
}
