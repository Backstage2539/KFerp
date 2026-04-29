import { apiGet, apiSend } from './client'

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

export function resetEmployeePassword(employeeId, password) {
  return apiSend('/api/auth/password/reset', {
    method: 'POST',
    body: { employee_id: employeeId, password },
  })
}

export function logoutCurrentSession() {
  return apiSend('/api/auth/logout', { method: 'POST' })
}
