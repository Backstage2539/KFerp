export type NotificationDetailType = 'receipt' | 'bill'
export type NotificationDetailTarget = { type: NotificationDetailType, id: string }

export function parseNotificationDetail(type: unknown, id: unknown): NotificationDetailTarget | null {
  if ((type !== 'receipt' && type !== 'bill') || typeof id !== 'string' || !/^[1-9][0-9]{0,14}$/.test(id)) return null
  return { type, id }
}

export function notificationDetailRoute(type: unknown, id: unknown): string {
  const target = parseNotificationDetail(type, id)
  return target ? `/pages/notification-detail/notification-detail?type=${target.type}&id=${target.id}` : ''
}

export function notificationDetailAPI(type: unknown, id: unknown): string {
  const target = parseNotificationDetail(type, id)
  return target ? `/api/mini/notification-details/${target.type === 'receipt' ? 'receipts' : 'bills'}/${target.id}` : ''
}

export function formatNoticeCents(value: unknown): string {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) return '—'
  return `${Math.floor(value / 100)}.${String(value % 100).padStart(2, '0')}`
}
