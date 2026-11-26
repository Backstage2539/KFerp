import { describe, expect, it, vi } from 'vitest'
import { createEntrySessionCache } from './employeeEntryCache'

describe('employee order entry revisions', () => {
  it('reuses the payload and joins simultaneous validations', async () => {
    const cache = createEntrySessionCache()
    const fetch = vi.fn().mockResolvedValueOnce({revision:'a', items:[1]}).mockResolvedValueOnce({revision:'a', unchanged:true})
    const first = await cache.read('session','catalog:7',fetch)
    const [left,right] = await Promise.all([cache.read('session','catalog:7',fetch),cache.read('session','catalog:7',fetch)])
    expect(fetch.mock.calls).toEqual([[''],['a']])
    expect(left).toBe(first)
    expect(right).toBe(first)
  })
  it('discards results that arrive after logout or revocation', async () => {
    let finish!: (value:{revision:string})=>void
    const cache = createEntrySessionCache()
    const read = cache.read('session','base',()=>new Promise<{revision:string}>(resolve=>{finish=resolve}))
    cache.clear()
    finish({revision:'old'})
    await expect(read).rejects.toThrow()
  })
})
