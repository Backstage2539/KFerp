import { describe, expect, it } from 'vitest'
import { customerPriceUsages } from './customerPriceUsages'
import { readFileSync } from 'node:fs'

describe('customer price table usages', () => {
  it('derives the applicable tables from capabilities and enabled modes', () => {
    expect(customerPriceUsages(['product_order'], [])).toEqual(['product_order'])
    expect(customerPriceUsages(['direct_ship'], ['stock'])).toEqual(['product_order'])
    expect(customerPriceUsages(['direct_ship'], ['on_demand'])).toEqual(['direct_ship'])
    expect(customerPriceUsages(['direct_ship'], ['stock', 'on_demand'])).toEqual(['product_order', 'direct_ship'])
    expect(customerPriceUsages([], ['stock', 'on_demand'])).toEqual([])
  })
  it('loads the price page independently of the direct ship product catalog', () => {
    const page = readFileSync(new URL('../pages/bean-list-center/bean-list-center.vue', import.meta.url), 'utf8')
    expect(page).not.toContain('fetchDirectShipCatalog')
    expect(page).toContain('session.fulfillmentModes')
    expect(page).toContain('CustomerPricePreview')
  })
})
