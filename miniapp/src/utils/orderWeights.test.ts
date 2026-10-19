import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { orderWeightLines } from './orderWeights'
import { showDirectShipSupplyPreview } from './customerFulfillment'
const source = (path: string) => readFileSync(resolve(path), 'utf8')
describe('frozen order weight display', () => {
  it('distinguishes material reference weight and logistics weight without recalculating history', () => {
    expect(orderWeightLines({ material_unit_weight_g: 90, material_item_weight_g: 180, logistics_unit_weight_g: 100, logistics_item_weight_g: 200 })).toEqual([
      { label: '用料重量（参考）', value: '单件 90g · 本项 180g' },
      { label: '物流重量', value: '单件 100g · 本项 200g' },
    ])
    expect(orderWeightLines({}).map(row => row.value)).toEqual(['未记录', '未记录'])
    expect(orderWeightLines({ weight_snapshot_json: '{"version":1}' }).map(row => row.value)).toEqual(['未计算', '未配置'])
    expect(orderWeightLines({ weight_snapshot_json: '{"rule_version":1,"historical_unknown":true}' }).map(row => row.value)).toEqual(['未记录', '未记录'])
    expect(orderWeightLines({ logistics_unit_weight_g: 0.001 })[1].value).toContain('0.001g')
  })
  it('hides supply allocation for on-demand dropship and retains ordinary fulfillment supply', () => {
    expect(showDirectShipSupplyPreview({ fulfillmentMode: 'on_demand', isProductOrder: false, retainedPurchase: false })).toBe(false)
    expect(showDirectShipSupplyPreview({ fulfillmentMode: 'stock', isProductOrder: false, retainedPurchase: false })).toBe(false)
    expect(showDirectShipSupplyPreview({ fulfillmentMode: '', isProductOrder: true, retainedPurchase: true })).toBe(false)
    expect(showDirectShipSupplyPreview({ fulfillmentMode: '', isProductOrder: true, retainedPurchase: false })).toBe(true)
    expect(showDirectShipSupplyPreview({ fulfillmentMode: '', isProductOrder: false, retainedPurchase: false })).toBe(true)
  })
  it('wires customer, employee, draft quote and saved dropship details to read-only weights', () => {
    for (const path of ['src/pages/service/service.vue', 'src/pages/employee-order-detail/employee-order-detail.vue', 'src/components/CustomerDirectShipPanel.vue']) expect(source(path)).toContain('<OrderItemWeights')
    const panel=source('src/components/CustomerDirectShipPanel.vue')
    expect(panel).toContain('showDirectShipSupplyPreview')
    expect(panel).toContain('v-if="showSupplyPreview && previewSupplyRows.length"')
    expect(panel).toContain('if (!showSupplyPreview.value) return')
    expect(panel).toContain('v-if="showSupplyPreview && blockingShortages.length"')
    expect(panel).toContain("freight_protocol: 'deferred_freight_v1'")
    const component=source('src/components/OrderItemWeights.vue')
    expect(component).toContain('orderWeightLines')
    expect(component).not.toMatch(/v-model|request\(/)
    const freight=source('src/components/EmployeeFreightEstimate.vue')
    expect(freight).toContain('freightLabel(quote)')
    expect(freight).toContain('!quote.estimate_known && quote.cost_pending_reason')
  })
})
