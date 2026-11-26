import { miniRequest, isUnauthorizedRequestError } from './client'
import type { EmployeeOrderForm } from './customerPortal'
import { employeeEntryCache } from '../utils/employeeEntryCache'

type Conditional = { revision: string; unchanged?: boolean }
type EntryFields = Partial<EmployeeOrderForm> & Conditional & { selected_revision?: string }

export async function readEntryPart<T extends Conditional>(token: string, path: string): Promise<T> {
  try {
    return await employeeEntryCache.read<T>(token, path, known => {
      const suffix = known ? (path.includes('?') ? '&' : '?') + 'known_revision=' + encodeURIComponent(known) : ''
      return miniRequest<T>(path + suffix, { token })
    })
  } catch (err) {
    if (isUnauthorizedRequestError(err)) employeeEntryCache.clear()
    throw err
  }
}

const root = '/api/mini/employee/order-entry/'
export function fetchEntryBase(token: string) {
  return readEntryPart<EntryFields>(token, root + 'base')
}
function catalogPath(customer: number, selected: number[], preferred: number[]) {
  const ids = selected.length ? [...new Set(selected)].sort((a,b) => a-b) : preferred
  const key = selected.length ? 'selected_price_table_ids' : 'preferred_price_table_ids'
  return root + 'catalog?customer_id=' + customer + (ids.length ? '&' + key + '=' + ids.join(',') : '')
}
export async function fetchEntryCatalog(token: string, customer: number, selected: number[] = [], preferred: number[] = []) {
  const path = catalogPath(customer, selected, preferred)
  const result = await readEntryPart<EntryFields>(token, path)
  const explicit = catalogPath(customer,result.selected_price_table_ids || [],[])
  if (result.selected_revision && explicit !== path) {
    employeeEntryCache.remember(token, explicit, {...result, revision: result.selected_revision, catalog_updated:false})
  }
  return result
}
export function fetchEntryWarehouses(token: string, customer: number) {
  return readEntryPart<EntryFields>(token, root + 'warehouses?customer_id=' + customer)
}
