import { describe, expect, it } from 'vitest'
import { visibleHomeEntries } from './capabilities'

describe('visibleHomeEntries', () => {
  it('shows only entries enabled by customer capabilities', () => {
    const entries = visibleHomeEntries([
      { code: 'direct_ship', enabled: true },
      { code: 'processing', enabled: false },
      { code: 'settlement', enabled: true },
    ])
    expect(entries.map((entry) => entry.key)).toEqual(['directShip', 'settlement'])
  })
})
