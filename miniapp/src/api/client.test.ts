import { describe, expect, it } from 'vitest'
import { buildAPIURL, normalizeAPIBase } from './client'

describe('miniapp API client URL helpers', () => {
  it('normalizes configured API base by trimming whitespace and trailing slashes', () => {
    expect(normalizeAPIBase(' https://erp.example.com/app/// ')).toBe('https://erp.example.com/app')
  })

  it('builds mini API URLs from the configured base and request path', () => {
    expect(buildAPIURL('/api/mini/me', 'https://erp.example.com/app/')).toBe(
      'https://erp.example.com/app/api/mini/me',
    )
  })
})
