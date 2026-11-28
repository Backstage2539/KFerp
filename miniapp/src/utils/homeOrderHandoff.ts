import { fetchEntryCatalog, fetchEntryWarehouses, type fetchEntryBase } from '../api/employeeEntry'
import type { EmployeeOrderDraft } from '../api/customerPortal'

export type HomeOrderBase = Awaited<ReturnType<typeof fetchEntryBase>>
export type HomeOrderIntent =
  | { kind: 'draft'; draft: EmployeeOrderDraft }
  | { kind: 'customer'; customerId: number }
  | { kind: 'create' }

type Selection = {
  catalog: ReturnType<typeof fetchEntryCatalog>
  warehouses: ReturnType<typeof fetchEntryWarehouses>
}
type HomeOrderEntry = { base: HomeOrderBase; intent: HomeOrderIntent; selection?: Selection }

let sequence = 0
let pending: { id: string; token: string; scope: string; expires: number; entry: HomeOrderEntry } | undefined
let expiry: ReturnType<typeof setTimeout> | undefined
const lifetime = 30_000

export function clearHomeOrderEntry() {
  clearTimeout(expiry)
  expiry = undefined
  pending = undefined
}

export function cancelHomeOrderEntry(id: string) {
  if (pending?.id === id) clearHomeOrderEntry()
}

export function passHomeOrderEntry(token: string, scope: string, base: HomeOrderBase, intent: HomeOrderIntent) {
  clearHomeOrderEntry()
  if (!token || !scope) throw new Error('请先确认当前员工和收件信息授权')
  const entry: HomeOrderEntry = { base, intent }
  if (intent.kind === 'customer') {
    if (!base.customers?.some(customer => customer.id === intent.customerId)) {
      throw new Error('客户不在当前可选范围，请重新搜索')
    }
    const catalog = fetchEntryCatalog(token, intent.customerId, [])
    const warehouses = fetchEntryWarehouses(token, intent.customerId)
    // Navigation may finish after either request rejects. The form receives the original promises.
    void Promise.all([catalog, warehouses]).catch(() => {})
    entry.selection = { catalog, warehouses }
  }
  const id = Date.now().toString(36) + '-' + (++sequence).toString(36)
  pending = { id, token, scope, expires: Date.now() + lifetime, entry }
  expiry = setTimeout(() => cancelHomeOrderEntry(id), lifetime)
  return id
}

export function takeHomeOrderEntry(id: string, token: string, scope: string): HomeOrderEntry | undefined {
  if (!pending || pending.id !== id) return
  const saved = pending
  clearHomeOrderEntry()
  if (saved.token === token && saved.scope === scope && Date.now() < saved.expires) return saved.entry
}
