import { apiGet, apiSend } from './client.js'

export function hasStoredAuthToken() {
  try {
    if (typeof window === 'undefined') return false
    return !!window.localStorage?.getItem('auth_token')
  } catch {
    return false
  }
}

export function clearStoredAuthToken() {
  try {
    if (typeof window === 'undefined') return
    window.localStorage?.removeItem('auth_token')
  } catch {
    // localStorage may be unavailable in private or embedded contexts.
  }
}

export function fetchCurrentActor() {
  return apiGet('/api/auth/me')
}

export function fetchRoles() {
  return apiGet('/api/auth/roles')
}

export function fetchEmployeeRoles() {
  return apiGet('/api/auth/employee-roles')
}

export function saveEmployeeRoles(employeeId, roleCodes) {
  return apiSend('/api/auth/employee-roles', {
    method: 'POST',
    body: { employee_id: employeeId, role_codes: roleCodes },
  })
}

export function fetchAuthAccounts() {
  return apiGet('/api/auth/accounts')
}

export function setAccountState(employeeId, loginEnabled) {
  return apiSend('/api/auth/account-state', {
    method: 'POST',
    body: { employee_id: employeeId, login_enabled: loginEnabled },
  })
}

export function setAccountType(employeeId, accountType) {
  return apiSend('/api/auth/account-type', {
    method: 'POST',
    body: { employee_id: employeeId, account_type: accountType },
  })
}

export function resetEmployeePassword(employeeId, password) {
  return apiSend('/api/auth/password/reset', {
    method: 'POST',
    body: { employee_id: employeeId, password },
  })
}

export function logoutCurrentSession() {
  return apiSend('/api/auth/logout', { method: 'POST' })
}
