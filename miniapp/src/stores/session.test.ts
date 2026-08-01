import { describe, expect, it } from 'vitest'
import { miniappTokenStorageKey } from './session'

describe('miniapp session storage boundary', () => {
  it('uses separate token keys for development and production', () => {
    expect(miniappTokenStorageKey('development')).toBe('kferp.mini.development.token')
    expect(miniappTokenStorageKey('production')).toBe('kferp.mini.production.token')
    expect(miniappTokenStorageKey('development')).not.toBe('kferp.mini.token')
  })
})
