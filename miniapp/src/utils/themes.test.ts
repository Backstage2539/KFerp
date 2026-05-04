import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  defaultMiniappThemeKey,
  miniappThemeClass,
  miniappThemeMeta,
  miniappThemeOptions,
  normalizeMiniappThemeKey,
} from './themes'

describe('miniapp themes', () => {
  it('exposes the three built-in customer portal themes', () => {
    expect(defaultMiniappThemeKey).toBe('coffee_factory')
    expect(miniappThemeOptions.map((item) => item.key)).toEqual([
      'coffee_factory',
      'clean_ops',
      'premium_partner',
    ])
  })

  it('normalizes invalid theme keys to the default coffee factory theme', () => {
    expect(normalizeMiniappThemeKey('clean_ops')).toBe('clean_ops')
    expect(normalizeMiniappThemeKey('premium_partner')).toBe('premium_partner')
    expect(normalizeMiniappThemeKey('')).toBe('coffee_factory')
    expect(normalizeMiniappThemeKey('unknown')).toBe('coffee_factory')
  })

  it('maps theme keys to stable page classes and display metadata', () => {
    expect(miniappThemeClass('coffee_factory')).toBe('theme-coffee-factory')
    expect(miniappThemeClass('clean_ops')).toBe('theme-clean-ops')
    expect(miniappThemeClass('premium_partner')).toBe('theme-premium-partner')
    expect(miniappThemeMeta('premium_partner').eyebrow).toBe('ROASTERY PARTNER')
    expect(miniappThemeMeta('unknown').eyebrow).toBe('QACOOHEE SERVICE')
  })
})

describe('miniapp theme source wiring', () => {
  it('applies theme classes to login, home, and service pages', () => {
    for (const file of [
      'src/pages/login/login.vue',
      'src/pages/home/home.vue',
      'src/pages/service/service.vue',
    ]) {
      const source = readFileSync(resolve(file), 'utf8')
      expect(source).toContain('miniappThemeClass')
      expect(source).toContain('theme-coffee-factory')
      expect(source).toContain('theme-clean-ops')
      expect(source).toContain('theme-premium-partner')
    }
  })
})
