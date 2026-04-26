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
