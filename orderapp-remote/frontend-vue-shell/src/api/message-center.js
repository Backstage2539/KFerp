import { apiGet, apiSend } from './client'

export function fetchERPNotifications(limit = 100) {
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

const external = '/api/message-center/external'
export const fetchOrderNotifications = () => apiGet(external)
export const saveOrderNotifications = body => apiSend(external+'/config',{method:'PUT',body})
export const requestNotificationReselection = () => apiSend(external+'/prompt/reselect',{body:{}})
export const fetchNotificationSubscribers = q => apiGet(`${external}/subscribers?q=${encodeURIComponent(q || '')}`)
export const requestSubscriberReselection = body => apiSend(external+'/subscribers/reselect',{body})
export const fetchOrderNotificationTemplates = () => apiGet(external+'/templates')
export const testOrderNotification = body => apiSend(external+'/tests',{body})
export const retryOrderNotification = (id,confirmed_not_sent=false) => apiSend(`${external}/deliveries/${id}/retry`,{body:{confirmed_not_sent}})
export const confirmOrderNotification = id => apiSend(`${external}/deliveries/${id}/confirm`,{body:{}})
export const verifyBusinessNotificationTest = id => apiSend(`${external}/deliveries/${id}/verify-business-test`,{body:{confirmed_received_and_opened:true}})
export const addNotificationDevice = body => apiSend(external+'/devices',{body})
export const revokeNotificationDevice = id => apiSend(`${external}/devices/${id}`,{method:'DELETE'})
