import { apiGet, apiSend } from './client.js'

export function fetchUISettings() {
  return apiGet('/api/ui-settings')
}

export function saveUISettings(settings) {
  return apiSend('/api/ui-settings', {
    method: 'PUT',
    body: settings,
  })
}
