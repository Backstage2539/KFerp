import { apiGet, apiSend } from './client'

export function fetchERPNotifications(limit = 5) {
  const url = new URL('/api/message-center/notifications', window.location.origin)
  url.searchParams.set('channel', 'erp_platform')
  url.searchParams.set('status', 'unread')
  url.searchParams.set('limit', String(limit))
  return apiGet(url)
}

export function markNotificationRead(id) {
  return apiSend(`/api/message-center/notifications/${id}/read`)
}

export function fetchNotificationRules() {
  return apiGet('/api/message-center/rules')
}

export function saveNotificationRule(rule) {
  return apiSend('/api/message-center/rules', { body: rule })
}
