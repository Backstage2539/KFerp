import { describe, expect, it } from 'vitest'
import { hydrateEmployeeOrderEditItems } from './employeeOrder'
import type { EmployeeOrderDetailItem } from '../api/customerPortal'

describe('order snapshot before the current sales catalog arrives', () => {
  it('preserves the saved price and does not label an unverified product unavailable', () => {
    const item = {item_id:7, product_id:20, product_name:'旧版', qty:'3', unit_price:'45.60'} as EmployeeOrderDetailItem
    const [draft] = hydrateEmployeeOrderEditItems([item], [], 1, false, false, true)
    expect(draft).toMatchObject({ item_id:7, product_id:20, unit_price:45.60, qty:3, validation_error:'' })
    expect(hydrateEmployeeOrderEditItems([item], [], 1)[0].validation_error).toBeTruthy()
  })
})

import { passOrderToEditor, takeOrderForEditor } from './employeeOrderHandoff'
import type { EmployeeOrderDetailResponse } from '../api/customerPortal'

it('passes a fresh detail once to the same employee and order, never across sessions', () => {
  const response = {order:{id:5}, can_edit:true} as EmployeeOrderDetailResponse
  passOrderToEditor('employee', response, 100)
  expect(takeOrderForEditor('employee', 5, 200)).toBe(response)
  expect(takeOrderForEditor('employee', 5, 201)).toBeUndefined()
  passOrderToEditor('employee', response, 100)
  expect(takeOrderForEditor('another', 5, 200)).toBeUndefined()
  passOrderToEditor('employee', response, 100)
  expect(takeOrderForEditor('employee', 6, 200)).toBeUndefined()
  passOrderToEditor('employee', response, 100)
  expect(takeOrderForEditor('employee', 5, 60000)).toBeUndefined()
})
