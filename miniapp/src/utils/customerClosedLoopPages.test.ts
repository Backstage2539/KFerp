import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function source(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('customer closed-loop miniapp pages', () => {
  it('keeps direct ship in one employee-style order flow with independent recipient maintenance', () => {
    const page = source('src/components/CustomerDirectShipPanel.vue')

    expect(page).toContain('新建代发订单')
    expect(page).toContain('useCustomerOrderDraftStore')
    expect(page).toContain('选择收件客户')
    expect(page).toContain('createDirectShipRequest')
    expect(page).toContain('idempotency_key')
    expect(page).toContain('ProductFamilyPickerSheet')
    expect(page).not.toContain('ProductSpecPickerSheet')
    expect(page).toContain('class="spec-choice-list"')
    expect(page).toContain('@tap="chooseInlineSpec(line, spec)"')
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
    expect(page).toContain('商品估算合计')
    expect(page).toContain('成品库存优先')
    expect(page).toContain('在制产出')
    expect(page).toContain('代加工商品可履约数量不足，整单不能提交')
    expect(page).toContain('暂未获取物流轨迹')
    expect(page).not.toContain('新建代发批次')
    expect(page).not.toContain('导入代发地址')
    expect(page).not.toContain('现货商品')
    expect(page).not.toContain('v-model="pastedRecipient"')
  })

  it('selects multiple production targets and previews the server BOM without input-material controls', () => {
    const page = source('src/components/CustomerProcessingPanel.vue')

    expect(page).toContain('CustomerProductSelector')
    expect(page).toContain('previewProcessingRequest')
    expect(page).toContain('idempotency_key')
    expect(page).toContain('newProcessingIdempotencyKey')
    expect(page).toContain('mergeProcessingTargetLines')
    expect(page).toContain('最大可生产')
		expect(page).toContain('qty-stepper')
		expect(page).toContain('changeLineQty')
		expect(page).toContain('提交后立即预订物料')
		expect(page).not.toContain('申请阶段不会占用物料')
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
		expect(page).toContain('processing-request-detail')
  })

  it('shows the pre-order supply split and a real processing timeline on standalone pages', () => {
    const directShip = source('src/components/CustomerDirectShipPanel.vue')
    const create = source('src/pages/processing-request-create/processing-request-create.vue')
    const detail = source('src/pages/processing-request-detail/processing-request-detail.vue')
    const fulfillment = source('src/utils/customerFulfillment.ts')
    const pages = source('src/pages.json')

    for (const label of ['现货可用', '工单可预订', '本次可下单', '本单拟占用现货', '本单拟占用在制', '提交后剩余', '价格来源']) {
      expect(directShip).toContain(label)
    }
    expect(create).toContain('CustomerProcessingPanel')
    expect(create).toContain('standalone')
    for (const label of ['待接单', '已接单', '开始生产', '生产完成']) {
      expect(fulfillment).toContain(label)
    }
    for (const label of ['申请数量', '累计入库', '有效预订', '剩余可预订', '当前工序', '使用现货或在制产出继续下单']) {
      expect(detail).toContain(label)
    }
    expect(detail).toContain('processingRequestTimeline')
    expect(pages).toContain('pages/processing-request-create/processing-request-create')
    expect(pages).toContain('pages/processing-request-detail/processing-request-detail')
  })

  it('shows owned finished, green, packaging and semi-finished assets with batches and ledger', () => {
    const list = source('src/components/CustomerInventoryPanel.vue')
    const detail = source('src/pages/customer-inventory-detail/customer-inventory-detail.vue')

    expect(list).toContain('fetchCustomerAssetInventory')
    expect(list).toContain('customerInventoryDetailPath')
    expect(detail).toContain('fetchCustomerInventoryBatches')
    expect(list).toContain("key: 'finished_product'")
    expect(list).toContain("key: 'green_bean'")
    expect(list).toContain("key: 'packaging'")
    expect(list).toContain("key: 'semi_finished'")
    expect(list).toContain('可用')
    expect(list).toContain('占用')
    expect(list).toContain('生产中')
    expect(detail).toContain('库存批次')
    expect(detail).toContain('入库时间')
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

  it('shows the current customer, ERP business contact, shared recipient address book and service explanation', () => {
    const page = source('src/pages/profile/profile.vue')
    const addresses = source('src/pages/customer-addresses/customer-addresses.vue')

    expect(page).toContain('客户资料')
    expect(page).toContain('business_contact_name')
    expect(page).toContain('openRecipientAddresses')
    expect(page).toContain('收件地址')
    expect(addresses).toContain('fetchRecipientAddresses')
    expect(addresses).toContain('createRecipientAddress')
    expect(addresses).toContain('updateRecipientAddress')
    expect(addresses).toContain('deleteRecipientAddress')
    expect(addresses).toContain('当前客户的所有账号共用')
    expect(page).toContain('服务说明')
    expect(page).toContain('一件代发按 ERP 指定价格表下单')
  })
})
