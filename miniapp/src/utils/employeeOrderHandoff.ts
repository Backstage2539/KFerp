import type { EmployeeOrderDetailResponse } from '../api/customerPortal'

let pending: { token: string; response: EmployeeOrderDetailResponse; readAt: number } | undefined

export function passOrderToEditor(token: string, response: EmployeeOrderDetailResponse, readAt: number) {
  pending = { token, response, readAt }
}

export function takeOrderForEditor(token: string, id: number, now = Date.now()) {
  const last = pending
  pending = undefined
  if (token && last?.token === token && last.response.order.id === id && now - last.readAt < 30_000) {
    return last.response
  }
}
