import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const page = readFileSync(resolve('src/pages/employee-orders/employee-orders.vue'), 'utf8')
const editor = () => readFileSync(resolve('src/components/EmployeeOrderListStatus.vue'), 'utf8')

describe('employee order list', () => {
  it('loads a short first page and appends more without clearing visible rows', () => {
    expect(page).toContain('onReachBottom')
    expect(page).toContain('fetchEmployeeOrderPage')
    expect(page).toContain('nextCursor')
    expect(page).toContain('...rows.value')
  })

  it('edits each status in place using the current order revision', () => {
    expect(page).toContain('EmployeeOrderListStatus')
    expect(editor()).toContain('edit_revision')
    expect(editor()).toContain('payment_unchanged')
    expect(editor()).toContain('chooseEmployeePaymentVoucher')
  })
})
