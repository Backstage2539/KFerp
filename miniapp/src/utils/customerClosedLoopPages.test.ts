import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function source(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('customer closed-loop miniapp pages', () => {
  it('keeps direct ship to one new-shipment flow and reuses the shared recipient parser', () => {
    const page = source('src/components/CustomerDirectShipPanel.vue')

    expect(page).toContain('一件代发下单')
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
    expect(page).toContain('price_tables')
    expect(page).toContain('订单金额')
    expect(page).toContain('ERP 将按现有订单生产流程补货')
    expect(page).toContain('暂未获取物流轨迹')
    expect(page).not.toContain('新建代发批次')
    expect(page).not.toContain('导入代发地址')
    expect(page).not.toContain('现货商品')
  })

  it('selects multiple production targets and previews the server BOM without input-material controls', () => {
    const page = source('src/components/CustomerProcessingPanel.vue')

    expect(page).toContain('CustomerProductSelector')
    expect(page).toContain('previewProcessingRequest')
    expect(page).toContain('idempotency_key')
    expect(page).toContain('newProcessingIdempotencyKey')
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

  it('shows owned finished, green, packaging and semi-finished assets with batches and ledger', () => {
    const list = source('src/components/CustomerInventoryPanel.vue')

    expect(list).toContain('fetchCustomerAssetInventory')
    expect(list).toContain('fetchCustomerAssetInventoryLedger')
    expect(list).toContain("key: 'finished_product'")
    expect(list).toContain("key: 'green_bean'")
    expect(list).toContain("key: 'packaging'")
    expect(list).toContain("key: 'semi_finished'")
    expect(list).toContain('可用')
    expect(list).toContain('占用')
    expect(list).toContain('生产中')
    expect(list).toContain('库存批次')
    expect(list).toContain('出入库流水')
    expect(list).toContain('processingPrefill.stage')
  })

  it('shows unified ERP account statements with downloads, reconciliation and disputes', () => {
    const page = source('src/components/CustomerBillsPanel.vue')

    expect(page).toContain('fetchCustomerAccount')
    expect(page).toContain('confirmCustomerStatement')
    expect(page).toContain('createCustomerStatementDispute')
    expect(page).toContain("download('pdf')")
    expect(page).toContain("download('xlsx')")
    expect(page).toContain('商品货款')
    expect(page).toContain('加工费')
    expect(page).toContain('代发服务费')
    expect(page).toContain('确认对账')
    expect(page).toContain('不会改变付款状态')
    expect(page).toContain('ERP 回复')
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

  it('shows the current customer, ERP business contact, recent recipients and service explanation', () => {
    const page = source('src/pages/profile/profile.vue')

    expect(page).toContain('客户资料')
    expect(page).toContain('business_contact_name')
    expect(page).toContain('fetchDirectShipRequests')
    expect(page).toContain('常用收件人')
    expect(page).toContain('服务说明')
    expect(page).toContain('一件代发按 ERP 指定价格表下单')
  })
})
