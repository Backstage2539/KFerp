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
