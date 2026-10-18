import { miniRequest } from './client'
import type { OrderDetailsText } from '../utils/orderDetailsCopy'
export function fetchOrderDetailsText(token: string, id: number, employee = false): Promise<OrderDetailsText> {
  return miniRequest<OrderDetailsText>(`/api/mini/${employee ? 'employee/' : ''}orders/${id}/copy-details`, { token })
}
