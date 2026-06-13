import { apiGet, apiSend } from './client'

export async function fetchRunningProduction() {
  return apiGet('/api/produce/running')
}

export async function finishRunningProduction(payload) {
  return apiSend('/api/produce/running/finish', { body: payload })
}

export async function cancelRunningProduction(id) {
  return apiSend('/api/produce/running/cancel', { body: { id } })
}

export async function fetchProductionWorkstationOverview(params = {}) {
  const query = new URLSearchParams()
  if (params.limit) query.set('limit', String(params.limit))
  const suffix = query.toString()
  return apiGet(`/api/production/workstation-overview${suffix ? `?${suffix}` : ''}`)
}

export async function assignProductionSchedule(payload) {
  return apiSend('/api/production-schedule/assign', { body: payload })
}

export async function runProductionTaskAction(endpoint, payload = {}) {
  return apiSend(endpoint, { body: payload })
}
