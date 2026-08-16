import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function source(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('customer closed-loop miniapp pages', () => {
  it('keeps direct ship to one new-shipment flow and reuses the shared recipient parser', () => {
    const page = source('src/components/CustomerDirectShipPanel.vue')

    expect(page).toContain('新建发货')
    expect(page).toContain('parseEmployeeCustomerRecipient')
    expect(page).toContain('粘贴收货信息')
    expect(page).toContain('createDirectShipRequest')
    expect(page).toContain('idempotency_key')
    expect(page).toContain('ProductFamilyPickerSheet')
    expect(page).toContain('createDirectShipDraftLine')
    expect(page).toContain('buildDirectShipDraftItems')
    expect(page).toContain('v-if="!showCreate"')
    expect(page).toContain('product_name ||')
    expect(page).toContain('pkg.events')
    expect(page).toContain('发货时间')
    expect(page).toContain('directShipStatusLabel')
    expect(page).toContain('line.bom_spec_id')
    expect(page).toContain('item.bom_spec_id')
    expect(page).toContain('inventory_unit')
    expect(page).not.toContain('新建代发批次')
    expect(page).not.toContain('导入代发地址')
    expect(page).not.toContain('现货商品')
  })

  it('selects multiple production targets and previews the server BOM without input-material controls', () => {
    const page = source('src/components/CustomerProcessingPanel.vue')

    expect(page).toContain('CustomerProductSelector')
    expect(page).toContain('previewProcessingRequest')
    expect(page).toContain('mergeProcessingTargetLines')
    expect(page).toContain('最大可生产')
    expect(page).toContain('客户库存')
    expect(page).toContain('工厂库存')
    expect(page).toContain('在制品')
    expect(page).toContain('schedulePreview')
    expect(page).toContain('qty: 0')
    expect(page).toContain('productionStatusLabel')
    expect(page).toContain('bom_spec_id')
    expect(page).toContain('bom_variant_id')
    expect(page).toContain('inventory_unit')
    expect(page).not.toContain('选择投入的物料')
    expect(page).not.toContain('投入生豆克重')
    expect(page).not.toContain('新建发货订单')
  })

  it('opens central inventory batches in a separate page and prefills production requests', () => {
    const list = source('src/components/CustomerInventoryPanel.vue')
    const detail = source('src/pages/customer-inventory-detail/customer-inventory-detail.vue')

    expect(list).toContain('fetchCustomerInventory')
    expect(list).toContain('customerInventoryDetailPath')
    expect(list).toContain('生成生产工单')
    expect(list).not.toContain('fetchCustomerInventoryBatches')
    expect(detail).toContain('fetchCustomerInventoryBatches')
    expect(detail).toContain('生产日期')
    expect(detail).toContain('入库时间')
    expect(detail).toContain('历史库存，暂无生产日期')
    expect(detail).toContain('添加生产工单')
    expect(detail).toContain('processingPrefill.stage')
    expect(detail).toContain('bomSpecID')
    expect(detail).toContain('bomVariantID')
    expect(detail).toContain('inventoryUnit')
  })

  it('shows only pushed processing bills and opens snapshotted bill details', () => {
    const page = source('src/components/CustomerBillsPanel.vue')

    expect(page).toContain('fetchCustomerBills')
    expect(page).toContain('fetchCustomerBillDetail')
    expect(page).toContain('关联工单')
    expect(page).toContain('计费数量')
    expect(page).toContain('单价')
    expect(page).toContain('billingStatusLabel')
    expect(page).toContain('billingBasisLabel')
    expect(page).not.toContain('销售订单')
  })

  it('routes processing customers to a shipment-only fulfillment center', () => {
    const service = source('src/pages/service/service.vue')

    expect(service).toContain('CustomerDirectShipPanel')
    expect(service).toContain('CustomerProcessingPanel')
    expect(service).toContain('CustomerInventoryPanel')
    expect(service).toContain('CustomerBillsPanel')
    expect(service).toContain('isProcessingCustomer')
    expect(service).toContain("return '发货中心'")
    expect(service).toContain('inventory:${session.currentCustomerID}')
    expect(service).not.toContain('closedLoopRefreshKey')
    expect(service).toContain('v-if="!isProcessingCustomer"')
  })
})
