import { beforeEach, expect, it, vi } from 'vitest'
import { miniRequest, MiniRequestError } from './client'
import { fetchEntryCatalog, fetchEntryBase } from './employeeEntry'
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
