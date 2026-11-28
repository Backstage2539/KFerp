import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fetchEntryCatalog, fetchEntryWarehouses } from '../api/employeeEntry'
import { cancelHomeOrderEntry, clearHomeOrderEntry, passHomeOrderEntry, takeHomeOrderEntry } from './homeOrderHandoff'

vi.mock('../api/employeeEntry', () => ({
  fetchEntryCatalog: vi.fn(),
  fetchEntryWarehouses: vi.fn(),
}))

const base = { revision: '1', customers: [{ id: 14, name: '测试客户' }] }
const customer = { kind: 'customer', customerId: 14 } as const

describe('首页录单交接', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.mocked(fetchEntryCatalog).mockResolvedValue({ revision: 'catalog' })
    vi.mocked(fetchEntryWarehouses).mockResolvedValue({ revision: 'warehouses' })
  })
  afterEach(() => {
    clearHomeOrderEntry()
    vi.clearAllMocks()
    vi.useRealTimers()
  })
  it('选中的客户在导航前开始加载，一次消费同一份基础资料和两个请求', async () => {
    const id = passHomeOrderEntry('s', 'scope', base, customer)
    expect(fetchEntryCatalog).toHaveBeenCalledTimes(1)
    expect(fetchEntryCatalog).toHaveBeenCalledWith('s', 14, [])
    expect(fetchEntryWarehouses).toHaveBeenCalledTimes(1)
    expect(fetchEntryWarehouses).toHaveBeenCalledWith('s', 14)
    const result = takeHomeOrderEntry(id, 's', 'scope')
    expect(result?.base).toBe(base)
    expect(result?.intent).toEqual(customer)
    expect(await result?.selection?.catalog).toEqual({ revision: 'catalog' })
    expect(await result?.selection?.warehouses).toEqual({ revision: 'warehouses' })
    expect(takeHomeOrderEntry(id, 's', 'scope')).toBeUndefined()
  })
  it('过期、切换账号/授权范围和撤回后均不能消费交接', () => {
    for (const [token, scope] of [['other', 'scope'], ['s', 'other']]) {
      const id = passHomeOrderEntry('s', 'scope', base, { kind: 'create' })
      expect(takeHomeOrderEntry(id, token, scope)).toBeUndefined()
      expect(takeHomeOrderEntry(id, 's', 'scope')).toBeUndefined()
    }
    const expired = passHomeOrderEntry('s', 'scope', base, { kind: 'create' })
    vi.advanceTimersByTime(31_000)
    expect(takeHomeOrderEntry(expired, 's', 'scope')).toBeUndefined()
    const cleared = passHomeOrderEntry('s', 'scope', base, { kind: 'create' })
    clearHomeOrderEntry()
    expect(takeHomeOrderEntry(cleared, 's', 'scope')).toBeUndefined()
  })
  it('旧导航的取消与消费均不会影响刚选中的新客户', () => {
    const old = passHomeOrderEntry('s', 'scope', base, { kind: 'create' })
    const current = passHomeOrderEntry('s', 'scope', base, customer)
    cancelHomeOrderEntry(old)
    expect(takeHomeOrderEntry(old, 's', 'scope')).toBeUndefined()
    expect(takeHomeOrderEntry(current, 's', 'scope')?.intent).toEqual(customer)
    const failed = passHomeOrderEntry('s', 'scope', base, { kind: 'create' })
    cancelHomeOrderEntry(failed)
    expect(takeHomeOrderEntry(failed, 's', 'scope')).toBeUndefined()
  })
  it('未验证的客户不能触发客户数据预取', () => {
    expect(() => passHomeOrderEntry('s', 'scope', base, { kind: 'customer', customerId: 23 })).toThrow()
    expect(fetchEntryCatalog).not.toHaveBeenCalled()
    expect(fetchEntryWarehouses).not.toHaveBeenCalled()
  })
})
