import { describe,it,expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { paymentSummary,copyOrderURL } from './prepayment'
describe('prepayment and list copy',()=>{
 it('keeps partial payment incomplete and derives both amounts',()=>{expect(paymentSummary('预付款（付款未完成）',30,100)).toEqual({paid:'30.00',unpaid:'70.00'})})
 it('settling balance displays full amount paid',()=>{expect(paymentSummary('已付款',30,100)).toEqual({paid:'100.00',unpaid:'0.00'})})
 it('validates copy navigation',()=>{expect(copyOrderURL(42)).toContain('copy_id=42'); expect(copyOrderURL(0)).toBe('')})
 it('list provides copy without opening detail and uses payment badges',()=>{const s=readFileSync(new URL('../pages/employee-orders/employee-orders.vue',import.meta.url),'utf8');expect(s).toContain('复制订单');expect(s).toContain('@tap.stop');expect(s).toContain('PaymentSummary')})
})

import { employeeOrderCopyPayload } from './employeeOrder'
it('copies commercial data but clears all receipts',()=>{
 const items = [{ product_id: 7, qty: 2, unit_price: 88, bom_spec_id: 9002 }]
 const result=employeeOrderCopyPayload({customer_id:3,prepayment_amount:'30.00',payment_method:'微信支付',pay_status_id:9,shipping_amount:'12.00'},items as any,'2026-09-07')
 expect(result.items).toEqual(items)
 expect(result.shipping_amount).toBe(12)
 expect(result.prepayment_amount).toBe(0)
 expect(result.payment_method).toBe('')
 expect(result.pay_status_id).toBe(0)
})
