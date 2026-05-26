import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { expectedLossRate, expectedYieldRate, formatPercent } from './manufacturing-loss.js'

describe('manufacturing loss helpers', () => {
  it('derives expected loss rate from yield rate', () => {
    assert.equal(expectedLossRate(0.82), 0.18)
  })

  it('derives expected yield rate from loss rate', () => {
    assert.equal(expectedYieldRate(0.18), 0.82)
  })

  it('formats percentages with one decimal by default', () => {
    assert.equal(formatPercent(0.185), '18.5%')
  })
})
