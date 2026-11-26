import { beforeEach, expect, it, vi } from 'vitest'
import { miniRequest, MiniRequestError } from './client'
import { fetchEntryCatalog, fetchEntryBase, peekEntryCatalog } from './employeeEntry'
import { employeeEntryCache } from '../utils/employeeEntryCache'

vi.mock('./client', async (original) => ({ ...await original<typeof import('./client')>(), miniRequest: vi.fn() }))
const request = vi.mocked(miniRequest)
beforeEach(() => { employeeEntryCache.clear(); request.mockReset() })
it('validates the resolved selection without downloading the original default or another customer’s catalog', async () => {
  request.mockResolvedValueOnce({revision:'default', selected_revision:'named', selected_price_table_ids:[9,3], product_families:[{name:'A'}]})
  await fetchEntryCatalog('token',7)
  request.mockResolvedValueOnce({revision:'named',unchanged:true})
  expect((await fetchEntryCatalog('token',7,[3,9])).product_families).toEqual([{name:'A'}])
  expect(request.mock.calls[1][0]).toMatch(/customer_id=7&selected_price_table_ids=3,9&known_revision=named$/)
  request.mockResolvedValueOnce({revision:'B',product_families:[]})
  await fetchEntryCatalog('token',8,[3,9])
  expect(request.mock.calls[2][0]).not.toContain('known_revision')
})
it('drops cached contacts when the authorized session expires', async () => {
  request.mockResolvedValueOnce({revision:'contacts',customers:[{id:1}]})
  await fetchEntryBase('token')
  request.mockRejectedValueOnce(new MiniRequestError('过期',401))
  await expect(fetchEntryBase('token')).rejects.toThrow('过期')
  request.mockResolvedValueOnce({revision:'new',customers:[]})
  await fetchEntryBase('other')
  expect(request.mock.calls[2][0]).not.toContain('known_revision')
})

it('shows the same session and customer catalog while its network verification is pending', async () => {
  request.mockResolvedValueOnce({ revision: 'A', product_families: [{ name: '旧目录' }] })
  await fetchEntryCatalog('employee', 7)
  let finish!: (r: unknown) => void
  request.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  const verifying = fetchEntryCatalog('employee', 7)
  expect(peekEntryCatalog('employee', 7)?.product_families).toEqual([{ name: '旧目录' }])
  expect(peekEntryCatalog('employee', 8)).toBeUndefined()
  expect(peekEntryCatalog('other-login', 7)).toBeUndefined()
  finish({ revision: 'B', product_families: [{ name: '新目录' }] })
  await verifying
  expect(peekEntryCatalog('employee', 7)?.revision).toBe('B')
  employeeEntryCache.clear()
  expect(peekEntryCatalog('employee', 7)).toBeUndefined()
})
