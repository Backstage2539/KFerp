import {describe, expect, it} from 'vitest'
import { notificationDetailRoute, notificationDetailAPI, formatNoticeCents, parseNotificationDetail } from './notificationDetail'

describe('financial notification detail',()=>{
 it('constructs paths only for supported types and positive integer ids',()=>{
  expect(notificationDetailRoute('receipt','42')).toBe('/pages/notification-detail/notification-detail?type=receipt&id=42')
  expect(notificationDetailAPI('bill','9')).toBe('/api/mini/notification-details/bills/9')
  for(const value of ['0','-1','1.2','01','1&customer_id=2','1234567890123456']) expect(parseNotificationDetail('bill',value)).toBeNull()
  expect(parseNotificationDetail('orders','42')).toBeNull()
 })
 it('formats integer cents without changing the amount',()=>{
  expect(formatNoticeCents(12321)).toBe('123.21')
  expect(formatNoticeCents(5)).toBe('0.05')
  expect(formatNoticeCents(-1)).toBe('—')
  expect(formatNoticeCents(1.5)).toBe('—')
 })
})
