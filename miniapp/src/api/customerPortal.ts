import { miniRequest } from './client'
import type { Capability } from '../utils/capabilities'

export type CustomerBinding = {
  customer_id: number
  customer_name: string
  role: string
  status: string
}

export type LoginResponse = {
  token: string
  mini_user_id: number
  current_customer_id: number
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export type MeResponse = {
  mini_user_id: number
  current_customer_id: number
  current_customer_name: string
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export function loginWithCode(code: string): Promise<LoginResponse> {
  return miniRequest<LoginResponse>('/api/mini/login', { method: 'POST', data: { code } })
}

export function fetchMe(token: string): Promise<MeResponse> {
  return miniRequest<MeResponse>('/api/mini/me', { token })
}
