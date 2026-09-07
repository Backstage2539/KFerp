import { describe, it, expect } from 'vitest'
import { priceTableGroups, replaceSelectedPriceTable, priceTableLabel } from './priceTables'

describe('named price tables', () => {
  const tables = [
    { id: 1, list_type: 'commercial', classification_template_id: 10, table_name: '227g价格表', version_no: 'V3', is_default: true },
    { id: 2, list_type: 'commercial', classification_template_id: 10, table_name: '1kg价格表', version_no: 'V3' },
    { id: 3, list_type: 'green', classification_template_id: 20, table_name: '生豆价格表', version_no: 'V2', is_default: true },
  ]
  it('groups by product type and replaces only the selected type', () => {
    const groups = priceTableGroups(tables)
    expect(groups).toHaveLength(2)
    expect(replaceSelectedPriceTable([1,3],groups[0],2)).toEqual([3,2])
    expect(() => replaceSelectedPriceTable([1,3],groups[0],3)).toThrow()
    expect(priceTableLabel(tables[0])).toContain('227g价格表')
    expect(priceTableLabel(tables[0])).toContain('V3')
  })
})
